package pipeline

import (
	"errors"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// --- Route tests ---

// T-RTE-001: Service groups from routing (order + strategy)
func TestRoute_ServiceGroups(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"📲 Telegram": ["🇭🇰 Hong Kong", "DIRECT"]
"📺 Netflix": ["🇸🇬 Singapore", "🇭🇰 Hong Kong"]
"🐟 FINAL": ["🚀 快速选择", "DIRECT"]
`),
		Fallback: "🐟 FINAL",
	}

	result, err := Route(cfg, []string{"HK-01", "SG-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.RouteGroups) != 3 {
		t.Fatalf("got %d route groups, want 3", len(result.RouteGroups))
	}

	// Order matches declaration.
	wantNames := []string{"📲 Telegram", "📺 Netflix", "🐟 FINAL"}
	for i, want := range wantNames {
		if result.RouteGroups[i].Name != want {
			t.Errorf("group[%d].Name = %q, want %q", i, result.RouteGroups[i].Name, want)
		}
		if result.RouteGroups[i].Scope != model.ScopeRoute {
			t.Errorf("group[%d].Scope = %q, want route", i, result.RouteGroups[i].Scope)
		}
		if result.RouteGroups[i].Strategy != "select" {
			t.Errorf("group[%d].Strategy = %q, want select", i, result.RouteGroups[i].Strategy)
		}
	}

	// Verify members.
	if len(result.RouteGroups[0].Members) != 2 {
		t.Errorf("Telegram members = %v, want 2", result.RouteGroups[0].Members)
	}
}

// T-RTE-002: @all expansion in service group members
func TestRoute_AllExpansion(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 手动切换": ["@all"]
"🚀 混合": ["🇭🇰 Hong Kong", "@all", "DIRECT"]
`),
		Fallback: "🐟 FINAL",
	}

	allProxies := []string{"HK-01", "SG-01", "JP-01"}

	result, err := Route(cfg, allProxies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pure @all expansion.
	manual := result.RouteGroups[0]
	if len(manual.Members) != 3 {
		t.Fatalf("手动切换 members = %v, want 3", manual.Members)
	}
	for i, want := range allProxies {
		if manual.Members[i] != want {
			t.Errorf("Members[%d] = %q, want %q", i, manual.Members[i], want)
		}
	}

	// Mixed expansion.
	mixed := result.RouteGroups[1]
	wantMixed := []string{"🇭🇰 Hong Kong", "HK-01", "SG-01", "JP-01", "DIRECT"}
	if len(mixed.Members) != len(wantMixed) {
		t.Fatalf("混合 members = %v, want %v", mixed.Members, wantMixed)
	}
	for i, want := range wantMixed {
		if mixed.Members[i] != want {
			t.Errorf("Mixed.Members[%d] = %q, want %q", i, mixed.Members[i], want)
		}
	}
}

// T-RTE-003: Rulesets mapping
func TestRoute_Rulesets(t *testing.T) {
	cfg := &config.Config{
		Rulesets: mustRoutingMap(t, `
"📲 Telegram":
  - "https://example.com/telegram.list"
"🎯 China":
  - "https://example.com/china1.list"
  - "https://example.com/china2.list"
`),
		Fallback: "🐟 FINAL",
	}

	result, err := Route(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Rulesets) != 2 {
		t.Fatalf("got %d rulesets, want 2", len(result.Rulesets))
	}

	// Order preserved.
	if result.Rulesets[0].Policy != "📲 Telegram" {
		t.Errorf("ruleset[0].Policy = %q, want Telegram", result.Rulesets[0].Policy)
	}
	if len(result.Rulesets[0].URLs) != 1 {
		t.Errorf("Telegram URLs count = %d, want 1", len(result.Rulesets[0].URLs))
	}

	if result.Rulesets[1].Policy != "🎯 China" {
		t.Errorf("ruleset[1].Policy = %q, want China", result.Rulesets[1].Policy)
	}
	if len(result.Rulesets[1].URLs) != 2 {
		t.Errorf("China URLs count = %d, want 2", len(result.Rulesets[1].URLs))
	}
}

// T-RTE-004: Rules parsing
func TestRoute_RulesParsing(t *testing.T) {
	cfg := &config.Config{
		Rules: []string{
			"GEOIP,CN,🎯 China",
			"DOMAIN-SUFFIX,google.com,🔍 Google",
		},
		Fallback: "🐟 FINAL",
	}

	result, err := Route(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(result.Rules))
	}

	if result.Rules[0].Raw != "GEOIP,CN,🎯 China" {
		t.Errorf("rule[0].Raw = %q", result.Rules[0].Raw)
	}
	if result.Rules[0].Policy != "🎯 China" {
		t.Errorf("rule[0].Policy = %q, want %q", result.Rules[0].Policy, "🎯 China")
	}

	if result.Rules[1].Policy != "🔍 Google" {
		t.Errorf("rule[1].Policy = %q, want %q", result.Rules[1].Policy, "🔍 Google")
	}
}

// T-RTE-005: Rule with no comma
func TestRoute_RuleNoComma(t *testing.T) {
	cfg := &config.Config{
		Rules:    []string{"INVALID-RULE"},
		Fallback: "🐟 FINAL",
	}

	_, err := Route(cfg, nil)
	if err == nil {
		t.Fatal("expected error for rule without comma")
	}

	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) {
		t.Fatalf("error type = %T, want *errtype.BuildError", err)
	}
	if buildErr.Phase != "route" {
		t.Errorf("Phase = %q, want %q", buildErr.Phase, "route")
	}
}

// T-RTE-006: Fallback passthrough
func TestRoute_Fallback(t *testing.T) {
	cfg := &config.Config{
		Fallback: "🐟 FINAL",
	}

	result, err := Route(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Fallback != "🐟 FINAL" {
		t.Errorf("Fallback = %q, want %q", result.Fallback, "🐟 FINAL")
	}
}

// T-RTE-007: Empty routing
func TestRoute_EmptyRouting(t *testing.T) {
	cfg := &config.Config{
		Fallback: "🐟 FINAL",
	}

	result, err := Route(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.RouteGroups) != 0 {
		t.Errorf("got %d route groups, want 0", len(result.RouteGroups))
	}
	if len(result.Rulesets) != 0 {
		t.Errorf("got %d rulesets, want 0", len(result.Rulesets))
	}
	if len(result.Rules) != 0 {
		t.Errorf("got %d rules, want 0", len(result.Rules))
	}
}

// T-RTE-008: @all expansion with empty allProxies
func TestRoute_AllExpansionEmpty(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 手动切换": ["🇭🇰 Hong Kong", "@all", "DIRECT"]
`),
		Fallback: "🐟 FINAL",
	}

	result, err := Route(cfg, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// @all expands to nothing, leaving other members.
	wantMembers := []string{"🇭🇰 Hong Kong", "DIRECT"}
	members := result.RouteGroups[0].Members
	if len(members) != len(wantMembers) {
		t.Fatalf("members = %v, want %v", members, wantMembers)
	}
	for i, want := range wantMembers {
		if members[i] != want {
			t.Errorf("Members[%d] = %q, want %q", i, members[i], want)
		}
	}
}
