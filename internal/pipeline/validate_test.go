package pipeline

import (
	"errors"
	"strings"
	"testing"

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
		Rulesets: []model.Ruleset{
			{Policy: "Quick", URLs: []string{"https://example.com/rule.list"}},
		},
		Rules: []model.Rule{
			{Raw: "GEOIP,CN,Final", Policy: "Final"},
		},
		Fallback: "Final",
		RawRouteMembers: map[string][]string{
			"Quick": []string{"🇭🇰 HK", "🔗 MY-PROXY", "DIRECT"},
			"Final": []string{"Quick", "DIRECT"},
		},
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
	rr.RawRouteMembers["Quick"] = append(rr.RawRouteMembers["Quick"], "Nonexistent")

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `member "Nonexistent" not found`) {
		t.Errorf("error = %v, want mention of Nonexistent", err)
	}
	if got := countErrorContains(err, `member "Nonexistent" not found`); got != 1 {
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
	if !containsError(err, "circular reference") {
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
	if !containsError(err, "circular reference") {
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
	if !containsError(err, `chained group "🔗 MY-PROXY" has no members`) {
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
	if !containsError(err, `node group "🇭🇰 HK" has no members`) {
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
	if !containsError(err, `name "🇭🇰 HK" used by both`) {
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
	if !containsError(err, `name "HK-01" used by both proxy and node group`) {
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
	if !containsError(err, `route group "Quick" declared more than once`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_RouteGroupExplicitProxyMemberRejected(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.RawRouteMembers["Quick"] = []string{"HK-01", "DIRECT"}
	rr.RouteGroups[0].Members = []string{"HK-01", "DIRECT"}

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `member "HK-01" must reference a node group, route group, DIRECT, REJECT, @all, or @auto`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_RouteGroupExpandedProxyMembersAllowedViaAtAll(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.RawRouteMembers["Quick"] = []string{"@all", "DIRECT"}
	rr.RouteGroups[0].Members = []string{"HK-01", "SG-01", "MY-PROXY", "DIRECT"}

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
	if !containsError(err, `@all contains chained proxy "HK-01→MY-PROXY"`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_RulesetPolicyNotFound(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.Rulesets = []model.Ruleset{
		{Policy: "Nonexistent", URLs: []string{"https://example.com/rule.list"}},
	}

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `ruleset policy "Nonexistent" not found`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_RulePolicyNotFound(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.Rules = []model.Rule{
		{Raw: "GEOIP,CN,Missing", Policy: "Missing"},
	}

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `rule policy "Missing" not found`) {
		t.Errorf("error = %v", err)
	}
}

func TestValidateGraph_FallbackNotFound(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()
	rr.Fallback = "NonexistentFallback"

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsError(err, `fallback "NonexistentFallback" not found`) {
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
	rr.Fallback = "Missing"                                                      // bad fallback
	rr.RouteGroups[0].Members = append(rr.RouteGroups[0].Members, "AlsoMissing") // bad member
	rr.RawRouteMembers["Quick"] = append(rr.RawRouteMembers["Quick"], "AlsoMissing")

	_, err := ValidateGraph(gr, rr)
	if err == nil {
		t.Fatal("expected multiple errors")
	}
	errs := unwrapAll(err)
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors, got %d: %v", len(errs), err)
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

// TestValidateGraph_AutoTokenAccepted: @auto in raw routing members is accepted
func TestValidateGraph_AutoTokenAccepted(t *testing.T) {
	gr := validGroupResult()
	rr := validRouteResult()

	// Add @auto to raw members of Quick group; the expanded members stay valid.
	rr.RawRouteMembers["Quick"] = []string{"🇭🇰 HK", "@auto"}

	_, err := ValidateGraph(gr, rr)
	if err != nil {
		t.Errorf("@auto should be accepted in raw routing members, got: %v", err)
	}
}
