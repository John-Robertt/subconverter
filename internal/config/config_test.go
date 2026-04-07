package config

import (
	"slices"
	"testing"
)

const exampleConfigPath = "../../configs/example.yaml"

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

	// first routing entry members
	fast, ok := cfg.Routing.Get("🚀 快速选择")
	if !ok {
		t.Fatal("routing 🚀 快速选择 not found")
	}
	if fast[0] != "🇭🇰 Hong Kong" || fast[len(fast)-1] != "DIRECT" {
		t.Errorf("🚀 快速选择 members = %v", fast)
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

func mustLoadExample(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load(exampleConfigPath)
	if err != nil {
		t.Fatalf("Load example config: %v", err)
	}
	return cfg
}
