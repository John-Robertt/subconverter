package config

import "testing"

// T-CFG-016: prepare preserves route member declared and expanded origins
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
		t.Fatalf("Prepare() error = %v", err)
	}

	routing, _, _, fallback := rt.RouteInput()
	if fallback != "Quick" {
		t.Fatalf("fallback = %q, want Quick", fallback)
	}
	if len(routing) != 2 {
		t.Fatalf("routing len = %d, want 2", len(routing))
	}

	quick := routing[0]
	if len(quick.DeclaredMembers) != 1 || quick.DeclaredMembers[0].Raw != "@auto" || quick.DeclaredMembers[0].Origin != RouteMemberOriginLiteral {
		t.Fatalf("Quick declared = %+v, want one literal @auto", quick.DeclaredMembers)
	}
	wantExpanded := []PreparedRouteMember{
		{Raw: "HK", Origin: RouteMemberOriginAutoExpanded},
		{Raw: "SG", Origin: RouteMemberOriginAutoExpanded},
		{Raw: "Manual", Origin: RouteMemberOriginAutoExpanded},
		{Raw: "DIRECT", Origin: RouteMemberOriginAutoExpanded},
	}
	if len(quick.ExpandedMembers) != len(wantExpanded) {
		t.Fatalf("Quick expanded len = %d, want %d", len(quick.ExpandedMembers), len(wantExpanded))
	}
	for i, want := range wantExpanded {
		if quick.ExpandedMembers[i] != want {
			t.Fatalf("Quick expanded[%d] = %+v, want %+v", i, quick.ExpandedMembers[i], want)
		}
	}

	manual := routing[1]
	if len(manual.DeclaredMembers) != 1 || manual.DeclaredMembers[0].Raw != "@all" || manual.DeclaredMembers[0].Origin != RouteMemberOriginLiteral {
		t.Fatalf("Manual declared = %+v, want one literal @all", manual.DeclaredMembers)
	}
	if len(manual.ExpandedMembers) != 1 || manual.ExpandedMembers[0].Raw != "@all" || manual.ExpandedMembers[0].Origin != RouteMemberOriginLiteral {
		t.Fatalf("Manual expanded = %+v, want one literal @all", manual.ExpandedMembers)
	}
}

// T-CFG-017: runtime config accessors expose startup values
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
		t.Fatalf("Prepare() error = %v", err)
	}

	src := rt.SourceInput()
	if src.CustomProxies[0].Name != "MY-PROXY" {
		t.Fatalf("custom proxy name = %q, want MY-PROXY", src.CustomProxies[0].Name)
	}
	if src.CustomProxies[0].Parsed.Params["cipher"] != "aes-256-gcm" {
		t.Fatalf("cipher = %q, want aes-256-gcm", src.CustomProxies[0].Parsed.Params["cipher"])
	}
	if src.CustomProxies[0].Parsed.Plugin == nil || src.CustomProxies[0].Parsed.Plugin.Opts["obfs"] != "http" {
		t.Fatalf("plugin opts = %+v, want obfs=http", src.CustomProxies[0].Parsed.Plugin)
	}
	if src.FetchOrder[0] != "subscriptions" {
		t.Fatalf("fetch order = %v, want subscriptions first", src.FetchOrder)
	}

	fil := rt.FilterInput()
	if fil.RawExclude != "(HK)" {
		t.Fatalf("filter raw exclude = %q, want (HK)", fil.RawExclude)
	}
	if fil.ExcludePattern == nil || !fil.ExcludePattern.MatchString("HK-01") {
		t.Fatalf("exclude pattern should still match HK-01")
	}

	groups := rt.GroupInput()
	if groups[0].Name != "HK" {
		t.Fatalf("group name = %q, want HK", groups[0].Name)
	}

	routing, rulesets, rules, fallback := rt.RouteInput()
	if routing[0].DeclaredMembers[0].Raw != "HK" {
		t.Fatalf("declared member = %q, want HK", routing[0].DeclaredMembers[0].Raw)
	}
	if rulesets[0].URLs[0] != "https://example.com/rules.list" {
		t.Fatalf("ruleset url = %q, want https://example.com/rules.list", rulesets[0].URLs[0])
	}
	if rules[0].Raw != "GEOIP,CN,proxy" {
		t.Fatalf("rule raw = %q, want GEOIP,CN,proxy", rules[0].Raw)
	}
	if fallback != "proxy" {
		t.Fatalf("fallback = %q, want proxy", fallback)
	}

	if rt.BaseURL() != "https://example.com" {
		t.Fatalf("BaseURL() = %q, want https://example.com", rt.BaseURL())
	}
	if rt.Templates().Clash != "https://example.com/base.yaml" {
		t.Fatalf("Templates().Clash = %q, want https://example.com/base.yaml", rt.Templates().Clash)
	}
	if kind, ok := rt.StaticNamespace().Kind("HK"); !ok || kind != "节点组" {
		t.Fatalf("StaticNamespace().Kind(HK) = (%q, %v), want (节点组, true)", kind, ok)
	}
}
