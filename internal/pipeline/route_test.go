package pipeline

import (
	"errors"
	"fmt"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

func routeMemberLabel(member model.PolicyRef) string {
	if member.Kind == model.PolicyRefProxy {
		return fmt.Sprintf("#%d", member.ProxyID)
	}
	return member.Name
}

func routeMemberLabels(members []model.PolicyRef) []string {
	labels := make([]string, 0, len(members))
	for _, member := range members {
		labels = append(labels, routeMemberLabel(member))
	}
	return labels
}

// --- Route 测试 ---

// T-RTE-001：由 routing 生成服务组（顺序 + 策略）
func TestRoute_ServiceGroups(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"📲 Telegram": ["🇭🇰 Hong Kong", "DIRECT"]
"📺 Netflix": ["🇸🇬 Singapore", "🇭🇰 Hong Kong"]
"🐟 FINAL": ["🚀 快速选择", "DIRECT"]
`),
		Fallback: "🐟 FINAL",
	}

	result, err := routeFromConfig(cfg, &GroupResult{AllProxies: []model.ProxyID{1, 2}})
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result.RouteGroups) != 3 {
		t.Fatalf("得到 %d 个路由组，期望 3 个", len(result.RouteGroups))
	}

	// 顺序与声明一致。
	wantNames := []string{"📲 Telegram", "📺 Netflix", "🐟 FINAL"}
	for i, want := range wantNames {
		if result.RouteGroups[i].Name != want {
			t.Errorf("group[%d].Name = %q，期望 %q", i, result.RouteGroups[i].Name, want)
		}
		if result.RouteGroups[i].Strategy != "select" {
			t.Errorf("group[%d].Strategy = %q，期望 select", i, result.RouteGroups[i].Strategy)
		}
	}

	// 校验成员。
	if len(result.RouteGroups[0].Members) != 2 {
		t.Errorf("Telegram 成员 = %v，期望 2 个", result.RouteGroups[0].Members)
	}
}

// T-RTE-002：服务组成员中的 @all 展开
func TestRoute_AllExpansion(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 手动切换": ["@all"]
"🚀 混合": ["🇭🇰 Hong Kong", "@all", "DIRECT"]
`),
		Fallback: "🐟 FINAL",
	}

	allProxies := []model.ProxyID{1, 2, 3}

	result, err := routeFromConfig(cfg, &GroupResult{AllProxies: allProxies})
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// 只有 @all 的展开。
	manual := result.RouteGroups[0]
	if len(manual.Members) != 3 {
		t.Fatalf("手动切换 成员 = %v，期望 3 个", manual.Members)
	}
	for i, want := range []string{"#1", "#2", "#3"} {
		if got := routeMemberLabel(manual.Members[i]); got != want {
			t.Errorf("Members[%d] = %q，期望 %q", i, got, want)
		}
	}

	// 混合展开。
	mixed := result.RouteGroups[1]
	wantMixed := []string{"🇭🇰 Hong Kong", "#1", "#2", "#3", "DIRECT"}
	if len(mixed.Members) != len(wantMixed) {
		t.Fatalf("混合 成员 = %v，期望 %v", mixed.Members, wantMixed)
	}
	for i, want := range wantMixed {
		if got := routeMemberLabel(mixed.Members[i]); got != want {
			t.Errorf("Mixed.Members[%d] = %q，期望 %q", i, got, want)
		}
	}
}

// T-RTE-003：Rulesets 映射
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

	result, err := routeFromConfig(cfg, &GroupResult{})
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result.Rulesets) != 2 {
		t.Fatalf("得到 %d 个规则集，期望 2 个", len(result.Rulesets))
	}

	// 保持顺序。
	if result.Rulesets[0].Policy != "📲 Telegram" {
		t.Errorf("ruleset[0].Policy = %q，期望 Telegram", result.Rulesets[0].Policy)
	}
	if len(result.Rulesets[0].URLs) != 1 {
		t.Fatalf("Telegram URL 数量 = %d，期望 1", len(result.Rulesets[0].URLs))
	}
	if got := result.Rulesets[0].URLs[0]; got != "https://example.com/telegram.list" {
		t.Errorf("Telegram URL = %q，期望 https://example.com/telegram.list", got)
	}

	if result.Rulesets[1].Policy != "🎯 China" {
		t.Errorf("ruleset[1].Policy = %q，期望 China", result.Rulesets[1].Policy)
	}
	if len(result.Rulesets[1].URLs) != 2 {
		t.Errorf("China URL 数量 = %d，期望 2", len(result.Rulesets[1].URLs))
	}
}

// T-RTE-004：Rules 解析
func TestRoute_RulesParsing(t *testing.T) {
	cfg := &config.Config{
		Rules: []string{
			"GEOIP,CN,🎯 China",
			"DOMAIN-SUFFIX,google.com,🔍 Google",
		},
		Fallback: "🐟 FINAL",
	}

	result, err := routeFromConfig(cfg, &GroupResult{})
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result.Rules) != 2 {
		t.Fatalf("得到 %d 条规则，期望 2 条", len(result.Rules))
	}

	if result.Rules[0].Raw != "GEOIP,CN,🎯 China" {
		t.Errorf("rule[0].Raw = %q", result.Rules[0].Raw)
	}
	if result.Rules[0].Policy != "🎯 China" {
		t.Errorf("rule[0].Policy = %q，期望 %q", result.Rules[0].Policy, "🎯 China")
	}

	if result.Rules[1].Policy != "🔍 Google" {
		t.Errorf("rule[1].Policy = %q，期望 %q", result.Rules[1].Policy, "🔍 Google")
	}
}

// T-RTE-005：没有逗号的规则
func TestRoute_RuleNoComma(t *testing.T) {
	cfg := &config.Config{
		Rules:    []string{"INVALID-RULE"},
		Fallback: "🐟 FINAL",
	}

	_, err := routeFromConfig(cfg, &GroupResult{})
	if err == nil {
		t.Fatal("没有逗号的规则应返回错误")
	}

	var configErr *errtype.ConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.ConfigError", err)
	}
	if configErr.Field != "rules[0]" {
		t.Errorf("Field = %q，期望 %q", configErr.Field, "rules[0]")
	}
}

// T-RTE-006：Fallback 原样传递
func TestRoute_Fallback(t *testing.T) {
	cfg := &config.Config{
		Fallback: "🐟 FINAL",
	}

	result, err := routeFromConfig(cfg, &GroupResult{})
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if result.Fallback != "🐟 FINAL" {
		t.Errorf("Fallback = %q，期望 %q", result.Fallback, "🐟 FINAL")
	}
}

// T-RTE-007：空 routing
func TestRoute_EmptyRouting(t *testing.T) {
	cfg := &config.Config{
		Fallback: "🐟 FINAL",
	}

	result, err := routeFromConfig(cfg, &GroupResult{})
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if len(result.RouteGroups) != 0 {
		t.Errorf("得到 %d 个路由组，期望 0 个", len(result.RouteGroups))
	}
	if len(result.Rulesets) != 0 {
		t.Errorf("得到 %d 个规则集，期望 0 个", len(result.Rulesets))
	}
	if len(result.Rules) != 0 {
		t.Errorf("得到 %d 条规则，期望 0 条", len(result.Rules))
	}
}

// T-RTE-008：allProxies 为空时的 @all 展开
func TestRoute_AllExpansionEmpty(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 手动切换": ["🇭🇰 Hong Kong", "@all", "DIRECT"]
`),
		Fallback: "🐟 FINAL",
	}

	result, err := routeFromConfig(cfg, &GroupResult{})
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	// @all 展开为空，只留下其他成员。
	wantMembers := []string{"🇭🇰 Hong Kong", "DIRECT"}
	members := result.RouteGroups[0].Members
	if len(members) != len(wantMembers) {
		t.Fatalf("成员 = %v，期望 %v", members, wantMembers)
	}
	for i, want := range wantMembers {
		if got := routeMemberLabel(members[i]); got != want {
			t.Errorf("Members[%d] = %q，期望 %q", i, got, want)
		}
	}
}

func TestRoute_ResultMembersContainExpandedValuesOnly(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 快速选择": ["@auto"]
"🚀 手动切换": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	result, err := routeFromConfig(cfg, &GroupResult{
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 Hong Kong"},
		},
		AllProxies: []model.ProxyID{1, 2},
	})
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	if got := routeMemberLabels(result.RouteGroups[0].Members); len(got) != 3 || got[0] != "🇭🇰 Hong Kong" || got[1] != "🚀 手动切换" || got[2] != "DIRECT" {
		t.Fatalf("快速选择 成员 = %v，期望 [🇭🇰 Hong Kong 🚀 手动切换 DIRECT]", got)
	}
	if got := routeMemberLabels(result.RouteGroups[1].Members); len(got) != 2 || got[0] != "#1" || got[1] != "#2" {
		t.Fatalf("手动切换 成员 = %v，期望 [#1 #2]", got)
	}
}

// --- @auto 测试 ---

// TestRoute_AutoFillBasic：[@auto] 展开为全部节点组 + 含 @all 的路由组 + DIRECT
func TestRoute_AutoFillBasic(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 快速选择": ["@auto"]
"🚀 手动切换": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 Hong Kong"},
			{Name: "🇸🇬 Singapore"},
			{Name: "🔗 HK-ISP"},
		},
		AllProxies: []model.ProxyID{1, 2, 3},
	}

	result, err := routeFromConfig(cfg, gr)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	quick := result.RouteGroups[0]
	want := []string{
		"🇭🇰 Hong Kong", "🇸🇬 Singapore", "🔗 HK-ISP", // 节点组
		"🚀 手动切换", // 含 @all 的路由组
		"DIRECT",
	}
	if len(quick.Members) != len(want) {
		t.Fatalf("快速选择 成员 = %v，期望 %v", quick.Members, want)
	}
	for i, w := range want {
		if got := routeMemberLabel(quick.Members[i]); got != w {
			t.Errorf("Members[%d] = %q，期望 %q", i, got, w)
		}
	}
}

// TestRoute_AutoFillWithPreferred：手写的优先项排在自动填充之前，且不重复
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
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 Hong Kong"},
			{Name: "🇸🇬 Singapore"},
			{Name: "🔗 HK-ISP"},
		},
		AllProxies: []model.ProxyID{1, 2},
	}

	result, err := routeFromConfig(cfg, gr)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	telegram := result.RouteGroups[0]
	want := []string{
		"🇭🇰 Hong Kong", // 优先项
		"🚀 快速选择",       // 优先项
		"🇸🇬 Singapore", // 自动填充（HK 已在优先项中）
		"🔗 HK-ISP",     // 自动填充（链式组）
		"🚀 手动切换",       // 自动填充（含 @all 的路由组）
		"DIRECT",
	}
	if len(telegram.Members) != len(want) {
		t.Fatalf("Telegram 成员 = %v，\n期望 %v", telegram.Members, want)
	}
	for i, w := range want {
		if got := routeMemberLabel(telegram.Members[i]); got != w {
			t.Errorf("Members[%d] = %q，期望 %q", i, got, w)
		}
	}
}

func TestRoute_AutoFillPreservesManualRejectPlacement(t *testing.T) {
	gr := &GroupResult{
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 Hong Kong"},
			{Name: "🇸🇬 Singapore"},
			{Name: "🔗 HK-ISP"},
		},
		AllProxies: []model.ProxyID{1, 2},
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

			result, err := routeFromConfig(cfg, gr)
			if err != nil {
				t.Fatalf("意外错误：%v", err)
			}

			telegram := result.RouteGroups[0]
			if len(telegram.Members) != len(tt.want) {
				t.Fatalf("Telegram 成员 = %v，期望 %v", telegram.Members, tt.want)
			}
			for i, w := range tt.want {
				if got := routeMemberLabel(telegram.Members[i]); got != w {
					t.Errorf("Members[%d] = %q，期望 %q", i, got, w)
				}
			}
		})
	}
}

// TestRoute_AutoFillExcludesSelf：自动填充时组不会包含自己
func TestRoute_AutoFillExcludesSelf(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🚀 快速选择": ["@auto"]
"🚀 手动切换": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 Hong Kong"},
		},
		AllProxies: []model.ProxyID{1},
	}

	result, err := routeFromConfig(cfg, gr)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	quick := result.RouteGroups[0]
	for _, m := range quick.Members {
		if routeMemberLabel(m) == "🚀 快速选择" {
			t.Error("快速选择 不应包含自己")
		}
	}
}

// TestRoute_AutoFillIncludesChainedGroups：链式组会出现在自动填充结果中
func TestRoute_AutoFillIncludesChainedGroups(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"📺 Netflix": ["🇸🇬 Singapore", "@auto"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 Hong Kong"},
			{Name: "🇸🇬 Singapore"},
			{Name: "🔗 HK-ISP"},
			{Name: "🔗 SG-ISP"},
		},
	}

	result, err := routeFromConfig(cfg, gr)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	netflix := result.RouteGroups[0]
	// SG 是优先项；HK、🔗 HK-ISP、🔗 SG-ISP 应被自动填充
	found := make(map[string]bool)
	for _, m := range netflix.Members {
		found[routeMemberLabel(m)] = true
	}
	for _, want := range []string{"🔗 HK-ISP", "🔗 SG-ISP", "🇭🇰 Hong Kong"} {
		if !found[want] {
			t.Errorf("Netflix 成员中应有 %q，实际 %v", want, netflix.Members)
		}
	}
}

// TestRoute_AutoFillIncludesAllRouteGroups：含 @all 的路由组会出现在自动填充结果中
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
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 Hong Kong"},
		},
		AllProxies: []model.ProxyID{1},
	}

	result, err := routeFromConfig(cfg, gr)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	telegram := result.RouteGroups[0]
	found := make(map[string]bool)
	for _, m := range telegram.Members {
		found[routeMemberLabel(m)] = true
	}
	if !found["🚀 手动切换"] {
		t.Error("自动填充结果中应有 🚀 手动切换")
	}
	if !found["🔀 另一个全选"] {
		t.Error("自动填充结果中应有 🔀 另一个全选")
	}
}

// TestRoute_AutoFillOrder：校验候选池顺序：节点组 → 含 @all 的路由组 → DIRECT
func TestRoute_AutoFillOrder(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🐟 FINAL": ["@auto"]
"🚀 手动切换": ["@all"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 Hong Kong"},
			{Name: "🇸🇬 Singapore"},
			{Name: "🔗 HK-ISP"},
		},
	}

	result, err := routeFromConfig(cfg, gr)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	final := result.RouteGroups[0]
	want := []string{
		"🇭🇰 Hong Kong", // 节点组 1
		"🇸🇬 Singapore", // 节点组 2
		"🔗 HK-ISP",     // 链式组
		"🚀 手动切换",       // 含 @all 的路由组
		"DIRECT",
	}
	if len(final.Members) != len(want) {
		t.Fatalf("FINAL 成员 = %v，期望 %v", final.Members, want)
	}
	for i, w := range want {
		if got := routeMemberLabel(final.Members[i]); got != w {
			t.Errorf("Members[%d] = %q，期望 %q", i, got, w)
		}
	}
}

// TestRoute_NoAutoFill：不含 @auto 的条目行为不变（向后兼容）
func TestRoute_NoAutoFill(t *testing.T) {
	cfg := &config.Config{
		Routing: mustRoutingMap(t, `
"🛑 BanList": ["REJECT", "DIRECT"]
`),
		Fallback: "🐟 FINAL",
	}

	gr := &GroupResult{
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 Hong Kong"},
		},
	}

	result, err := routeFromConfig(cfg, gr)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	ban := result.RouteGroups[0]
	want := []string{"REJECT", "DIRECT"}
	if len(ban.Members) != len(want) {
		t.Fatalf("BanList 成员 = %v，期望 %v", ban.Members, want)
	}
	for i, w := range want {
		if got := routeMemberLabel(ban.Members[i]); got != w {
			t.Errorf("Members[%d] = %q，期望 %q", i, got, w)
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

	result, err := routeFromConfig(cfg, nil)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}

	telegram := result.RouteGroups[0]
	want := []string{"🚀 手动切换", "DIRECT"}
	if len(telegram.Members) != len(want) {
		t.Fatalf("Telegram 成员 = %v，期望 %v", telegram.Members, want)
	}
	for i, w := range want {
		if got := routeMemberLabel(telegram.Members[i]); got != w {
			t.Errorf("Members[%d] = %q，期望 %q", i, got, w)
		}
	}
}
