package render

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// buildSnellFilterPipeline builds a small Pipeline exercising the filter cascade.
//
// Naming convention (chosen so substring assertions below are unambiguous):
//   - Proxy names:   HK-01, HK-Snell, SG-Snell  (contain "HK" / "SG")
//   - Node groups:   GRP_HK, GRP_SG             (prefixed, never a substring of a proxy)
//   - Route groups:  SVC_Quick, SVC_SGOnly, SVC_FINAL
//   - Ruleset URLs:  marker files (NF.list, Global.list) let us assert presence.
//
// Layout:
//
//	HK-01 (ss)       ──┐
//	HK-Snell (snell) ──┼─► GRP_HK   (1 survivor after filter)
//	SG-Snell (snell) ────► GRP_SG   (→ dropped: empty after filter)
//
//	SVC_Quick   members: [GRP_HK, GRP_SG]          → survives (GRP_SG pruned)
//	SVC_SGOnly  members: [GRP_SG]                  → dropped (cascade)
//	SVC_FINAL   members: [GRP_HK, SVC_SGOnly, DIRECT] → survives (SVC_SGOnly pruned)
func buildSnellFilterPipeline() *model.Pipeline {
	return &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: model.KindSubscription},
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{Name: "SG-Snell", Type: "snell", Server: "5.6.7.8", Port: 8989, Params: map[string]string{"psk": "y", "version": "4"}, Kind: model.KindSnell},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01", "HK-Snell"}},
			{Name: "GRP_SG", Scope: model.ScopeNode, Strategy: "select", Members: []string{"SG-Snell"}},
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
		AllProxies: []string{"HK-01", "HK-Snell", "SG-Snell"},
	}
}

// T-CLASH-SNELL-001: snell proxies disappear; all-snell node group disappears.
func TestClash_Filter_DropsSnellProxiesAndEmptyGroups(t *testing.T) {
	p := buildSnellFilterPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "HK-Snell") || strings.Contains(out, "SG-Snell") {
		t.Errorf("snell proxy name leaked into Clash output:\n%s", out)
	}
	if strings.Contains(out, "type: snell") {
		t.Errorf("snell type leaked into Clash output:\n%s", out)
	}
	// GRP_SG had only a snell member → must be gone.
	if strings.Contains(out, "GRP_SG") {
		t.Errorf("empty node group GRP_SG should be dropped from output:\n%s", out)
	}
	// GRP_HK survives (HK-01 is ss, not snell).
	if !strings.Contains(out, "GRP_HK") {
		t.Errorf("partially-filtered group GRP_HK should survive:\n%s", out)
	}
	if !strings.Contains(out, "HK-01") {
		t.Errorf("non-snell proxy HK-01 should still render:\n%s", out)
	}
}

// T-CLASH-SNELL-002: Route group cascades when its only members are dropped.
func TestClash_Filter_CascadesEmptyRouteGroup(t *testing.T) {
	p := buildSnellFilterPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "SVC_SGOnly") {
		t.Errorf("SVC_SGOnly should cascade-drop (only referenced GRP_SG):\n%s", out)
	}
	if !strings.Contains(out, "SVC_Quick") {
		t.Errorf("SVC_Quick should survive (had GRP_HK besides GRP_SG):\n%s", out)
	}
	if !strings.Contains(out, "SVC_FINAL") {
		t.Errorf("SVC_FINAL should survive (had GRP_HK + DIRECT):\n%s", out)
	}
}

// T-CLASH-SNELL-003: Rulesets pointing at a dropped group are removed.
func TestClash_Filter_DropsRulesetsForDroppedGroups(t *testing.T) {
	p := buildSnellFilterPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "NF.list") {
		t.Errorf("ruleset for dropped group SVC_SGOnly should be removed:\n%s", out)
	}
	if !strings.Contains(out, "Global.list") {
		t.Errorf("ruleset for surviving group SVC_Quick should render:\n%s", out)
	}
}

// T-CLASH-SNELL-004: Inline rules referencing dropped groups are removed.
func TestClash_Filter_DropsInlineRulesForDroppedGroups(t *testing.T) {
	p := buildSnellFilterPipeline()
	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "GEOIP,SG,SVC_SGOnly") {
		t.Errorf("inline rule for dropped group should be removed:\n%s", out)
	}
	if !strings.Contains(out, "GEOIP,CN,SVC_Quick") {
		t.Errorf("inline rule for surviving group should render:\n%s", out)
	}
}

// T-CLASH-SNELL-005: Chained proxies whose upstream is snell are also dropped.
func TestClash_Filter_DropsChainedOnDroppedUpstream(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: model.KindSubscription},
			{Name: "HK-Snell→MY-PROXY", Type: "socks5", Server: "1.1.1.1", Port: 1080, Kind: model.KindChained, Dialer: "HK-Snell"},
			{Name: "HK-01→MY-PROXY", Type: "socks5", Server: "1.1.1.1", Port: 1080, Kind: model.KindChained, Dialer: "HK-01"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
			{Name: "GRP_CHAIN", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell→MY-PROXY", "HK-01→MY-PROXY"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "GRP_CHAIN", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-Snell", "HK-01"},
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "HK-Snell→MY-PROXY") {
		t.Errorf("chained proxy with snell upstream should be dropped:\n%s", out)
	}
	if !strings.Contains(out, "HK-01→MY-PROXY") {
		t.Errorf("chained proxy with ss upstream should survive:\n%s", out)
	}
	// GRP_CHAIN had 2 members; 1 is pruned → group survives with 1 member.
	if !strings.Contains(out, "GRP_CHAIN") {
		t.Errorf("chain group with surviving member should remain:\n%s", out)
	}
}

// T-CLASH-SNELL-006: Fallback cleared by cascade → RenderError.
func TestClash_Filter_FallbackCleared(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_SG", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SG"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-Snell"},
	}

	_, err := Clash(p, nil)
	if err == nil {
		t.Fatal("expected RenderError when fallback group is cleared")
	}
	var re *errtype.RenderError
	if !errors.As(err, &re) {
		t.Fatalf("err type = %T, want *errtype.RenderError", err)
	}
	if re.Code != errtype.CodeRenderClashFallbackEmpty {
		t.Errorf("Code = %q, want %q", re.Code, errtype.CodeRenderClashFallbackEmpty)
	}
	// Error message should include the cascade path so users can find the
	// root cause (the Snell proxy that triggered the clearance).
	msg := re.Error()
	for _, keyword := range []string{"SVC_FINAL", "GRP_SG", "HK-Snell(snell)"} {
		if !strings.Contains(msg, keyword) {
			t.Errorf("error message missing cascade keyword %q:\n%s", keyword, msg)
		}
	}
}

// T-CLASH-SNELL-009: Chained drops should be labeled as chained and point to
// the upstream Snell cause, rather than being mislabeled as snell themselves.
func TestClash_Filter_FallbackPathLabelsChainedProxy(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{Name: "HK-Snell→MY-PROXY", Type: "socks5", Server: "10.0.0.1", Port: 1080, Kind: model.KindChained, Dialer: "HK-Snell"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_CHAIN", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell→MY-PROXY"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_CHAIN"}},
		},
		Fallback: "SVC_FINAL",
	}

	_, err := Clash(p, nil)
	if err == nil {
		t.Fatal("expected RenderError when fallback group is cleared")
	}
	var re *errtype.RenderError
	if !errors.As(err, &re) {
		t.Fatalf("err type = %T, want *errtype.RenderError", err)
	}

	msg := re.Error()
	if !strings.Contains(msg, "HK-Snell→MY-PROXY(chained)") {
		t.Errorf("cascade path should label chained proxy explicitly:\n%s", msg)
	}
	if !strings.Contains(msg, "HK-Snell(snell)") {
		t.Errorf("cascade path should include upstream snell root:\n%s", msg)
	}
}

// T-CLASH-SNELL-010: Shared dropped subgraphs should expand normally on each
// branch, not be misreported as cycles.
func TestClash_Filter_SharedSubgraphDoesNotReportCycle(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_SHARED", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_A", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SHARED"}},
			{Name: "SVC_B", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_SHARED"}},
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"SVC_A", "SVC_B"}},
		},
		Fallback: "SVC_FINAL",
	}

	_, err := Clash(p, nil)
	if err == nil {
		t.Fatal("expected RenderError when fallback group is cleared")
	}
	var re *errtype.RenderError
	if !errors.As(err, &re) {
		t.Fatalf("err type = %T, want *errtype.RenderError", err)
	}
	if strings.Contains(re.Error(), "(cycle)") {
		t.Errorf("shared drop subgraph should not be labeled as a cycle:\n%s", re.Error())
	}
}

// T-RENDER-SNELL-CHAIN: Snell-upstream chained node visibility differs
// between formats: Surge emits the chained line, Clash cascades it out.
func TestRender_SnellUpstreamChainVisibility(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "s"}, Kind: model.KindSubscription},
			{Name: "HK-Snell→MY-PROXY", Type: "socks5", Server: "10.0.0.1", Port: 1080, Params: map[string]string{"username": "u", "password": "p"}, Kind: model.KindChained, Dialer: "HK-Snell"},
			{Name: "HK-01→MY-PROXY", Type: "socks5", Server: "10.0.0.1", Port: 1080, Params: map[string]string{"username": "u", "password": "p"}, Kind: model.KindChained, Dialer: "HK-01"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01", "HK-Snell"}},
			{Name: "GRP_CHAIN", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-Snell→MY-PROXY", "HK-01→MY-PROXY"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "GRP_CHAIN", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-Snell", "HK-01"},
	}

	surgeOut, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	clashOut, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	// Surge: Snell upstream chain line MUST appear with underlying-proxy set.
	if !strings.Contains(string(surgeOut), "HK-Snell→MY-PROXY = socks5, 10.0.0.1, 1080") {
		t.Errorf("Surge missing snell-chained proxy line:\n%s", string(surgeOut))
	}
	if !strings.Contains(string(surgeOut), "underlying-proxy=HK-Snell") {
		t.Errorf("Surge missing underlying-proxy=HK-Snell directive:\n%s", string(surgeOut))
	}
	// Clash: Snell-upstream chained node MUST be absent.
	if strings.Contains(string(clashOut), "HK-Snell→MY-PROXY") {
		t.Errorf("Clash output leaked Snell-upstream chained node:\n%s", string(clashOut))
	}
	// Clash: SS-upstream chained node MUST be present.
	if !strings.Contains(string(clashOut), "HK-01→MY-PROXY") {
		t.Errorf("Clash missing SS-upstream chained node:\n%s", string(clashOut))
	}
}

// T-RENDER-SNELL-PARITY: Same Pipeline rendered with both Clash and Surge:
// Surge includes the snell node; Clash omits it and any group that would be
// empty as a result. Verifies the Surge-only visibility rule end-to-end.
func TestRender_SnellVisibilityDiffersBetweenFormats(t *testing.T) {
	p := buildSnellFilterPipeline()

	surgeOut, err := Surge(p, "", nil)
	if err != nil {
		t.Fatalf("Surge() error: %v", err)
	}
	clashOut, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}

	// Surge: snell proxies MUST appear as proxy lines.
	if !strings.Contains(string(surgeOut), "HK-Snell = snell, 1.2.3.4, 57891, psk=x, version=4") {
		t.Errorf("Surge output missing HK-Snell line:\n%s", string(surgeOut))
	}
	if !strings.Contains(string(surgeOut), "SG-Snell = snell, 5.6.7.8, 8989, psk=y, version=4") {
		t.Errorf("Surge output missing SG-Snell line:\n%s", string(surgeOut))
	}
	// Surge: GRP_SG survives because SG-Snell is there.
	if !strings.Contains(string(surgeOut), "GRP_SG = select") {
		t.Errorf("Surge output missing GRP_SG group:\n%s", string(surgeOut))
	}

	// Clash: snell proxies MUST be absent.
	if strings.Contains(string(clashOut), "HK-Snell") || strings.Contains(string(clashOut), "SG-Snell") {
		t.Errorf("Clash output leaked snell proxy name:\n%s", string(clashOut))
	}
	// Clash: GRP_SG (all-snell group) MUST be absent.
	if strings.Contains(string(clashOut), "GRP_SG") {
		t.Errorf("Clash output still references dropped GRP_SG:\n%s", string(clashOut))
	}
	// Clash: GRP_HK (mixed group) must survive.
	if !strings.Contains(string(clashOut), "GRP_HK") {
		t.Errorf("Clash output missing surviving GRP_HK:\n%s", string(clashOut))
	}
}

// T-CLASH-SNELL-008: RouteGroup whose @all-expanded members are all Snell
// must be cascade-dropped (this is the scenario where a user's routing uses
// "@all" against a node pool that happens to be entirely Snell-backed).
func TestClash_Filter_AllSnellRouteGroupViaAtAll(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-Snell", Type: "snell", Server: "1.2.3.4", Port: 57891, Params: map[string]string{"psk": "x", "version": "4"}, Kind: model.KindSnell},
			{Name: "SG-Snell", Type: "snell", Server: "5.6.7.8", Port: 8989, Params: map[string]string{"psk": "y", "version": "4"}, Kind: model.KindSnell},
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: model.KindSubscription},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
		},
		RouteGroups: []model.ProxyGroup{
			// SVC_ManualAll simulates the expansion of ["@all"] — but only
			// snell nodes are listed (HK-01 excluded to simulate the edge
			// case). Clash should cascade-drop this group entirely.
			{Name: "SVC_ManualAll", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"HK-Snell", "SG-Snell"}},
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK", "SVC_ManualAll", "DIRECT"}},
		},
		Fallback:   "SVC_FINAL",
		AllProxies: []string{"HK-Snell", "SG-Snell", "HK-01"},
	}

	got, err := Clash(p, nil)
	if err != nil {
		t.Fatalf("Clash() error: %v", err)
	}
	out := string(got)
	if strings.Contains(out, "SVC_ManualAll") {
		t.Errorf("RouteGroup with all-snell @all expansion should be dropped:\n%s", out)
	}
	// SVC_FINAL survives because SVC_ManualAll was pruned and GRP_HK + DIRECT remain.
	if !strings.Contains(out, "SVC_FINAL") {
		t.Errorf("SVC_FINAL should survive (still has GRP_HK and DIRECT):\n%s", out)
	}
}

// T-CLASH-SNELL-007: Pipelines with no snell nodes pass through unchanged.
func TestClash_Filter_NoOpWhenNoSnell(t *testing.T) {
	p := &model.Pipeline{
		Proxies: []model.Proxy{
			{Name: "HK-01", Type: "ss", Server: "hk.example.com", Port: 8388, Kind: model.KindSubscription},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "GRP_HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
		},
		RouteGroups: []model.ProxyGroup{
			{Name: "SVC_FINAL", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"GRP_HK"}},
		},
		Fallback: "SVC_FINAL",
	}
	out, err := filterForClash(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != p {
		t.Errorf("expected identical pointer when no filtering is needed")
	}
}
