package pipeline

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// --- helpers ---

func validGroupResult() *GroupResult {
	return &GroupResult{
		Proxies: []model.Proxy{
			{Name: "HK-01", Kind: model.KindSubscription},
			{Name: "SG-01", Kind: model.KindSubscription},
			{Name: "MY-PROXY", Kind: model.KindCustom},
			{Name: "HK-01→MY-PROXY", Kind: model.KindChained, Dialer: "HK-01"},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "🇭🇰 HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
			{Name: "🔗 MY-PROXY", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01→MY-PROXY"}},
		},
		AllProxies: []string{"HK-01", "SG-01", "MY-PROXY"},
	}
}

func validRouteResult() *RouteResult {
	return &RouteResult{
		RouteGroups: []model.ProxyGroup{
			{Name: "Quick", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"🇭🇰 HK", "🔗 MY-PROXY", "DIRECT"}},
			{Name: "Final", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"Quick", "DIRECT"}},
		},
		PreparedRouteGroups: []config.PreparedRouteGroup{
			{
				Name: "Quick",
				DeclaredMembers: []config.PreparedRouteMember{
					{Raw: "🇭🇰 HK", Origin: config.RouteMemberOriginLiteral},
					{Raw: "🔗 MY-PROXY", Origin: config.RouteMemberOriginLiteral},
					{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
				},
				ExpandedMembers: []config.PreparedRouteMember{
					{Raw: "🇭🇰 HK", Origin: config.RouteMemberOriginLiteral},
					{Raw: "🔗 MY-PROXY", Origin: config.RouteMemberOriginLiteral},
					{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
				},
			},
			{
				Name: "Final",
				DeclaredMembers: []config.PreparedRouteMember{
					{Raw: "Quick", Origin: config.RouteMemberOriginLiteral},
					{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
				},
				ExpandedMembers: []config.PreparedRouteMember{
					{Raw: "Quick", Origin: config.RouteMemberOriginLiteral},
					{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
				},
			},
		},
		ResolvedRouteGroups: []ResolvedRouteGroup{
			{
				Name: "Quick",
				Members: []config.PreparedRouteMember{
					{Raw: "🇭🇰 HK", Origin: config.RouteMemberOriginLiteral},
					{Raw: "🔗 MY-PROXY", Origin: config.RouteMemberOriginLiteral},
					{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
				},
			},
			{
				Name: "Final",
				Members: []config.PreparedRouteMember{
					{Raw: "Quick", Origin: config.RouteMemberOriginLiteral},
					{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
				},
			},
		},
		Rulesets: []model.Ruleset{
			{Policy: "Quick", URLs: []string{"https://example.com/rule.list"}},
		},
		Rules: []model.Rule{
			{Raw: "GEOIP,CN,Final", Policy: "Final"},
		},
		Fallback: "Final",
	}
}

func containsError(err error, substr string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), substr)
}

func unwrapAll(err error) []error {
	if err == nil {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}

func countErrorContains(err error, substr string) int {
	count := 0
	for _, e := range unwrapAll(err) {
		if strings.Contains(e.Error(), substr) {
			count++
		}
	}
	return count
}

// --- tests ---

func TestValidateGraph_ValidGraph(t *testing.T) {
	p, err := ValidateGraph(validGroupResult(), validRouteResult())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("pipeline is nil")
	}
	if len(p.Proxies) != 4 {
		t.Errorf("Proxies count = %d, want 4", len(p.Proxies))
	}
	if p.Fallback != "Final" {
		t.Errorf("Fallback = %q, want %q", p.Fallback, "Final")
	}
}

func TestValidateGraph_RouteGroupMemberNotFound(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.RouteGroups[0].Members = append(rr.RouteGroups[0].Members, "Nonexistent")
	rr.PreparedRouteGroups[0].DeclaredMembers = append(rr.PreparedRouteGroups[0].DeclaredMembers, config.PreparedRouteMember{
		Raw:    "Nonexistent",
		Origin: config.RouteMemberOriginLiteral,
	})
	rr.PreparedRouteGroups[0].ExpandedMembers = append(rr.PreparedRouteGroups[0].ExpandedMembers, config.PreparedRouteMember{
		Raw:    "Nonexistent",
		Origin: config.RouteMemberOriginLiteral,
	})
	rr.ResolvedRouteGroups[0].Members = append(rr.ResolvedRouteGroups[0].Members, config.PreparedRouteMember{
		Raw:    "Nonexistent",
		Origin: config.RouteMemberOriginLiteral,
	})

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `成员 "Nonexistent" 不存在`) {
		t.Errorf("error = %v, want mention of Nonexistent", err)
	}
	if got := countErrorContains(err, `成员 "Nonexistent" 不存在`); got != 1 {
		t.Errorf("expected exactly 1 missing-member error, got %d: %v", got, err)
	}
}

func TestValidateGraph_CircularReference(t *testing.T) {
	gr := validGroupResult()
	rr := &RouteResult{
		RouteGroups: []model.ProxyGroup{
			{Name: "A", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"B", "DIRECT"}},
			{Name: "B", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"A", "DIRECT"}},
		},
		Fallback: "A",
	}

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, "循环引用") {
		t.Errorf("error = %v, want circular reference", err)
	}
}

func TestValidateGraph_SelfReference(t *testing.T) {
	gr := validGroupResult()
	rr := &RouteResult{
		RouteGroups: []model.ProxyGroup{
			{Name: "A", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"A", "DIRECT"}},
		},
		Fallback: "A",
	}

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, "循环引用") {
		t.Errorf("error = %v, want circular reference", err)
	}
}

func TestValidateGraph_EmptyChainedGroup(t *testing.T) {
	gr := validGroupResult()
	gr.NodeGroups[1].Members = nil // 🔗 MY-PROXY becomes empty

	_, err := ValidateGraph(gr, validRouteResult())
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `节点组 "🔗 MY-PROXY" 没有成员`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_EmptyRegionGroup(t *testing.T) {
	gr := validGroupResult()
	gr.NodeGroups[0].Members = nil // 🇭🇰 HK becomes empty

	_, err := ValidateGraph(gr, validRouteResult())
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `节点组 "🇭🇰 HK" 没有成员`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_NameCollision(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	// Make a route group with same name as a node group
	rr.RouteGroups = append(rr.RouteGroups, model.ProxyGroup{
		Name: "🇭🇰 HK", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"DIRECT"},
	})

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `名称 "🇭🇰 HK" 同时被`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_ProxyAndNodeGroupNameCollision(t *testing.T) {
	gr := validGroupResult()
	gr.NodeGroups = append(gr.NodeGroups, model.ProxyGroup{
		Name: "HK-01", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"},
	})

	_, err := ValidateGraph(gr, validRouteResult())
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `名称 "HK-01" 同时被 代理 和 节点组 使用`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_DuplicateRouteGroupNames(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.RouteGroups = append(rr.RouteGroups, model.ProxyGroup{
		Name: "Quick", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"DIRECT"},
	})

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `服务组 "Quick" 重复声明`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_RouteGroupExpandedProxyMembersAllowedViaAtAll(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.RouteGroups[0].Members = []string{"HK-01", "SG-01", "MY-PROXY", "DIRECT"}
	rr.PreparedRouteGroups[0].DeclaredMembers = []config.PreparedRouteMember{
		{Raw: "@all", Origin: config.RouteMemberOriginLiteral},
		{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
	}
	rr.PreparedRouteGroups[0].ExpandedMembers = []config.PreparedRouteMember{
		{Raw: "@all", Origin: config.RouteMemberOriginLiteral},
		{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
	}
	rr.ResolvedRouteGroups[0].Members = []config.PreparedRouteMember{
		{Raw: "HK-01", Origin: config.RouteMemberOriginAllExpanded},
		{Raw: "SG-01", Origin: config.RouteMemberOriginAllExpanded},
		{Raw: "MY-PROXY", Origin: config.RouteMemberOriginAllExpanded},
		{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
	}

	_, err := ValidateGraph(gr, rr)
	if err != nil {
		t.Fatalf("@all-expanded proxies should remain valid after raw-member validation: %v", err)
	}
}

func TestValidateGraph_AllProxiesContainsChainedRejected(t *testing.T) {
	gr := validGroupResult()
	gr.AllProxies = append(gr.AllProxies, "HK-01→MY-PROXY")

	_, err := ValidateGraph(gr, validRouteResult())
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `@all 包含了链式节点 "HK-01→MY-PROXY"`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_DirectRejectAreValid(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.RouteGroups[0].Members = []string{"DIRECT", "REJECT", "🇭🇰 HK"}

	_, err := ValidateGraph(gr, rr)
	if err != nil {
		t.Fatalf("DIRECT/REJECT should be valid members: %v", err)
	}
}

func TestValidateGraph_MultipleErrors(t *testing.T) {
	gr := validGroupResult()
	gr.NodeGroups[1].Members = nil // empty chained group

	rr := validRouteResult()
	rr.RouteGroups[0].Members = append(rr.RouteGroups[0].Members, "AlsoMissing") // bad member
	rr.PreparedRouteGroups[0].DeclaredMembers = append(rr.PreparedRouteGroups[0].DeclaredMembers, config.PreparedRouteMember{
		Raw:    "AlsoMissing",
		Origin: config.RouteMemberOriginLiteral,
	})
	rr.PreparedRouteGroups[0].ExpandedMembers = append(rr.PreparedRouteGroups[0].ExpandedMembers, config.PreparedRouteMember{
		Raw:    "AlsoMissing",
		Origin: config.RouteMemberOriginLiteral,
	})
	rr.ResolvedRouteGroups[0].Members = append(rr.ResolvedRouteGroups[0].Members, config.PreparedRouteMember{
		Raw:    "AlsoMissing",
		Origin: config.RouteMemberOriginLiteral,
	})

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected multiple errors")
	}
	errs := unwrapAll(err)
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors, got %d: %v", len(errs), err)
	}
	// Verify all errors are BuildError with phase "validate".
	for _, e := range errs {
		var be *errtype.BuildError
		if !errors.As(e, &be) {
			t.Errorf("expected *errtype.BuildError, got %T", e)
		} else if be.Phase != "validate" {
			t.Errorf("Phase = %q, want %q", be.Phase, "validate")
		}
	}
}

// T-VAL-SNELL-001: ValidateGraph happy path with KindSnell proxies. Guards
// against KindSnell being excluded from namespace registration or @all
// validation, which would cause Snell references to look like dangling
// identifiers.
func TestValidateGraph_WithSnellProxy(t *testing.T) {
	gr := &GroupResult{
		Proxies: []model.Proxy{
			{Name: "HK-01", Kind: model.KindSubscription},
			{Name: "HK-Snell", Kind: model.KindSnell},
			{Name: "SG-Snell", Kind: model.KindSnell},
		},
		NodeGroups: []model.ProxyGroup{
			{Name: "HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01", "HK-Snell"}},
			{Name: "SG", Scope: model.ScopeNode, Strategy: "select", Members: []string{"SG-Snell"}},
		},
		AllProxies: []string{"HK-01", "HK-Snell", "SG-Snell"},
	}
	rr := &RouteResult{
		RouteGroups: []model.ProxyGroup{
			{Name: "Quick", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"HK", "SG", "DIRECT"}},
			{Name: "Final", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"Quick", "DIRECT"}},
		},
		Fallback: "Final",
	}

	p, err := ValidateGraph(gr, rr)
	if err != nil {
		t.Fatalf("ValidateGraph with Snell proxies failed: %v", err)
	}
	if len(p.Proxies) != 3 {
		t.Errorf("Proxies count = %d, want 3", len(p.Proxies))
	}
	// Snell nodes must be present in the returned Pipeline.
	names := map[string]bool{}
	for _, px := range p.Proxies {
		names[px.Name] = true
	}
	if !names["HK-Snell"] || !names["SG-Snell"] {
		t.Errorf("Snell proxies missing from validated Pipeline: %v", names)
	}
}

// Without the 🔗 prefix, a custom proxy name (used as a chain group name when
// relay_through is set) shares the namespace with region groups and route
// groups. If a user picks a cp.Name already used by a region group,
// ValidateGraph must surface the collision — users relied on 🔗 to keep
// them apart before, so this guards the fallback path.
func TestValidateGraph_ChainGroupNameCollidesWithRegionGroup(t *testing.T) {
	gr := &GroupResult{
		Proxies: []model.Proxy{
			{Name: "HK-01", Kind: model.KindSubscription},
			{Name: "HK-01→HK", Kind: model.KindChained, Dialer: "HK-01"},
		},
		NodeGroups: []model.ProxyGroup{
			// Region group "HK" declared by the user.
			{Name: "HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01"}},
			// Chain group "HK" auto-generated because some cp.Name is also "HK".
			{Name: "HK", Scope: model.ScopeNode, Strategy: "select", Members: []string{"HK-01→HK"}},
		},
		AllProxies: []string{"HK-01"},
	}
	rr := &RouteResult{
		RouteGroups: []model.ProxyGroup{
			{Name: "Final", Scope: model.ScopeRoute, Strategy: "select", Members: []string{"HK", "DIRECT"}},
		},
		Fallback: "Final",
	}

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected collision error")
	}
	if !containsError(err, `节点组 "HK" 重复声明`) {
		t.Errorf("error = %v, want duplicate-declaration for HK", err)
	}
}
