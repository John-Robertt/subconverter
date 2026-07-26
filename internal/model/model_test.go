package model

import "testing"

// T-MDL-001: ProxyKind constants have expected string values
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

// T-MDL-002: PolicyRefKind constants have expected string values
func TestPolicyRefKindValues(t *testing.T) {
	tests := []struct {
		kind PolicyRefKind
		want string
	}{
		{PolicyRefProxy, "proxy"},
		{PolicyRefNodeGroup, "node_group"},
		{PolicyRefRouteGroup, "route_group"},
		{PolicyRefReserved, "reserved"},
	}
	for _, tt := range tests {
		if string(tt.kind) != tt.want {
			t.Errorf("PolicyRefKind = %q, want %q", tt.kind, tt.want)
		}
	}
}

// T-MDL-003: Pipeline struct construction with all fields
func TestPipelineConstruction(t *testing.T) {
	p := Pipeline{
		Proxies: []Proxy{
			{ID: 1, Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: KindSubscription},
		},
		NodeGroups: []NodeGroup{
			{Name: "HK", Strategy: "select", Members: []ProxyID{1}},
		},
		RouteGroups: []RouteGroup{
			{Name: "Proxy", Strategy: "select", Members: []PolicyRef{
				{Kind: PolicyRefNodeGroup, Name: "HK"},
				{Kind: PolicyRefReserved, Name: "DIRECT"},
			}},
		},
		Rulesets:   []Ruleset{{Policy: "Proxy", URLs: []string{"https://example.com/rules.list"}}},
		Rules:      []Rule{{Raw: "GEOIP,CN,DIRECT", Policy: "DIRECT"}},
		Fallback:   "Proxy",
		AllProxies: []ProxyID{1},
	}

	if len(p.Proxies) != 1 || p.Proxies[0].Name != "HK-01" {
		t.Errorf("unexpected Proxies: %+v", p.Proxies)
	}
	if p.Fallback != "Proxy" {
		t.Errorf("Fallback = %q", p.Fallback)
	}
}
