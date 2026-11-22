package config

import "testing"

// T-CFG-016：Prepare 保留服务组成员在声明和展开后的来源
func TestPrepare_RouteInputPreservesDeclaredAndExpandedOrigins(t *testing.T) {
	cfg := validBase()
	cfg.Groups = mustOrderedMap[Group](`
"HK": { match: "(HK)", strategy: select }
"SG": { match: "(SG)", strategy: select }
`)
	cfg.Routing = mustOrderedMap[[]string](`
"Quick": ["@auto"]
"Manual": ["@all"]
`)
	cfg.Rulesets = OrderedMap[[]string]{}
	cfg.Rules = nil
	cfg.Fallback = "Quick"

	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("Prepare() 错误 = %v", err)
	}

	routing, _, _, fallback := rt.RouteInput()
	if fallback != "Quick" {
		t.Fatalf("fallback = %q，期望 Quick", fallback)
	}
	if len(routing) != 2 {
		t.Fatalf("routing 长度 = %d，期望 2", len(routing))
	}

	quick := routing[0]
	if len(quick.DeclaredMembers) != 1 || quick.DeclaredMembers[0].Raw != "@auto" || quick.DeclaredMembers[0].Origin != RouteMemberOriginLiteral {
		t.Fatalf("Quick 声明成员 = %+v，期望一个字面量 @auto", quick.DeclaredMembers)
	}
	wantExpanded := []PreparedRouteMember{
		{Raw: "HK", Origin: RouteMemberOriginAutoExpanded},
		{Raw: "SG", Origin: RouteMemberOriginAutoExpanded},
		{Raw: "Manual", Origin: RouteMemberOriginAutoExpanded},
		{Raw: "DIRECT", Origin: RouteMemberOriginAutoExpanded},
	}
	if len(quick.ExpandedMembers) != len(wantExpanded) {
		t.Fatalf("Quick 展开成员数 = %d，期望 %d", len(quick.ExpandedMembers), len(wantExpanded))
	}
	for i, want := range wantExpanded {
		if quick.ExpandedMembers[i] != want {
			t.Fatalf("Quick 展开成员[%d] = %+v，期望 %+v", i, quick.ExpandedMembers[i], want)
		}
	}

	manual := routing[1]
	if len(manual.DeclaredMembers) != 1 || manual.DeclaredMembers[0].Raw != "@all" || manual.DeclaredMembers[0].Origin != RouteMemberOriginLiteral {
		t.Fatalf("Manual 声明成员 = %+v，期望一个字面量 @all", manual.DeclaredMembers)
	}
	if len(manual.ExpandedMembers) != 1 || manual.ExpandedMembers[0].Raw != "@all" || manual.ExpandedMembers[0].Origin != RouteMemberOriginLiteral {
		t.Fatalf("Manual 展开成员 = %+v，期望一个字面量 @all", manual.ExpandedMembers)
	}
}

// T-CFG-017：RuntimeConfig 的访问方法返回启动时的值
func TestRuntimeConfig_AccessorsExposeStartupValues(t *testing.T) {
	cfg := validBase()
	cfg.Filters.Exclude = "(HK)"
	cfg.Sources.CustomProxies = []CustomProxy{
		{
			Name: "MY-PROXY",
			URL:  "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp",
		},
	}
	cfg.Sources.FetchOrder = []string{"subscriptions"}
	cfg.Templates = Templates{
		Clash: "https://example.com/base.yaml",
		Surge: "./configs/base_surge.conf",
	}
	cfg.BaseURL = "https://example.com"

	rt, err := Prepare(&cfg)
	if err != nil {
		t.Fatalf("Prepare() 错误 = %v", err)
	}

	src := rt.SourceInput()
	if src.CustomProxies[0].Name != "MY-PROXY" {
		t.Fatalf("自定义代理名称 = %q，期望 MY-PROXY", src.CustomProxies[0].Name)
	}
	if src.CustomProxies[0].Parsed.Params["cipher"] != "aes-256-gcm" {
		t.Fatalf("cipher = %q，期望 aes-256-gcm", src.CustomProxies[0].Parsed.Params["cipher"])
	}
	if src.CustomProxies[0].Parsed.Plugin == nil || src.CustomProxies[0].Parsed.Plugin.Opts["obfs"] != "http" {
		t.Fatalf("plugin 选项 = %+v，期望 obfs=http", src.CustomProxies[0].Parsed.Plugin)
	}
	if src.FetchOrder[0] != "subscriptions" {
		t.Fatalf("拉取顺序 = %v，期望 subscriptions 排在第一位", src.FetchOrder)
	}

	fil := rt.FilterInput()
	if fil.RawExclude != "(HK)" {
		t.Fatalf("过滤器原始 exclude = %q，期望 (HK)", fil.RawExclude)
	}
	if fil.ExcludePattern == nil || !fil.ExcludePattern.MatchString("HK-01") {
		t.Fatalf("exclude 正则仍应匹配 HK-01")
	}

	groups := rt.GroupInput()
	if groups[0].Name != "HK" {
		t.Fatalf("组名 = %q，期望 HK", groups[0].Name)
	}

	routing, rulesets, rules, fallback := rt.RouteInput()
	if routing[0].DeclaredMembers[0].Raw != "HK" {
		t.Fatalf("声明成员 = %q，期望 HK", routing[0].DeclaredMembers[0].Raw)
	}
	if rulesets[0].URLs[0] != "https://example.com/rules.list" {
		t.Fatalf("规则集 URL = %q，期望 https://example.com/rules.list", rulesets[0].URLs[0])
	}
	if rules[0].Raw != "GEOIP,CN,proxy" {
		t.Fatalf("规则原文 = %q，期望 GEOIP,CN,proxy", rules[0].Raw)
	}
	if fallback != "proxy" {
		t.Fatalf("fallback = %q，期望 proxy", fallback)
	}

	if rt.BaseURL() != "https://example.com" {
		t.Fatalf("BaseURL() = %q，期望 https://example.com", rt.BaseURL())
	}
	if rt.Templates().Clash != "https://example.com/base.yaml" {
		t.Fatalf("Templates().Clash = %q，期望 https://example.com/base.yaml", rt.Templates().Clash)
	}
}
