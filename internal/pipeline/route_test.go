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

	result, err := Route(cfg, &GroupResult{AllProxies: []string{"HK-01", "SG-01"}})
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

	result, err := Route(cfg, &GroupResult{AllProxies: allProxies})
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

	result, err := Route(cfg, &GroupResult{})
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
		t.Fatalf("Telegram URLs count = %d, want 1", len(result.Rulesets[0].URLs))
	}
	if got := result.Rulesets[0].URLs[0]; got != "https://example.com/telegram.list" {
		t.Errorf("Telegram URL = %q, want https://example.com/telegram.list", got)
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

	result, err := Route(cfg, &GroupResult{})
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

	_, err := Route(cfg, &GroupResult{})
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

	result, err := Route(cfg, &GroupResult{})
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

	result, err := Route(cfg, &GroupResult{})
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

	result, err := Route(cfg, &GroupResult{})
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

func TestRoute_RawMembersPreserved(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 手动切换": ["@all"]
"📲 Telegram": ["🇭🇰 Hong Kong", "DIRECT"]
`),
		Fallback: "🐟 FINAL",
	}

	result, err := Route(cfg, &GroupResult{AllProxies: []string{"HK-01", "SG-01"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.RawRouteMembers["🚀 手动切换"]; len(got) != 1 || got[0] != "@all" {
		t.Fatalf("raw members for 手动切换 = %v, want [@all]", got)
	}
	if got := result.RawRouteMembers["📲 Telegram"]; len(got) != 2 || got[0] != "🇭🇰 Hong Kong" || got[1] != "DIRECT" {
		t.Fatalf("raw members for Telegram = %v, want [🇭🇰 Hong Kong DIRECT]", got)
	}
}

// --- @auto tests ---

// TestRoute_AutoFillBasic: [@auto] expands to all node groups + @all route groups + DIRECT
func TestRoute_AutoFillBasic(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 快速选择": ["@auto"]
"🚀 手动切换": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 Hong Kong", Scope: model.ScopeNode},
			{Name: "🇸🇬 Singapore", Scope: model.ScopeNode},
			{Name: "🔗 HK-ISP", Scope: model.ScopeNode},
		},
		AllProxies: []string{"HK-01", "SG-01", "HK-ISP"},
	}

	result, err := Route(cfg, gr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	quick := result.RouteGroups[0]
	want := []string{
		"🇭🇰 Hong Kong", "🇸🇬 Singapore", "🔗 HK-ISP", // node groups
		"🚀 手动切换", // @all route group
		"DIRECT",
	}
	if len(quick.Members) != len(want) {
		t.Fatalf("快速选择 members = %v, want %v", quick.Members, want)
	}
	for i, w := range want {
		if quick.Members[i] != w {
			t.Errorf("Members[%d] = %q, want %q", i, quick.Members[i], w)
		}
	}
}

// TestRoute_AutoFillWithPreferred: preferred items come before auto-fill, no duplicates
func TestRoute_AutoFillWithPreferred(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"📲 Telegram": ["🇭🇰 Hong Kong", "🚀 快速选择", "@auto"]
"🚀 快速选择": ["@auto"]
"🚀 手动切换": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 Hong Kong", Scope: model.ScopeNode},
			{Name: "🇸🇬 Singapore", Scope: model.ScopeNode},
			{Name: "🔗 HK-ISP", Scope: model.ScopeNode},
		},
		AllProxies: []string{"HK-01", "SG-01"},
	}

	result, err := Route(cfg, gr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	telegram := result.RouteGroups[0]
	want := []string{
		"🇭🇰 Hong Kong", // preferred
		"🚀 快速选择",       // preferred
		"🇸🇬 Singapore", // auto-fill (HK already in preferred)
		"🔗 HK-ISP",     // auto-fill (chained)
		"🚀 手动切换",       // auto-fill (@all route group)
		"DIRECT",
	}
	if len(telegram.Members) != len(want) {
		t.Fatalf("Telegram members = %v,\nwant %v", telegram.Members, want)
	}
	for i, w := range want {
		if telegram.Members[i] != w {
			t.Errorf("Members[%d] = %q, want %q", i, telegram.Members[i], w)
		}
	}
}

func TestRoute_AutoFillPreservesManualRejectPlacement(t *testing.T) {
	gr := &GroupResult{
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 Hong Kong", Scope: model.ScopeNode},
			{Name: "🇸🇬 Singapore", Scope: model.ScopeNode},
			{Name: "🔗 HK-ISP", Scope: model.ScopeNode},
		},
		AllProxies: []string{"HK-01", "SG-01"},
	}

	tests := []struct {
		name    string
		routing string
		want    []string
	}{
		{
			name: "reject after auto",
			routing: `
"📲 Telegram": ["🇭🇰 Hong Kong", "🚀 快速选择", "@auto", "REJECT"]
"🚀 快速选择": ["@auto"]
"🚀 手动切换": ["@all"]
`,
			want: []string{"🇭🇰 Hong Kong", "🚀 快速选择", "🇸🇬 Singapore", "🔗 HK-ISP", "🚀 手动切换", "DIRECT", "REJECT"},
		},
		{
			name: "reject before auto",
			routing: `
"📲 Telegram": ["🇭🇰 Hong Kong", "🚀 快速选择", "REJECT", "@auto"]
"🚀 快速选择": ["@auto"]
"🚀 手动切换": ["@all"]
`,
			want: []string{"🇭🇰 Hong Kong", "🚀 快速选择", "REJECT", "🇸🇬 Singapore", "🔗 HK-ISP", "🚀 手动切换", "DIRECT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Routing:  mustRoutingMap(t, tt.routing),
				Fallback: "🐟 FINAL",
			}

			result, err := Route(cfg, gr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			telegram := result.RouteGroups[0]
			if len(telegram.Members) != len(tt.want) {
				t.Fatalf("Telegram members = %v, want %v", telegram.Members, tt.want)
			}
			for i, w := range tt.want {
				if telegram.Members[i] != w {
					t.Errorf("Members[%d] = %q, want %q", i, telegram.Members[i], w)
				}
			}
		})
	}
}

// TestRoute_AutoFillExcludesSelf: a group does not include itself in auto-fill
func TestRoute_AutoFillExcludesSelf(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 快速选择": ["@auto"]
"🚀 手动切换": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 Hong Kong", Scope: model.ScopeNode},
		},
		AllProxies: []string{"HK-01"},
	}

	result, err := Route(cfg, gr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	quick := result.RouteGroups[0]
	for _, m := range quick.Members {
		if m == "🚀 快速选择" {
			t.Error("快速选择 should not contain itself")
		}
	}
}

// TestRoute_AutoFillIncludesChainedGroups: chained groups appear in auto-fill
func TestRoute_AutoFillIncludesChainedGroups(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"📺 Netflix": ["🇸🇬 Singapore", "@auto"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 Hong Kong", Scope: model.ScopeNode},
			{Name: "🇸🇬 Singapore", Scope: model.ScopeNode},
			{Name: "🔗 HK-ISP", Scope: model.ScopeNode},
			{Name: "🔗 SG-ISP", Scope: model.ScopeNode},
		},
	}

	result, err := Route(cfg, gr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	netflix := result.RouteGroups[0]
	// SG is preferred; HK, 🔗 HK-ISP, 🔗 SG-ISP should be auto-filled
	found := make(map[string]bool)
	for _, m := range netflix.Members {
		found[m] = true
	}
	for _, want := range []string{"🔗 HK-ISP", "🔗 SG-ISP", "🇭🇰 Hong Kong"} {
		if !found[want] {
			t.Errorf("expected %q in Netflix members, got %v", want, netflix.Members)
		}
	}
}

// TestRoute_AutoFillIncludesAllRouteGroups: route groups containing @all appear in auto-fill
func TestRoute_AutoFillIncludesAllRouteGroups(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"📲 Telegram": ["@auto"]
"🚀 手动切换": ["@all"]
"🔀 另一个全选": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 Hong Kong", Scope: model.ScopeNode},
		},
		AllProxies: []string{"HK-01"},
	}

	result, err := Route(cfg, gr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	telegram := result.RouteGroups[0]
	found := make(map[string]bool)
	for _, m := range telegram.Members {
		found[m] = true
	}
	if !found["🚀 手动切换"] {
		t.Error("expected 🚀 手动切换 in auto-fill")
	}
	if !found["🔀 另一个全选"] {
		t.Error("expected 🔀 另一个全选 in auto-fill")
	}
}

// TestRoute_AutoFillOrder: verify pool order: node groups → @all route groups → DIRECT
func TestRoute_AutoFillOrder(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🐟 FINAL": ["@auto"]
"🚀 手动切换": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 Hong Kong", Scope: model.ScopeNode},
			{Name: "🇸🇬 Singapore", Scope: model.ScopeNode},
			{Name: "🔗 HK-ISP", Scope: model.ScopeNode},
		},
	}

	result, err := Route(cfg, gr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	final := result.RouteGroups[0]
	want := []string{
		"🇭🇰 Hong Kong", // node group 1
		"🇸🇬 Singapore", // node group 2
		"🔗 HK-ISP",     // chained group
		"🚀 手动切换",       // @all route group
		"DIRECT",
	}
	if len(final.Members) != len(want) {
		t.Fatalf("FINAL members = %v, want %v", final.Members, want)
	}
	for i, w := range want {
		if final.Members[i] != w {
			t.Errorf("Members[%d] = %q, want %q", i, final.Members[i], w)
		}
	}
}

// TestRoute_NoAutoFill: entries without @auto work as before (backward compatible)
func TestRoute_NoAutoFill(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🛑 BanList": ["REJECT", "DIRECT"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 Hong Kong", Scope: model.ScopeNode},
		},
	}

	result, err := Route(cfg, gr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ban := result.RouteGroups[0]
	want := []string{"REJECT", "DIRECT"}
	if len(ban.Members) != len(want) {
		t.Fatalf("BanList members = %v, want %v", ban.Members, want)
	}
	for i, w := range want {
		if ban.Members[i] != w {
			t.Errorf("Members[%d] = %q, want %q", i, ban.Members[i], w)
		}
	}
}

func TestRoute_NilGroupResult(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"📲 Telegram": ["🚀 手动切换", "@auto"]
"🚀 手动切换": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	result, err := Route(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	telegram := result.RouteGroups[0]
	want := []string{"🚀 手动切换", "DIRECT"}
	if len(telegram.Members) != len(want) {
		t.Fatalf("Telegram members = %v, want %v", telegram.Members, want)
	}
	for i, w := range want {
		if telegram.Members[i] != w {
			t.Errorf("Members[%d] = %q, want %q", i, telegram.Members[i], w)
		}
	}
}
