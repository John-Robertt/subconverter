package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"gopkg.in/yaml.v3"
)

// validBase 返回一份最小的合法 Config，供测试在其基础上修改。
func validBase() Config {
	var cfg Config
	cfg.Sources.Subscriptions = []Source{{URL: "https://example.com/sub"}}
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
		t.Fatalf("期望 nil，实际为 %v", err)
	}

	groups := rt.GroupInput()
	if len(groups) == 0 || groups[0].Match == nil {
		t.Fatal("组正则未编译")
	}
	if !groups[0].Match.MatchString("HK-01") {
		t.Error("编译后的正则应匹配 HK-01")
	}
}

func TestPrepare_MissingSubscriptionURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Source{{URL: ""}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.subscriptions[0].url")
}

func TestPrepare_InvalidSubscriptionURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Source{{URL: "not-a-url"}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.subscriptions[0].url")
}

func TestPrepare_MissingSnellURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Snell = []Source{{URL: ""}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.snell[0].url")
}

func TestPrepare_InvalidSnellURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Snell = []Source{{URL: "ftp://example.com/nodes.txt"}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.snell[0].url")
}

func TestPrepare_ValidSnellURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Snell = []Source{{URL: "https://example.com/snell.txt"}}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("期望 nil，实际为 %v", err)
	}
	if len(rt.SourceInput().Snell) != 1 || rt.SourceInput().Snell[0].URL != "https://example.com/snell.txt" {
		t.Errorf("Snell 来源未被保留：%+v", rt.SourceInput().Snell)
	}
}

func TestPrepare_MissingVLessURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.VLess = []Source{{URL: ""}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.vless[0].url")
}

func TestPrepare_InvalidVLessURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.VLess = []Source{{URL: "ftp://example.com/vless.txt"}}
	_, err := Prepare(&cfg)
	assertFieldError(t, err, "sources.vless[0].url")
}

func TestPrepare_ValidVLessURL(t *testing.T) {
	cfg := validBase()
	cfg.Sources.VLess = []Source{{URL: "https://example.com/vless.txt"}}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("期望 nil，实际为 %v", err)
	}
	if len(rt.SourceInput().VLess) != 1 || rt.SourceInput().VLess[0].URL != "https://example.com/vless.txt" {
		t.Errorf("VLess 来源未被保留：%+v", rt.SourceInput().VLess)
	}
}

// ---- custom_proxies：URL 模式 ----

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
		t.Fatalf("期望 nil，实际为 %v", err)
	}
	parsed := rt.SourceInput().CustomProxies[0].Parsed
	if parsed.Type != "socks5" {
		t.Errorf("解析出的类型 = %q，期望 socks5", parsed.Type)
	}
}

func TestPrepare_CustomProxyHTTPValid(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "http://user:pass@1.2.3.4:8080", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("期望 nil，实际为 %v", err)
	}
	parsed := rt.SourceInput().CustomProxies[0].Parsed
	if parsed.Type != "http" {
		t.Errorf("解析出的类型 = %q，期望 http", parsed.Type)
	}
}

func TestPrepare_CustomProxySSValid(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("期望 nil，实际为 %v", err)
	}
	parsed := rt.SourceInput().CustomProxies[0].Parsed
	if parsed.Params["cipher"] != "aes-256-gcm" {
		t.Errorf("cipher = %q，期望 aes-256-gcm", parsed.Params["cipher"])
	}
	if parsed.Params["password"] != "mypass" {
		t.Errorf("password = %q，期望 mypass", parsed.Params["password"])
	}
}

func TestPrepare_CustomProxySSWithPlugin(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("p1", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("期望 nil，实际为 %v", err)
	}
	plugin := rt.SourceInput().CustomProxies[0].Parsed.Plugin
	if plugin == nil {
		t.Fatal("plugin 不应为 nil")
	}
	if plugin.Opts["obfs"] != "http" {
		t.Errorf("plugin obfs = %q，期望 http", plugin.Opts["obfs"])
	}
}

func TestPrepare_CustomProxySSFragmentIgnored(t *testing.T) {
	cfg := validBase()
	cp := testCustomProxy("MY-NODE", "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388#OtherName", nil)
	cfg.Sources.CustomProxies = []CustomProxy{cp}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("期望 nil，实际为 %v", err)
	}
	if rt.SourceInput().CustomProxies[0].Name != "MY-NODE" {
		t.Errorf("Name = %q，期望 MY-NODE（应忽略 fragment）", rt.SourceInput().CustomProxies[0].Name)
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

// T-CFG-ID-001：独立的自定义代理可以使用相同的显示名。
func TestPrepare_DuplicateStandaloneCustomProxyNamesAllowed(t *testing.T) {
	cfg := validBase()
	cfg.Sources.CustomProxies = []CustomProxy{
		testCustomProxy("dup", "socks5://1.2.3.4:1080", nil),
		testCustomProxy("dup", "http://5.6.7.8:8080", nil),
	}
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("独立代理的显示名重复应视为合法：%v", err)
	}
	if got := len(rt.SourceInput().CustomProxies); got != 2 {
		t.Fatalf("预处理后的自定义代理数 = %d，期望 2", got)
	}
}

// T-CFG-ID-002：relay_through 声明仍会创建组，
// 这些组名在组命名空间内必须唯一。
func TestPrepare_DuplicateChainGroupNamesRejected(t *testing.T) {
	cfg := validBase()
	relay := &RelayThrough{Type: "all", Strategy: "select"}
	cfg.Sources.CustomProxies = []CustomProxy{
		testCustomProxy("dup-chain", "socks5://1.2.3.4:1080", relay),
		testCustomProxy("dup-chain", "http://5.6.7.8:8080", relay),
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
		t.Fatalf("期望 nil，实际为 %v", err)
	}
	relay := rt.SourceInput().CustomProxies[0].RelayThrough
	if relay == nil {
		t.Fatal("RelayThrough 不应为 nil")
	}
	if relay.Type != "all" {
		t.Errorf("relay 类型 = %q，期望 all", relay.Type)
	}
	if relay.Strategy != "select" {
		t.Errorf("relay 策略 = %q，期望 select", relay.Strategy)
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
		t.Fatalf("期望 nil，实际为 %v", err)
	}
	if rt.BaseURL() != "https://example.com" {
		t.Errorf("BaseURL() = %q，期望 https://example.com", rt.BaseURL())
	}
}

func TestPrepare_MultipleErrors(t *testing.T) {
	cfg := validBase()
	cfg.Sources.Subscriptions = []Source{{URL: ""}}
	cfg.Fallback = ""

	_, err := Prepare(&cfg)
	if err == nil {
		t.Fatal("期望返回错误")
	}

	msg := err.Error()
	if !strings.Contains(msg, "sources.subscriptions[0].url") {
		t.Errorf("错误信息中缺少订阅相关的错误：%s", msg)
	}
	if !strings.Contains(msg, "fallback") {
		t.Errorf("错误信息中缺少 fallback 相关的错误：%s", msg)
	}
}

// --- 辅助函数 ---

func assertFieldError(t *testing.T, err error, fieldSubstr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("期望错误包含字段 %q，实际为 nil", fieldSubstr)
	}

	for _, e := range unwrapAll(err) {
		var ce *errtype.ConfigError
		if errors.As(e, &ce) && strings.Contains(ce.Field, fieldSubstr) {
			return
		}
	}
	t.Errorf("没有找到字段包含 %q 的 ConfigError，错误为：%v", fieldSubstr, err)
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
		t.Fatal("模板 URL 无效时应返回错误")
	}
	if !strings.Contains(err.Error(), "templates.clash") {
		t.Errorf("错误信息应提到 templates.clash：%v", err)
	}
}

func TestPrepare_TemplateClashValidURL(t *testing.T) {
	cfg := validBase()
	cfg.Templates.Clash = "https://example.com/base.yaml"
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("合法的模板 URL 应通过校验：%v", err)
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
		t.Fatalf("本地模板路径应通过校验：%v", err)
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
		t.Fatal("同一条目同时使用 @all 和 @auto 时应返回错误")
	}
	if !strings.Contains(err.Error(), "@all 和 @auto 不能同时使用") {
		t.Errorf("错误 = %v，期望包含 @all 与 @auto 冲突的提示", err)
	}
}

func TestPrepare_RoutingAutoAloneIsValid(t *testing.T) {
	cfg := validBase()
	cfg.Routing = mustOrderedMap[[]string](`proxy: ["@auto"]`)
	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("单独使用 @auto 应视为合法：%v", err)
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
		t.Error("@auto 应在 ExpandedMembers 中产生 AutoExpanded 成员")
	}
}

func TestPrepare_RoutingAutoRepeatedRejected(t *testing.T) {
	cfg := validBase()
	cfg.Routing = mustOrderedMap[[]string](`proxy: ["@auto", "@auto"]`)
	_, err := Prepare(&cfg)
	if err == nil {
		t.Fatal("同一条目重复使用 @auto 时应返回错误")
	}
	if !strings.Contains(err.Error(), "@auto 不能重复使用") {
		t.Errorf("错误 = %v，期望包含 @auto 重复使用的提示", err)
	}
}
