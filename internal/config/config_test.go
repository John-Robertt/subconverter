package config

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const exampleConfigPath = "../../configs/base_config.yaml"

// T-CFG-001: groups 保序解析
func TestIntegration_GroupsOrder(t *testing.T) {
	cfg := mustLoadExample(t)

	want := []string{
		"🇭🇰 Hong Kong",
		"🇸🇬 Singapore",
		"🇨🇳 Taiwan",
		"🇯🇵 Japan",
		"🇺🇲 United States",
	}
	if !slices.Equal(cfg.Groups.Keys(), want) {
		t.Errorf("Groups.Keys() = %v\n期望 %v", cfg.Groups.Keys(), want)
	}
}

// T-CFG-002: routing 保序解析
func TestIntegration_RoutingOrder(t *testing.T) {
	cfg := mustLoadExample(t)

	want := []string{
		"🚀 快速选择",
		"🚀 手动切换",
		"📲 Telegram",
		"📺 Netflix",
		"📺 DisneyPlus",
		"📺 ViuTV",
		"🎬 YouTube",
		"🍎 Apple",
		"🔍 Google",
		"💻 Github",
		"☁️ OneDrive",
		"Ⓜ️ Microsoft",
		"💳 PayPal",
		"💳 Stripe",
		"🌍 DMM",
		"🎯 Global",
		"🎯 China",
		"🛑 BanList",
		"🐟 FINAL",
	}
	if !slices.Equal(cfg.Routing.Keys(), want) {
		t.Errorf("Routing.Keys() = %v\n期望 %v", cfg.Routing.Keys(), want)
	}
}

// T-CFG-003: rulesets 保序解析
func TestIntegration_RulesetsOrder(t *testing.T) {
	cfg := mustLoadExample(t)

	want := []string{
		"🛑 BanList",
		"📺 Netflix",
		"📲 Telegram",
		"🎬 YouTube",
		"🔍 Google",
		"💻 Github",
		"🍎 Apple",
		"Ⓜ️ Microsoft",
		"☁️ OneDrive",
		"💳 PayPal",
		"💳 Stripe",
		"📺 DisneyPlus",
		"🎯 China",
		"📺 ViuTV",
		"🌍 DMM",
		"🎯 Global",
	}
	if !slices.Equal(cfg.Rulesets.Keys(), want) {
		t.Errorf("Rulesets.Keys() = %v\n期望 %v", cfg.Rulesets.Keys(), want)
	}
}

func TestIntegration_PrepareExampleConfig(t *testing.T) {
	cfg := mustLoadExample(t)
	rt, err := Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare 示例配置：%v", err)
	}
	if len(rt.GroupInput()) != cfg.Groups.Len() {
		t.Errorf("预处理后的 groups 数量 = %d，期望 %d", len(rt.GroupInput()), cfg.Groups.Len())
	}
	routing, _, _, _ := rt.RouteInput()
	if len(routing) != cfg.Routing.Len() {
		t.Errorf("预处理后的 routing 数量 = %d，期望 %d", len(routing), cfg.Routing.Len())
	}
}

func TestIntegration_SpotCheckValues(t *testing.T) {
	cfg := mustLoadExample(t)

	// base_url
	if cfg.BaseURL != "https://my-server.com" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}

	// fallback
	if cfg.Fallback != "🐟 FINAL" {
		t.Errorf("Fallback = %q", cfg.Fallback)
	}

	// 第一个组
	hk, ok := cfg.Groups.Get("🇭🇰 Hong Kong")
	if !ok {
		t.Fatal("未找到组 🇭🇰 Hong Kong")
	}
	if hk.Match != "(港|HK|Hong Kong)" {
		t.Errorf("HK Match = %q", hk.Match)
	}
	if hk.Strategy != "select" {
		t.Errorf("HK Strategy = %q", hk.Strategy)
	}

	// Japan 的策略是 url-test
	jp, ok := cfg.Groups.Get("🇯🇵 Japan")
	if !ok {
		t.Fatal("未找到组 🇯🇵 Japan")
	}
	if jp.Strategy != "url-test" {
		t.Errorf("JP Strategy = %q，期望 url-test", jp.Strategy)
	}

	// 第一个 routing 条目的成员（使用 @auto 简写）
	fast, ok := cfg.Routing.Get("🚀 快速选择")
	if !ok {
		t.Fatal("未找到 routing 🚀 快速选择")
	}
	if len(fast) != 1 || fast[0] != "@auto" {
		t.Errorf("🚀 快速选择 成员 = %v，期望 [@auto]", fast)
	}

	// rulesets：BanList 有 3 个 URL
	ban, ok := cfg.Rulesets.Get("🛑 BanList")
	if !ok {
		t.Fatal("未找到规则集 🛑 BanList")
	}
	if len(ban) != 3 {
		t.Errorf("BanList URL 数量 = %d，期望 3", len(ban))
	}

	// rules
	if len(cfg.Rules) != 1 || cfg.Rules[0] != "GEOIP,CN,🎯 China" {
		t.Errorf("Rules = %v", cfg.Rules)
	}

	// filters
	if cfg.Filters.Exclude != "过期|剩余流量|到期|Traffic|Expire|Reset|Date|\\|.*G" {
		t.Errorf("Filters.Exclude = %q", cfg.Filters.Exclude)
	}

	// 自定义代理
	cp := cfg.Sources.CustomProxies[0]
	if cp.RelayThrough.Name != "🇭🇰 Hong Kong" {
		t.Errorf("RelayThrough.Name = %q", cp.RelayThrough.Name)
	}
}

// TestSources_FetchOrderPreserved：FetchOrder 按 YAML 声明顺序记录拉取类键。
// subscriptions/snell/vless 只在被声明时出现，并按声明顺序排列。
// custom_proxies 不进入 FetchOrder。
func TestSources_FetchOrderPreserved(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want []string
	}{
		{
			name: "snell_then_vless_then_subscriptions",
			yaml: `
snell:
  - url: https://example.com/snell.txt
vless:
  - url: https://example.com/vless.txt
subscriptions:
  - url: https://example.com/sub
`,
			want: []string{"snell", "vless", "subscriptions"},
		},
		{
			name: "subscriptions_only",
			yaml: `
subscriptions:
  - url: https://example.com/sub
`,
			want: []string{"subscriptions"},
		},
		{
			name: "vless_then_snell_with_custom_proxies_between",
			yaml: `
vless:
  - url: https://example.com/vless.txt
custom_proxies:
  - name: local
    url: socks5://127.0.0.1:1080
snell:
  - url: https://example.com/snell.txt
`,
			want: []string{"vless", "snell"}, // 不包含 custom_proxies
		},
		{
			name: "empty_sections_still_recorded",
			yaml: `
vless:
snell:
`,
			want: []string{"vless", "snell"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var s Sources
			if err := yaml.Unmarshal([]byte(tc.yaml), &s); err != nil {
				t.Fatalf("反序列化：%v", err)
			}
			if !slices.Equal(s.FetchOrder, tc.want) {
				t.Errorf("FetchOrder = %v，期望 %v", s.FetchOrder, tc.want)
			}
		})
	}
}

// TestSources_RejectsDuplicateKey：同一个顶层键出现两次
// （例如在一个 sources 块中两次声明 `vless:`）属于错误。
func TestSources_RejectsDuplicateKey(t *testing.T) {
	y := `
vless:
  - url: https://a.example/v.txt
vless:
  - url: https://b.example/v.txt
`
	var s Sources
	err := yaml.Unmarshal([]byte(y), &s)
	if err == nil {
		t.Fatal("期望返回键重复错误，实际为 nil")
	}
}

func TestSources_JSONIncludesCompleteFetchOrder(t *testing.T) {
	var s Sources
	if err := yaml.Unmarshal([]byte(`
subscriptions:
  - url: https://example.com/sub
`), &s); err != nil {
		t.Fatalf("反序列化 YAML：%v", err)
	}
	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("序列化 JSON：%v", err)
	}
	var got struct {
		FetchOrder []string `json:"fetch_order"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("反序列化输出：%v", err)
	}
	if !slices.Equal(got.FetchOrder, []string{"subscriptions", "snell", "vless"}) {
		t.Fatalf("fetch_order = %v", got.FetchOrder)
	}
}

func TestSources_MarshalYAMLUsesFetchOrderAndCustomLast(t *testing.T) {
	s := Sources{
		Subscriptions: []Source{{URL: "https://example.com/sub"}},
		Snell:         []Source{{URL: "https://example.com/snell"}},
		VLess:         []Source{{URL: "https://example.com/vless"}},
		CustomProxies: []CustomProxy{{Name: "local", URL: "socks5://127.0.0.1:1080"}},
		FetchOrder:    []string{"vless", "subscriptions", "snell"},
	}
	out, err := yaml.Marshal(s)
	if err != nil {
		t.Fatalf("序列化 YAML：%v", err)
	}
	wantOrder := []string{"vless:", "subscriptions:", "snell:", "custom_proxies:"}
	last := -1
	for _, marker := range wantOrder {
		idx := strings.Index(string(out), marker)
		if idx < 0 {
			t.Fatalf("YAML 缺少 %q：\n%s", marker, out)
		}
		if idx <= last {
			t.Fatalf("%q 出现的顺序不对，输出为：\n%s", marker, out)
		}
		last = idx
	}
}

// TestSources_RejectsUnknownKey：`vles`、`subscrition` 这类拼写错误
// 会明确报错，而不是被悄悄忽略。
func TestSources_RejectsUnknownKey(t *testing.T) {
	y := `
vles:
  - url: https://example.com/v.txt
`
	var s Sources
	err := yaml.Unmarshal([]byte(y), &s)
	if err == nil {
		t.Fatal("期望返回未知键错误，实际为 nil")
	}
}

func mustLoadExample(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load(context.Background(), exampleConfigPath, nil)
	if err != nil {
		t.Fatalf("加载示例配置：%v", err)
	}
	return cfg
}
