package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"gopkg.in/yaml.v3"
)

// validBase returns a minimal valid Config for mutation in tests.
func validBase() Config {
	var cfg Config
	cfg.Sources.Subscriptions = []Subscription{{URL: "https://example.com/sub"}}
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "(HK)", strategy: select }`)
	cfg.Routing = mustOrderedMap[[]string](`proxy: [HK, DIRECT]`)
	cfg.Rulesets = mustOrderedMap[[]string](`proxy: ["https://example.com/rules.list"]`)
	cfg.Rules = []string{"GEOIP,CN,proxy"}
	cfg.Fallback = "proxy"
	return cfg
}

func testCustomProxy(name, rawURL string, rt *RelayThrough) CustomProxy {
	return CustomProxy{
		URL:          rawURL,
		Name:         name,
		RelayThrough: rt,
	}
}

func TestPrepare_ValidConfig(t *testing.T) {
	cfg := validBase()
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	groups := rt.GroupInput()
	if len(groups) == 0 || groups[0].Match == nil {
		t.Fatal("group regex not compiled")
	}
	if !groups[0].Match.MatchString("HK-01") {
		t.Error("compiled regex should match HK-01")
	}
	if kind, ok := rt.StaticNamespace().Kind("HK"); !ok || kind != staticKindNodeGroup {
		t.Errorf("StaticNamespace HK = (%q, %v), want (%q, true)", kind, ok, staticKindNodeGroup)
	}
	if kind, ok := rt.StaticNamespace().Kind("DIRECT"); !ok || kind != staticKindReserved {
		t.Errorf("StaticNamespace DIRECT = (%q, %v), want (%q, true)", kind, ok, staticKindReserved)
	}
	if kind, ok := rt.StaticNamespace().Kind("proxy"); !ok || kind != staticKindRouteGroup {
		t.Errorf("StaticNamespace proxy = (%q, %v), want (%q, true)", kind, ok, staticKindRouteGroup)
	}
}

func TestPrepare_MissingSubscriptionURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Subscription{{URL: ""}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.subscriptions[0].url")
}

func TestPrepare_InvalidSubscriptionURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Subscription{{URL: "not-a-url"}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.subscriptions[0].url")
}

func TestPrepare_MissingSnellURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Snell = []SnellSource{{URL: ""}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.snell[0].url")
}

func TestPrepare_InvalidSnellURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Snell = []SnellSource{{URL: "ftp://example.com/nodes.txt"}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.snell[0].url")
}

func TestPrepare_ValidSnellURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Snell = []SnellSource{{URL: "https://example.com/snell.txt"}}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rt.SourceInput().Snell) != 1 || rt.SourceInput().Snell[0].URL != "https://example.com/snell.txt" {
		t.Errorf("Snell source not preserved: %+v", rt.SourceInput().Snell)
	}
}

func TestPrepare_MissingVLessURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.VLess = []VLessSource{{URL: ""}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.vless[0].url")
}

func TestPrepare_InvalidVLessURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.VLess = []VLessSource{{URL: "ftp://example.com/vless.txt"}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.vless[0].url")
}

func TestPrepare_ValidVLessURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.VLess = []VLessSource{{URL: "https://example.com/vless.txt"}}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rt.SourceInput().VLess) != 1 || rt.SourceInput().VLess[0].URL != "https://example.com/vless.txt" {
		t.Errorf("VLess source not preserved: %+v", rt.SourceInput().VLess)
	}
}

// ---- custom_proxies: URL mode ----

func TestPrepare_MissingCustomProxyName(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("", "socks5://1.2.3.4:1080", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.custom_proxies[0].name")
}

func TestPrepare_MissingCustomProxyURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{Name: "p1"}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.custom_proxies[0].url")
}

func TestPrepare_InvalidCustomProxyScheme(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "vmess://1.2.3.4:1080", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.custom_proxies[0].url")
}

func TestPrepare_CustomProxySocks5Valid(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://user:pass@1.2.3.4:1080", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	parsed := rt.SourceInput().CustomProxies[0].Parsed
	if parsed.Type != "socks5" {
		t.Errorf("parsed type = %q, want socks5", parsed.Type)
	}
	if kind, ok := rt.StaticNamespace().Kind("p1"); !ok || kind != staticKindCustom {
		t.Errorf("StaticNamespace p1 = (%q, %v), want (%q, true)", kind, ok, staticKindCustom)
	}
}

func TestPrepare_CustomProxyHTTPValid(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "http://user:pass@1.2.3.4:8080", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	parsed := rt.SourceInput().CustomProxies[0].Parsed
	if parsed.Type != "http" {
		t.Errorf("parsed type = %q, want http", parsed.Type)
	}
}

func TestPrepare_CustomProxySSValid(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	parsed := rt.SourceInput().CustomProxies[0].Parsed
	if parsed.Params["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher = %q, want aes-256-gcm", parsed.Params["cipher"])
	}
	if parsed.Params["password"] != "mypass" {
		t.Errorf("password = %q, want mypass", parsed.Params["password"])
	}
}

func TestPrepare_CustomProxySSWithPlugin(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	plugin := rt.SourceInput().CustomProxies[0].Parsed.Plugin
	if plugin == nil {
		t.Fatal("plugin should not be nil")
	}
	if plugin.Opts["obfs"] != "http" {
		t.Errorf("plugin obfs = %q, want http", plugin.Opts["obfs"])
	}
}

func TestPrepare_CustomProxySSFragmentIgnored(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("MY-NODE", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388#OtherName", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if rt.SourceInput().CustomProxies[0].Name != "MY-NODE" {
		t.Errorf("Name = %q, want MY-NODE (fragment should be ignored)", rt.SourceInput().CustomProxies[0].Name)
	}
}

func TestPrepare_CustomProxySSMissingCipher(t *testing.T) {
	cfg := validBase()
	cp := CustomProxy{URL: "ss://YmFkcGFzcw@1.2.3.4:8388", Name: "p1"}
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.custom_proxies[0].url")
}

func TestPrepare_CustomProxySSMissingPassword(t *testing.T) {
	cfg := validBase()
	cp := CustomProxy{URL: "ss://YWVzLTI1Ni1nY206@1.2.3.4:8388", Name: "p1"}
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.custom_proxies[0].url")
}

func TestPrepare_DuplicateCustomProxyNames(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{
		testCustomProxy("dup", "socks5://1.2.3.4:1080", nil),
		testCustomProxy("dup", "http://5.6.7.8:8080", nil),
	}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.custom_proxies[1].name")
}

func TestPrepare_CustomProxySSWithRelayThrough(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("chain-ss", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388",
		&RelayThrough{Type: "all", Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	relay := rt.SourceInput().CustomProxies[0].RelayThrough
	if relay == nil {
		t.Fatal("RelayThrough should not be nil")
	}
	if relay.Type != "all" {
		t.Errorf("relay type = %q, want all", relay.Type)
	}
	if relay.Strategy != "select" {
		t.Errorf("relay strategy = %q, want select", relay.Strategy)
	}
	if kind, ok := rt.StaticNamespace().Kind("chain-ss"); !ok || kind != staticKindChainGroup {
		t.Errorf("StaticNamespace chain-ss = (%q, %v), want (%q, true)", kind, ok, staticKindChainGroup)
	}
}

func TestPrepare_RelayThroughMissingStrategy(t *testing.T) {
	// T-CFG-004
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "all"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "relay_through.strategy")
}

func TestPrepare_RelayThroughInvalidStrategy(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "all", Strategy: "random"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "relay_through.strategy")
}

func TestPrepare_RelayThroughMissingType(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "relay_through.type")
}

func TestPrepare_RelayThroughInvalidType(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "invalid", Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "relay_through.type")
}

func TestPrepare_RelayThroughGroupMissingName(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "group", Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "relay_through.name")
}

func TestPrepare_RelayThroughSelectMissingMatch(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "select", Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "relay_through.match")
}

func TestPrepare_RelayThroughSelectInvalidRegex(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "select", Strategy: "select", Match: "[invalid"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "relay_through.match")
}

func TestPrepare_FiltersInvalidRegex(t *testing.T) {
	cfg := validBase()
	cfg.Filters.Exclude = "[broken"
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "filters.exclude")
}

func TestPrepare_GroupMissingMatch(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "", strategy: select }`)
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "groups.HK.match")
}

func TestPrepare_GroupInvalidRegex(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "[bad", strategy: select }`)
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "groups.HK.match")
}

func TestPrepare_GroupInvalidStrategy(t *testing.T) {
	// T-CFG-005
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "(HK)", strategy: random }`)
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "groups.HK.strategy")
}

func TestPrepare_GroupMissingStrategy(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "(HK)" }`)
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "groups.HK.strategy")
}

func TestPrepare_MissingFallback(t *testing.T) {
	cfg := validBase()
	cfg.Fallback = ""
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "fallback")
}

func TestPrepare_EmptyRoutingEntryRejected(t *testing.T) {
	cfg := validBase()
	cfg.Routing = mustOrderedMap[[]string](`proxy: []`)
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "routing.proxy")
}

func TestPrepare_RulesetEmptyList(t *testing.T) {
	cfg := validBase()
	cfg.Rulesets = mustOrderedMap[[]string](`proxy: []`)
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "rulesets.proxy")
}

func TestPrepare_RulesetEmptyURL(t *testing.T) {
	cfg := validBase()
	cfg.Rulesets = mustOrderedMap[[]string](`proxy: [""]`)
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "rulesets.proxy[0]")
}

func TestPrepare_RulesetInvalidURL(t *testing.T) {
	cfg := validBase()
	cfg.Rulesets = mustOrderedMap[[]string](`proxy: ["not-a-url"]`)
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "rulesets.proxy[0]")
}

func TestPrepare_BaseURLInvalidScheme(t *testing.T) {
	cfg := validBase()
	cfg.BaseURL = "ftp://example.com"
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "base_url")
}

func TestPrepare_BaseURLWithPath(t *testing.T) {
	cfg := validBase()
	cfg.BaseURL = "https://example.com/path"
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "base_url")
}

func TestPrepare_BaseURLValid(t *testing.T) {
	cfg := validBase()
	cfg.BaseURL = "https://example.com"
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if rt.BaseURL() != "https://example.com" {
		t.Errorf("BaseURL() = %q, want https://example.com", rt.BaseURL())
	}
}

func TestPrepare_MultipleErrors(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Subscription{{URL: ""}}
	cfg.Fallback = ""

	_, err := Prepare(&cfg)
	if err == nil {
		t.Fatal("expected error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "sources.subscriptions[0].url") {
		t.Errorf("missing subscription error in: %s", msg)
	}
	if !strings.Contains(msg, "fallback") {
		t.Errorf("missing fallback error in: %s", msg)
	}
}

// --- helpers ---

func assertFieldError(t *testing.T, err error, fieldSubstr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing field %q, got nil", fieldSubstr)
	}

	for _, e := range unwrapAll(err) {
		var ce *errtype.ConfigError
		if errors.As(e, &ce) && strings.Contains(ce.Field, fieldSubstr) {
			return
		}
	}
	t.Errorf("no ConfigError with field containing %q in: %v", fieldSubstr, err)
}

func unwrapAll(err error) []error {
	type joinedError interface{ Unwrap() []error }
	if je, ok := err.(joinedError); ok {
		return je.Unwrap()
	}
	return []error{err}
}

func mustOrderedMap[V any](yamlStr string) OrderedMap[V] {
	var m OrderedMap[V]
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		panic("mustOrderedMap: " + err.Error())
	}
	return m
}

func TestPrepare_TemplateClashInvalidURL(t *testing.T) {
	cfg := validBase()
	cfg.Templates.Clash = "https://"
	_, err := Prepare(&cfg)
	if err == nil {
		t.Fatal("expected error for invalid template URL")
	}
	if !strings.Contains(err.Error(), "templates.clash") {
		t.Errorf("error should mention templates.clash: %v", err)
	}
}

func TestPrepare_TemplateClashValidURL(t *testing.T) {
	cfg := validBase()
	cfg.Templates.Clash = "https://example.com/base.yaml"
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("valid template URL should pass: %v", err)
	}
	if rt.Templates().Clash != "https://example.com/base.yaml" {
		t.Errorf("Templates().Clash = %q", rt.Templates().Clash)
	}
}

func TestPrepare_TemplateLocalPath(t *testing.T) {
	cfg := validBase()
	cfg.Templates.Surge = "./configs/base_surge.conf"
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("local template path should pass validation: %v", err)
	}
	if rt.Templates().Surge != "./configs/base_surge.conf" {
		t.Errorf("Templates().Surge = %q", rt.Templates().Surge)
	}
}

func TestPrepare_RoutingAutoAndAllMutuallyExclusive(t *testing.T) {
	cfg := validBase()
	cfg.Routing = mustOrderedMap[[]string](`proxy: ["@all", "@auto"]`)
	_, err := Prepare(&cfg)
	if err == nil {
		t.Fatal("expected error for @all + @auto in same entry")
	}
	if !strings.Contains(err.Error(), "@all 和 @auto 不能同时使用") {
		t.Errorf("error = %v, want @all+@auto conflict message", err)
	}
}

func TestPrepare_RoutingAutoAloneIsValid(t *testing.T) {
	cfg := validBase()
	cfg.Routing = mustOrderedMap[[]string](`proxy: ["@auto"]`)
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("@auto alone should be valid: %v", err)
	}
	routing, _, _, _ := rt.RouteInput()
	expanded := routing[0].ExpandedMembers
	hasAutoExpanded := false
	for _, m := range expanded {
		if m.Origin == RouteMemberOriginAutoExpanded {
			hasAutoExpanded = true
			break
		}
	}
	if !hasAutoExpanded {
		t.Error("@auto should produce AutoExpanded members in ExpandedMembers")
	}
}

func TestPrepare_RoutingAutoRepeatedRejected(t *testing.T) {
	cfg := validBase()
	cfg.Routing = mustOrderedMap[[]string](`proxy: ["@auto", "@auto"]`)
	_, err := Prepare(&cfg)
	if err == nil {
		t.Fatal("expected error for repeated @auto in same entry")
	}
	if !strings.Contains(err.Error(), "@auto 不能重复使用") {
		t.Errorf("error = %v, want repeated @auto message", err)
	}
}
