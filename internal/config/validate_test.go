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

func TestValidate_ValidConfig(t *testing.T) {
	cfg := validBase()
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidate_MissingSubscriptionURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Subscription{{URL: ""}}
	assertFieldError(t, Validate(&cfg), "sources.subscriptions[0].url")
}

func TestValidate_InvalidSubscriptionURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Subscription{{URL: "not-a-url"}}
	assertFieldError(t, Validate(&cfg), "sources.subscriptions[0].url")
}

func TestValidate_MissingSnellURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Snell = []SnellSource{{URL: ""}}
	assertFieldError(t, Validate(&cfg), "sources.snell[0].url")
}

func TestValidate_InvalidSnellURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Snell = []SnellSource{{URL: "ftp://example.com/nodes.txt"}}
	assertFieldError(t, Validate(&cfg), "sources.snell[0].url")
}

func TestValidate_ValidSnellURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Snell = []SnellSource{{URL: "https://example.com/snell.txt"}}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidate_MissingVLessURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.VLess = []VLessSource{{URL: ""}}
	assertFieldError(t, Validate(&cfg), "sources.vless[0].url")
}

func TestValidate_InvalidVLessURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.VLess = []VLessSource{{URL: "ftp://example.com/vless.txt"}}
	assertFieldError(t, Validate(&cfg), "sources.vless[0].url")
}

func TestValidate_ValidVLessURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.VLess = []VLessSource{{URL: "https://example.com/vless.txt"}}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// ---- custom_proxies: URL mode ----

func TestValidate_MissingCustomProxyName(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("", "socks5://1.2.3.4:1080", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].name")
}

func TestValidate_MissingCustomProxyURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{{Name: "p1"}}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].url")
}

func TestValidate_InvalidCustomProxyScheme(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "vmess://1.2.3.4:1080", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].url")
}

func TestValidate_CustomProxySocks5Valid(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://user:pass@1.2.3.4:1080", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidate_CustomProxyHTTPValid(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "http://user:pass@1.2.3.4:8080", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidate_CustomProxySSValid(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidate_CustomProxySSWithPlugin(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidate_CustomProxySSFragmentIgnored(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("MY-NODE", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388#OtherName", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if cp.Name != "MY-NODE" {
		t.Errorf("Name = %q, want %q (fragment should be ignored)", cp.Name, "MY-NODE")
	}
}

func TestValidate_CustomProxySSMissingCipher(t *testing.T) {
	cfg := validBase()
	cp := CustomProxy{URL: "ss://YmFkcGFzcw@1.2.3.4:8388", Name: "p1"}
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].url")
}

func TestValidate_CustomProxySSMissingPassword(t *testing.T) {
	cfg := validBase()
	cp := CustomProxy{URL: "ss://YWVzLTI1Ni1nY206@1.2.3.4:8388", Name: "p1"}
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[0].url")
}

func TestValidate_DuplicateCustomProxyNames(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{
		testCustomProxy("dup", "socks5://1.2.3.4:1080", nil),
		testCustomProxy("dup", "http://5.6.7.8:8080", nil),
	}
	assertFieldError(t, Validate(&cfg), "sources.custom_proxies[1].name")
}

func TestValidate_CustomProxySSWithRelayThrough(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("chain-ss", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388",
		&RelayThrough{Type: "all", Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	if err := Validate(&cfg); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidate_RelayThroughMissingStrategy(t *testing.T) {
	// T-CFG-004
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "all"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "relay_through.strategy")
}

func TestValidate_RelayThroughInvalidStrategy(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "all", Strategy: "random"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "relay_through.strategy")
}

func TestValidate_RelayThroughMissingType(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "relay_through.type")
}

func TestValidate_RelayThroughInvalidType(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "invalid", Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "relay_through.type")
}

func TestValidate_RelayThroughGroupMissingName(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "group", Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "relay_through.name")
}

func TestValidate_RelayThroughSelectMissingMatch(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "select", Strategy: "select"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "relay_through.match")
}

func TestValidate_RelayThroughSelectInvalidRegex(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "socks5://1.2.3.4:1080",
		&RelayThrough{Type: "select", Strategy: "select", Match: "[invalid"})
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	assertFieldError(t, Validate(&cfg), "relay_through.match")
}

func TestValidate_FiltersInvalidRegex(t *testing.T) {
	cfg := validBase()
	cfg.Filters.Exclude = "[broken"
	assertFieldError(t, Validate(&cfg), "filters.exclude")
}

func TestValidate_GroupMissingMatch(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "", strategy: select }`)
	assertFieldError(t, Validate(&cfg), "groups.HK.match")
}

func TestValidate_GroupInvalidRegex(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "[bad", strategy: select }`)
	assertFieldError(t, Validate(&cfg), "groups.HK.match")
}

func TestValidate_GroupInvalidStrategy(t *testing.T) {
	// T-CFG-005
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "(HK)", strategy: random }`)
	assertFieldError(t, Validate(&cfg), "groups.HK.strategy")
}

func TestValidate_GroupMissingStrategy(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`HK: { match: "(HK)" }`)
	assertFieldError(t, Validate(&cfg), "groups.HK.strategy")
}

func TestValidate_MissingFallback(t *testing.T) {
	cfg := validBase()
	cfg.Fallback = ""
	assertFieldError(t, Validate(&cfg), "fallback")
}

func TestValidate_RulesetEmptyList(t *testing.T) {
	cfg := validBase()
	cfg.Rulesets = mustOrderedMap[[]string](`proxy: []`)
	assertFieldError(t, Validate(&cfg), "rulesets.proxy")
}

func TestValidate_RulesetEmptyURL(t *testing.T) {
	cfg := validBase()
	cfg.Rulesets = mustOrderedMap[[]string](`proxy: [""]`)
	assertFieldError(t, Validate(&cfg), "rulesets.proxy[0]")
}

func TestValidate_RulesetInvalidURL(t *testing.T) {
	cfg := validBase()
	cfg.Rulesets = mustOrderedMap[[]string](`proxy: ["not-a-url"]`)
	assertFieldError(t, Validate(&cfg), "rulesets.proxy[0]")
}

func TestValidate_BaseURLInvalidScheme(t *testing.T) {
	cfg := validBase()
	cfg.BaseURL = "ftp://example.com"
	assertFieldError(t, Validate(&cfg), "base_url")
}

func TestValidate_BaseURLWithPath(t *testing.T) {
	cfg := validBase()
	cfg.BaseURL = "https://example.com/path"
	assertFieldError(t, Validate(&cfg), "base_url")
}

func TestValidate_BaseURLValid(t *testing.T) {
	cfg := validBase()
	cfg.BaseURL = "https://example.com"
	if err := Validate(&cfg); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Subscription{{URL: ""}}
	cfg.Fallback = ""

	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected error")
	}

	// Should contain at least 2 errors
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

func TestValidate_TemplateClashInvalidURL(t *testing.T) {
	cfg := validBase()
	cfg.Templates.Clash = "https://"
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected error for invalid template URL")
	}
	if !strings.Contains(err.Error(), "templates.clash") {
		t.Errorf("error should mention templates.clash: %v", err)
	}
}

func TestValidate_TemplateClashValidURL(t *testing.T) {
	cfg := validBase()
	cfg.Templates.Clash = "https://example.com/base.yaml"
	if err := Validate(&cfg); err != nil {
		t.Errorf("valid template URL should pass: %v", err)
	}
}

func TestValidate_TemplateLocalPath(t *testing.T) {
	cfg := validBase()
	cfg.Templates.Surge = "./configs/base_surge.conf"
	if err := Validate(&cfg); err != nil {
		t.Errorf("local template path should pass validation: %v", err)
	}
}

func TestValidate_RoutingAutoAndAllMutuallyExclusive(t *testing.T) {
	cfg := validBase()
	cfg.Routing = mustOrderedMap[[]string](`proxy: ["@all", "@auto"]`)
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected error for @all + @auto in same entry")
	}
	if !strings.Contains(err.Error(), "@all 和 @auto 不能同时使用") {
		t.Errorf("error = %v, want @all+@auto conflict message", err)
	}
}

func TestValidate_RoutingAutoAloneIsValid(t *testing.T) {
	cfg := validBase()
	cfg.Routing = mustOrderedMap[[]string](`proxy: ["@auto"]`)
	if err := Validate(&cfg); err != nil {
		t.Errorf("@auto alone should be valid: %v", err)
	}
}

func TestValidate_RoutingAutoRepeatedRejected(t *testing.T) {
	cfg := validBase()
	cfg.Routing = mustOrderedMap[[]string](`proxy: ["@auto", "@auto"]`)
	err := Validate(&cfg)
	if err == nil {
		t.Fatal("expected error for repeated @auto in same entry")
	}
	if !strings.Contains(err.Error(), "@auto 不能重复使用") {
		t.Errorf("error = %v, want repeated @auto message", err)
	}
}
