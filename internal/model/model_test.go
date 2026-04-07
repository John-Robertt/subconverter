package model

import "testing"

func TestProxyKindValues(t *testing.T) {
	tests := []struct {
		kind ProxyKind
		want string
	}{
		{KindSubscription, "subscription"},
		{KindCustom, "custom"},
		{KindChained, "chained"},
	}
	for _, tt := range tests {
		if string(tt.kind) != tt.want {
			t.Errorf("ProxyKind = %q, want %q", tt.kind, tt.want)
		}
	}
}

func TestGroupScopeValues(t *testing.T) {
	tests := []struct {
		scope GroupScope
		want  string
	}{
		{ScopeNode, "node"},
		{ScopeRoute, "route"},
	}
	for _, tt := range tests {
		if string(tt.scope) != tt.want {
			t.Errorf("GroupScope = %q, want %q", tt.scope, tt.want)
		}
	}
}

func TestPipelineConstruction(t *testing.T) {
	p := Pipeline{
		Proxies: []Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: KindSubscription},
		},
		NodeGroups: []ProxyGroup{
			{Name: "HK", Scope: ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
		},
		RouteGroups: []ProxyGroup{
			{Name: "Proxy", Scope: ScopeRoute, Strategy: "select", Members: []string{"HK", "DIRECT"}},
		},
		Rulesets:   []Ruleset{{Policy: "Proxy", URLs: []string{"https://example.com/rules.list"}}},
		Rules:      []Rule{{Raw: "GEOIP,CN,DIRECT", Policy: "DIRECT"}},
		Fallback:   "Proxy",
		AllProxies: []string{"HK-01"},
	}

	if len(p.Proxies) != 1 || p.Proxies[0].Name != "HK-01" {
		t.Errorf("unexpected Proxies: %+v", p.Proxies)
	}
	if p.Fallback != "Proxy" {
		t.Errorf("Fallback = %q", p.Fallback)
	}
}
