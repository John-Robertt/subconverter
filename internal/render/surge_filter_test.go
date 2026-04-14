package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// buildVLessFilterPipeline mirrors buildSnellFilterPipeline but with VLESS
// nodes in the drop position. Layout:
//
//	HK-01 (ss)    ──┐
//	HK-VL (vless) ──┼─► GRP_HK   (1 survivor after filter)
//	SG-VL (vless) ────► GRP_SG   (→ dropped: empty after filter)
//
//	SVC_Quick   members: [GRP_HK, GRP_SG]          → survives (GRP_SG pruned)
//	SVC_SGOnly  members: [GRP_SG]                  → dropped (cascade)
//	SVC_FINAL   members: [GRP_HK, SVC_SGOnly, DIRECT] → survives
func buildVLessFilterPipeline() *model.Pipeline {
	return &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"}, Kind: model.KindSubscription},
			{Name: "HK-VL", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
			{Name: "SG-VL", Type: "vless", Server: "5.6.7.8", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01", "HK-VL"}},
			{Name: "GRP_SG", Scope: model.ScopeNode, Strategy: "select", Members: []string{"SG-VL"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_Quick", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "GRP_SG"}},
			{Name: "SVC_SGOnly", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SG"}},
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "SVC_SGOnly", "DIRECT"}},
		},
		Rulesets: []model.Ruleset{
			{Policy: "SVC_SGOnly", URLs: []string{"https://example.com/NF.list"}},
			{Policy: "SVC_Quick", URLs: []string{"https://example.com/Global.list"}},
		},
		Rules: []model.Rule{
			{Raw: "GEOIP,SG,SVC_SGOnly", Policy: "SVC_SGOnly"},
			{Raw: "GEOIP,CN,SVC_Quick", Policy: "SVC_Quick"},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-01", "HK-VL", "SG-VL"},
	}
}

// T-SURGE-VLESS-001: VLESS proxies are filtered out; all-VLESS node groups
// cascade out.
func TestSurge_Filter_DropsVLessProxiesAndEmptyGroups(t *testing.T) {
	p := buildVLessFilterPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "HK-VL") || strings.Contains(out, "SG-VL") {
		t.Errorf("vless proxy name leaked into Surge output:\n%s", out)
	}
	if strings.Contains(out, "= vless") {
		t.Errorf("vless type leaked into Surge output:\n%s", out)
	}
	if strings.Contains(out, "GRP_SG") {
		t.Errorf("empty node group GRP_SG should be dropped:\n%s", out)
	}
	if !strings.Contains(out, "GRP_HK") {
		t.Errorf("partially-filtered group GRP_HK should survive:\n%s", out)
	}
	if !strings.Contains(out, "HK-01") {
		t.Errorf("non-vless proxy HK-01 should still render:\n%s", out)
	}
}

// T-SURGE-VLESS-002: Route group cascades when its only members are dropped.
func TestSurge_Filter_CascadesEmptyRouteGroup(t *testing.T) {
	p := buildVLessFilterPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "SVC_SGOnly") {
		t.Errorf("SVC_SGOnly should cascade-drop:\n%s", out)
	}
	if !strings.Contains(out, "SVC_Quick") {
		t.Errorf("SVC_Quick should survive:\n%s", out)
	}
	if !strings.Contains(out, "SVC_FINAL") {
		t.Errorf("SVC_FINAL should survive:\n%s", out)
	}
}

// T-SURGE-VLESS-003: Rulesets pointing at a dropped group are removed.
func TestSurge_Filter_DropsRulesetsForDroppedGroups(t *testing.T) {
	p := buildVLessFilterPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "NF.list") {
		t.Errorf("ruleset for dropped group SVC_SGOnly should be removed:\n%s", out)
	}
	if !strings.Contains(out, "Global.list") {
		t.Errorf("ruleset for surviving group SVC_Quick should render:\n%s", out)
	}
}

// T-SURGE-VLESS-004: Inline rules for dropped groups are removed.
func TestSurge_Filter_DropsInlineRulesForDroppedGroups(t *testing.T) {
	p := buildVLessFilterPipeline()
	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "GEOIP,SG,SVC_SGOnly") {
		t.Errorf("inline rule for dropped group should be removed:\n%s", out)
	}
	if !strings.Contains(out, "GEOIP,CN,SVC_Quick") {
		t.Errorf("inline rule for surviving group should render:\n%s", out)
	}
}

// T-SURGE-VLESS-005: Chained proxy cascades when its VLESS upstream is dropped.
func TestSurge_Filter_DropsChainedOnDroppedUpstream(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"}, Kind: model.KindSubscription},
			{Name: "HK-VL", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
			{Name: "HK-VL→CHAIN", Type: "socks5", Server: "127.0.0.1", Port: 1080, Kind: model.KindChained, Dialer: "HK-VL"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
			{Name: "GRP_CHAIN", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-VL→CHAIN"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "GRP_CHAIN", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-01", "HK-VL"},
	}

	got, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "HK-VL→CHAIN") {
		t.Errorf("chained proxy with dropped VLESS upstream should be dropped:\n%s", out)
	}
	// GRP_CHAIN had only the dropped chained node → cascade drop.
	if strings.Contains(out, "GRP_CHAIN") {
		t.Errorf("GRP_CHAIN should cascade-drop after its only member was removed:\n%s", out)
	}
	if !strings.Contains(out, "HK-01") {
		t.Errorf("HK-01 (ss) must still render:\n%s", out)
	}
}

// T-SURGE-VLESS-006: Fallback cleared by VLESS cascade returns
// CodeRenderSurgeFallbackEmpty with the root-cause cascade path.
func TestSurge_Filter_FallbackCleared(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-VL", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-VL"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-VL"},
	}

	_, err := Surge(p, "", nil)
	if err == nil {
		t.Fatal("expected Surge fallback-empty error")
	}

	var re *errtype.RenderError
	if !errors.As(err, &re) {
		t.Fatalf("err type = %T, want *RenderError", err)
	}
	if re.Code != errtype.CodeRenderSurgeFallbackEmpty {
		t.Errorf("Code = %q, want %q", re.Code, errtype.CodeRenderSurgeFallbackEmpty)
	}
	if re.Format != "surge" {
		t.Errorf("Format = %q, want surge", re.Format)
	}
	for _, want := range []string{"SVC_FINAL", "GRP_HK", "HK-VL(vless)", "被 vless 过滤级联清空"} {
		if !strings.Contains(re.Message, want) {
			t.Errorf("message missing %q, got: %s", want, re.Message)
		}
	}
}

// T-SURGE-VLESS-007: Pipelines without any VLESS node pass through unchanged
// (pointer identity).
func TestSurge_Filter_NoOpWhenNoVless(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"}, Kind: model.KindSubscription},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-01"},
	}

	filtered, err := filterForSurge(p)
	if err != nil {
		t.Fatalf("filterForSurge error: %v", err)
	}
	if filtered != p {
		t.Errorf("expected pointer identity passthrough when no VLESS present")
	}
}

// T-SURGE-VLESS-008: Shared subgraph of chained proxies does not misreport
// a cycle (sibling of TestClash_Filter_SharedSubgraphDoesNotReportCycle).
// Two chained proxies share the same VLESS upstream; dropping the upstream
// must drop both children without triggering the cycle guard.
func TestSurge_Filter_SharedSubgraphNoCycleMisreport(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "VL-SRC", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp"}, Kind: model.KindVLess},
			{Name: "VL-SRC→A", Type: "socks5", Server: "127.0.0.1", Port: 1080, Kind: model.KindChained, Dialer: "VL-SRC"},
			{Name: "VL-SRC→B", Type: "socks5", Server: "127.0.0.1", Port: 1081, Kind: model.KindChained, Dialer: "VL-SRC"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_CHAIN", Scope: model.ScopeNode, Strategy: "select", Members: []string{"VL-SRC→A", "VL-SRC→B"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_CHAIN"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"VL-SRC"},
	}

	_, err := Surge(p, "", nil)
	if err == nil {
		t.Fatal("expected fallback-empty error")
	}
	if strings.Contains(err.Error(), "cycle") {
		t.Errorf("shared upstream should not trigger cycle report, got: %s", err.Error())
	}
}

// T-RENDER-VLESS-PARITY: Same pipeline rendered to both formats shows VLESS
// in Clash and omits it in Surge. Mirrors TestRender_SnellVisibilityDiffersBetweenFormats.
func TestRender_VLessVisibilityDiffersBetweenFormats(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pw"}, Kind: model.KindSubscription},
			{Name: "HK-VL", Type: "vless", Server: "1.2.3.4", Port: 443, Params: map[string]string{"uuid": "11111111-2222-3333-4444-555555555555", "security": "tls", "network": "tcp", "servername": "www.example.com"}, Kind: model.KindVLess},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01", "HK-VL"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-01", "HK-VL"},
	}

	clashOut, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash: %v", err)
	}
	if !strings.Contains(string(clashOut), "HK-VL") {
		t.Error("VLESS node should appear in Clash output")
	}
	if !strings.Contains(string(clashOut), "type: vless") {
		t.Error("VLESS type should appear in Clash output")
	}

	surgeOut, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge: %v", err)
	}
	if strings.Contains(string(surgeOut), "HK-VL") {
		t.Error("VLESS node should NOT appear in Surge output")
	}
	if strings.Contains(string(surgeOut), "= vless") {
		t.Error("VLESS type should NOT appear in Surge output")
	}
	// Surviving SS node must still render in both.
	if !strings.Contains(string(surgeOut), "HK-01") {
		t.Error("SS node must still render in Surge")
	}
}
