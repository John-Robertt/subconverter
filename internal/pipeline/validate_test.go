package pipeline

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

func proxyRef(id model.ProxyID) model.PolicyRef {
	return model.PolicyRef{Kind: model.PolicyRefProxy, ProxyID: id}
}

func nodeGroupRef(name string) model.PolicyRef {
	return model.PolicyRef{Kind: model.PolicyRefNodeGroup, Name: name}
}

func routeGroupRef(name string) model.PolicyRef {
	return model.PolicyRef{Kind: model.PolicyRefRouteGroup, Name: name}
}

func reservedRef(name string) model.PolicyRef {
	return model.PolicyRef{Kind: model.PolicyRefReserved, Name: name}
}

func unresolvedRef(name string) model.PolicyRef {
	return model.PolicyRef{Name: name}
}

func resolvedMember(ref model.PolicyRef, raw string, origin config.RouteMemberOrigin) ResolvedRouteMember {
	return ResolvedRouteMember{Ref: ref, Raw: raw, Origin: origin}
}

func validGroupResult() *GroupResult {
	return &GroupResult{
		Proxies: []model.Proxy{
			{ID: 1, Name: "HK-01", Kind: model.KindSubscription},
			{ID: 2, Name: "SG-01", Kind: model.KindSubscription},
			{ID: 3, Name: "MY-PROXY", Kind: model.KindCustom},
			{
				ID:              4,
				Name:            "HK-01→MY-PROXY",
				Kind:            model.KindChained,
				DialerID:        1,
				ChainNameSuffix: "MY-PROXY",
			},
		},
		NodeGroups: []model.NodeGroup{
			{Name: "🇭🇰 HK", Strategy: "select", Members: []model.ProxyID{1}},
			{Name: "🔗 MY-PROXY", Strategy: "select", Members: []model.ProxyID{4}},
		},
		AllProxies: []model.ProxyID{1, 2, 3},
	}
}

func validRouteResult() *RouteResult {
	return &RouteResult{
		RouteGroups: []model.RouteGroup{
			{Name: "Quick", Strategy: "select", Members: []model.PolicyRef{
				nodeGroupRef("🇭🇰 HK"),
				nodeGroupRef("🔗 MY-PROXY"),
				reservedRef("DIRECT"),
			}},
			{Name: "Final", Strategy: "select", Members: []model.PolicyRef{
				routeGroupRef("Quick"),
				reservedRef("DIRECT"),
			}},
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
				Members: []ResolvedRouteMember{
					resolvedMember(nodeGroupRef("🇭🇰 HK"), "🇭🇰 HK", config.RouteMemberOriginLiteral),
					resolvedMember(nodeGroupRef("🔗 MY-PROXY"), "🔗 MY-PROXY", config.RouteMemberOriginLiteral),
					resolvedMember(reservedRef("DIRECT"), "DIRECT", config.RouteMemberOriginLiteral),
				},
			},
			{
				Name: "Final",
				Members: []ResolvedRouteMember{
					resolvedMember(routeGroupRef("Quick"), "Quick", config.RouteMemberOriginLiteral),
					resolvedMember(reservedRef("DIRECT"), "DIRECT", config.RouteMemberOriginLiteral),
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
	return err != nil && strings.Contains(err.Error(), substr)
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
		t.Errorf("Fallback = %q, want Final", p.Fallback)
	}
}

// T-VAL-ID-001: display names may repeat because graph identity is ProxyID.
func TestValidateGraph_DuplicateProxyNamesWithDistinctIDsAllowed(t *testing.T) {
	gr := validGroupResult()
	gr.Proxies[1].Name = gr.Proxies[0].Name

	if _, err := ValidateGraph(gr, validRouteResult()); err != nil {
		t.Fatalf("duplicate display names with distinct IDs should be valid: %v", err)
	}
}

// T-VAL-ID-002: graph identity itself remains unique.
func TestValidateGraph_DuplicateProxyIDsRejected(t *testing.T) {
	gr := validGroupResult()
	gr.Proxies[1].ID = gr.Proxies[0].ID

	_, err := ValidateGraph(gr, validRouteResult())
	if !containsError(err, "代理 ID 1 重复声明") {
		t.Fatalf("error = %v, want duplicate proxy ID", err)
	}
}

func TestValidateGraph_RouteGroupMemberNotFound(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.RouteGroups[0].Members = append(rr.RouteGroups[0].Members, unresolvedRef("Nonexistent"))
	missing := config.PreparedRouteMember{Raw: "Nonexistent", Origin: config.RouteMemberOriginLiteral}
	rr.PreparedRouteGroups[0].DeclaredMembers = append(rr.PreparedRouteGroups[0].DeclaredMembers, missing)
	rr.PreparedRouteGroups[0].ExpandedMembers = append(rr.PreparedRouteGroups[0].ExpandedMembers, missing)
	rr.ResolvedRouteGroups[0].Members = append(
		rr.ResolvedRouteGroups[0].Members,
		resolvedMember(unresolvedRef("Nonexistent"), "Nonexistent", config.RouteMemberOriginLiteral),
	)

	_, err := ValidateGraph(gr, rr)
	if !containsError(err, `成员 "Nonexistent" 不存在`) {
		t.Errorf("error = %v, want mention of Nonexistent", err)
	}
	if got := countErrorContains(err, `成员 "Nonexistent" 不存在`); got != 2 {
		t.Errorf("expected declared and resolved missing-member errors, got %d: %v", got, err)
	}
}

func TestValidateGraph_CircularReference(t *testing.T) {
	rr := &RouteResult{
		RouteGroups: []model.RouteGroup{
			{Name: "A", Strategy: "select", Members: []model.PolicyRef{routeGroupRef("B"), reservedRef("DIRECT")}},
			{Name: "B", Strategy: "select", Members: []model.PolicyRef{routeGroupRef("A"), reservedRef("DIRECT")}},
		},
		Fallback: "A",
	}

	_, err := ValidateGraph(validGroupResult(), rr)
	if !containsError(err, "循环引用") {
		t.Errorf("error = %v, want circular reference", err)
	}
}

func TestValidateGraph_SelfReference(t *testing.T) {
	rr := &RouteResult{
		RouteGroups: []model.RouteGroup{
			{Name: "A", Strategy: "select", Members: []model.PolicyRef{routeGroupRef("A"), reservedRef("DIRECT")}},
		},
		Fallback: "A",
	}

	_, err := ValidateGraph(validGroupResult(), rr)
	if !containsError(err, "循环引用") {
		t.Errorf("error = %v, want circular reference", err)
	}
}

func TestValidateGraph_EmptyChainedGroup(t *testing.T) {
	gr := validGroupResult()
	gr.NodeGroups[1].Members = nil

	_, err := ValidateGraph(gr, validRouteResult())
	if !containsError(err, `节点组 "🔗 MY-PROXY" 没有成员`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_EmptyRegionGroup(t *testing.T) {
	gr := validGroupResult()
	gr.NodeGroups[0].Members = nil

	_, err := ValidateGraph(gr, validRouteResult())
	if !containsError(err, `节点组 "🇭🇰 HK" 没有成员`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_NameCollision(t *testing.T) {
	rr := validRouteResult()
	rr.RouteGroups = append(rr.RouteGroups, model.RouteGroup{
		Name: "🇭🇰 HK", Strategy: "select", Members: []model.PolicyRef{reservedRef("DIRECT")},
	})

	_, err := ValidateGraph(validGroupResult(), rr)
	if !containsError(err, `名称 "🇭🇰 HK" 同时被`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_ProxyAndNodeGroupMayShareDisplayName(t *testing.T) {
	gr := validGroupResult()
	gr.NodeGroups = append(gr.NodeGroups, model.NodeGroup{
		Name: "HK-01", Strategy: "select", Members: []model.ProxyID{1},
	})

	if _, err := ValidateGraph(gr, validRouteResult()); err != nil {
		t.Fatalf("proxy display name should not occupy group namespace: %v", err)
	}
}

func TestValidateGraph_DuplicateRouteGroupNames(t *testing.T) {
	rr := validRouteResult()
	rr.RouteGroups = append(rr.RouteGroups, model.RouteGroup{
		Name: "Quick", Strategy: "select", Members: []model.PolicyRef{reservedRef("DIRECT")},
	})

	_, err := ValidateGraph(validGroupResult(), rr)
	if !containsError(err, `服务组 "Quick" 重复声明`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_RouteGroupExpandedProxyMembersAllowedViaAtAll(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.RouteGroups[0].Members = []model.PolicyRef{
		proxyRef(1), proxyRef(2), proxyRef(3), reservedRef("DIRECT"),
	}
	rr.PreparedRouteGroups[0].DeclaredMembers = []config.PreparedRouteMember{
		{Raw: "@all", Origin: config.RouteMemberOriginLiteral},
		{Raw: "DIRECT", Origin: config.RouteMemberOriginLiteral},
	}
	rr.PreparedRouteGroups[0].ExpandedMembers = append(
		[]config.PreparedRouteMember(nil),
		rr.PreparedRouteGroups[0].DeclaredMembers...,
	)
	rr.ResolvedRouteGroups[0].Members = []ResolvedRouteMember{
		resolvedMember(proxyRef(1), "@all", config.RouteMemberOriginAllExpanded),
		resolvedMember(proxyRef(2), "@all", config.RouteMemberOriginAllExpanded),
		resolvedMember(proxyRef(3), "@all", config.RouteMemberOriginAllExpanded),
		resolvedMember(reservedRef("DIRECT"), "DIRECT", config.RouteMemberOriginLiteral),
	}

	if _, err := ValidateGraph(gr, rr); err != nil {
		t.Fatalf("@all-expanded proxies should remain valid: %v", err)
	}
}

func TestValidateGraph_AllProxiesContainsChainedRejected(t *testing.T) {
	gr := validGroupResult()
	gr.AllProxies = append(gr.AllProxies, 4)

	_, err := ValidateGraph(gr, validRouteResult())
	if !containsError(err, "@all 包含了链式节点 ID 4") {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_DirectRejectAreValid(t *testing.T) {
	rr := validRouteResult()
	rr.RouteGroups[0].Members = []model.PolicyRef{
		reservedRef("DIRECT"), reservedRef("REJECT"), nodeGroupRef("🇭🇰 HK"),
	}
	rr.ResolvedRouteGroups[0].Members = []ResolvedRouteMember{
		resolvedMember(reservedRef("DIRECT"), "DIRECT", config.RouteMemberOriginLiteral),
		resolvedMember(reservedRef("REJECT"), "REJECT", config.RouteMemberOriginLiteral),
		resolvedMember(nodeGroupRef("🇭🇰 HK"), "🇭🇰 HK", config.RouteMemberOriginLiteral),
	}

	if _, err := ValidateGraph(validGroupResult(), rr); err != nil {
		t.Fatalf("DIRECT/REJECT should be valid members: %v", err)
	}
}

func TestValidateGraph_MultipleErrors(t *testing.T) {
	gr := validGroupResult()
	gr.NodeGroups[1].Members = nil

	rr := validRouteResult()
	rr.RouteGroups[0].Members = append(rr.RouteGroups[0].Members, unresolvedRef("AlsoMissing"))
	missing := config.PreparedRouteMember{Raw: "AlsoMissing", Origin: config.RouteMemberOriginLiteral}
	rr.PreparedRouteGroups[0].DeclaredMembers = append(rr.PreparedRouteGroups[0].DeclaredMembers, missing)
	rr.PreparedRouteGroups[0].ExpandedMembers = append(rr.PreparedRouteGroups[0].ExpandedMembers, missing)
	rr.ResolvedRouteGroups[0].Members = append(
		rr.ResolvedRouteGroups[0].Members,
		resolvedMember(unresolvedRef("AlsoMissing"), "AlsoMissing", config.RouteMemberOriginLiteral),
	)

	_, err := ValidateGraph(gr, rr)
	errs := unwrapAll(err)
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors, got %d: %v", len(errs), err)
	}
	for _, e := range errs {
		var buildErr *errtype.BuildError
		if !errors.As(e, &buildErr) {
			t.Errorf("expected *errtype.BuildError, got %T", e)
		} else if buildErr.Phase != "validate" {
			t.Errorf("Phase = %q, want validate", buildErr.Phase)
		}
	}
}

// T-VAL-SNELL-001: Snell proxies are ordinary ID-addressed graph nodes.
func TestValidateGraph_WithSnellProxy(t *testing.T) {
	gr := &GroupResult{
		Proxies: []model.Proxy{
			{ID: 1, Name: "HK-01", Kind: model.KindSubscription},
			{ID: 2, Name: "HK-Snell", Kind: model.KindSnell},
			{ID: 3, Name: "SG-Snell", Kind: model.KindSnell},
		},
		NodeGroups: []model.NodeGroup{
			{Name: "HK", Strategy: "select", Members: []model.ProxyID{1, 2}},
			{Name: "SG", Strategy: "select", Members: []model.ProxyID{3}},
		},
		AllProxies: []model.ProxyID{1, 2, 3},
	}
	rr := &RouteResult{
		RouteGroups: []model.RouteGroup{
			{Name: "Quick", Strategy: "select", Members: []model.PolicyRef{nodeGroupRef("HK"), nodeGroupRef("SG"), reservedRef("DIRECT")}},
			{Name: "Final", Strategy: "select", Members: []model.PolicyRef{routeGroupRef("Quick"), reservedRef("DIRECT")}},
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
}

func TestValidateGraph_ChainGroupNameCollidesWithRegionGroup(t *testing.T) {
	gr := &GroupResult{
		Proxies: []model.Proxy{
			{ID: 1, Name: "HK-01", Kind: model.KindSubscription},
			{ID: 2, Name: "HK-01→HK", Kind: model.KindChained, DialerID: 1, ChainNameSuffix: "HK"},
		},
		NodeGroups: []model.NodeGroup{
			{Name: "HK", Strategy: "select", Members: []model.ProxyID{1}},
			{Name: "HK", Strategy: "select", Members: []model.ProxyID{2}},
		},
		AllProxies: []model.ProxyID{1},
	}
	rr := &RouteResult{
		RouteGroups: []model.RouteGroup{
			{Name: "Final", Strategy: "select", Members: []model.PolicyRef{nodeGroupRef("HK"), reservedRef("DIRECT")}},
		},
		Fallback: "Final",
	}

	_, err := ValidateGraph(gr, rr)
	if !containsError(err, `节点组 "HK" 重复声明`) {
		t.Errorf("error = %v, want duplicate-declaration for HK", err)
	}
}
