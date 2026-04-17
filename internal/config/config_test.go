package config

import (
	"context"
	"slices"
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
		t.Errorf("Groups.Keys() = %v\nwant %v", cfg.Groups.Keys(), want)
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
		t.Errorf("Routing.Keys() = %v\nwant %v", cfg.Routing.Keys(), want)
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
		t.Errorf("Rulesets.Keys() = %v\nwant %v", cfg.Rulesets.Keys(), want)
	}
}

func TestIntegration_ValidateExampleConfig(t *testing.T) {
	cfg := mustLoadExample(t)
	if err := Validate(cfg); err != nil {
		t.Errorf("Validate example config: %v", err)
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

	// first group
	hk, ok := cfg.Groups.Get("🇭🇰 Hong Kong")
	if !ok {
		t.Fatal("group 🇭🇰 Hong Kong not found")
	}
	if hk.Match != "(港|HK|Hong Kong)" {
		t.Errorf("HK Match = %q", hk.Match)
	}
	if hk.Strategy != "select" {
		t.Errorf("HK Strategy = %q", hk.Strategy)
	}

	// Japan strategy is url-test
	jp, ok := cfg.Groups.Get("🇯🇵 Japan")
	if !ok {
		t.Fatal("group 🇯🇵 Japan not found")
	}
	if jp.Strategy != "url-test" {
		t.Errorf("JP Strategy = %q, want url-test", jp.Strategy)
	}

	// first routing entry members (uses @auto shorthand)
	fast, ok := cfg.Routing.Get("🚀 快速选择")
	if !ok {
		t.Fatal("routing 🚀 快速选择 not found")
	}
	if len(fast) != 1 || fast[0] != "@auto" {
		t.Errorf("🚀 快速选择 members = %v, want [@auto]", fast)
	}

	// rulesets: BanList has 3 URLs
	ban, ok := cfg.Rulesets.Get("🛑 BanList")
	if !ok {
		t.Fatal("ruleset 🛑 BanList not found")
	}
	if len(ban) != 3 {
		t.Errorf("BanList URLs count = %d, want 3", len(ban))
	}

	// rules
	if len(cfg.Rules) != 1 || cfg.Rules[0] != "GEOIP,CN,🎯 China" {
		t.Errorf("Rules = %v", cfg.Rules)
	}

	// filters
	if cfg.Filters.Exclude != "(过期|剩余流量|到期)" {
		t.Errorf("Filters.Exclude = %q", cfg.Filters.Exclude)
	}

	// custom proxy
	cp := cfg.Sources.CustomProxies[0]
	if cp.RelayThrough.Name != "🇭🇰 Hong Kong" {
		t.Errorf("RelayThrough.Name = %q", cp.RelayThrough.Name)
	}
}

// TestSources_FetchOrderPreserved: FetchOrder records fetch-kind keys in YAML
// declaration order. Subscriptions/snell/vless each appear exactly when (and
// in the order) they are declared. custom_proxies does not enter FetchOrder.
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
			want: []string{"vless", "snell"}, // custom_proxies excluded
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
				t.Fatalf("unmarshal: %v", err)
			}
			if !slices.Equal(s.FetchOrder, tc.want) {
				t.Errorf("FetchOrder = %v, want %v", s.FetchOrder, tc.want)
			}
		})
	}
}

// TestSources_RejectsDuplicateKey: the same top-level key appearing twice
// (e.g. `vless:` declared twice in one sources block) is an error.
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
		t.Fatal("expected duplicate-key error, got nil")
	}
}

// TestSources_RejectsUnknownKey: typos like `vles` or `subscrition` fail loudly
// rather than being silently ignored.
func TestSources_RejectsUnknownKey(t *testing.T) {
	y := `
vles:
  - url: https://example.com/v.txt
`
	var s Sources
	err := yaml.Unmarshal([]byte(y), &s)
	if err == nil {
		t.Fatal("expected unknown-key error, got nil")
	}
}

func mustLoadExample(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load(context.Background(), exampleConfigPath, nil)
	if err != nil {
		t.Fatalf("Load example config: %v", err)
	}
	return cfg
}
