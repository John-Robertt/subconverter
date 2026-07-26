package pipeline

import (
	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ResolvedRouteMember stores one typed route reference plus its declaration
// provenance for validation and preview.
type ResolvedRouteMember struct {
	Ref    model.PolicyRef
	Raw    string
	Origin config.RouteMemberOrigin
}

// ResolvedRouteGroup stores the final request-time route members with provenance.
type ResolvedRouteGroup struct {
	Name    string
	Members []ResolvedRouteMember
}

// RouteResult holds the output of the Route stage (pipeline stage 6).
type RouteResult struct {
	RouteGroups         []model.RouteGroup
	PreparedRouteGroups []config.PreparedRouteGroup
	ResolvedRouteGroups []ResolvedRouteGroup
	Rulesets            []model.Ruleset
	Rules               []model.Rule
	Fallback            string
}

// Route executes pipeline stage 6: build service groups, rulesets,
// and record fallback.
func Route(
	routing []config.PreparedRouteGroup,
	rulesets []config.PreparedRuleset,
	rules []config.PreparedRule,
	fallback string,
	gr *GroupResult,
) (*RouteResult, error) {
	if gr == nil {
		gr = &GroupResult{}
	}

	routeGroups, resolvedGroups := buildRouteGroups(routing, gr)
	rulesetResults := buildRulesets(rulesets)

	return &RouteResult{
		RouteGroups:         routeGroups,
		PreparedRouteGroups: routing,
		ResolvedRouteGroups: resolvedGroups,
		Rulesets:            rulesetResults,
		Rules:               buildRules(rules),
		Fallback:            fallback,
	}, nil
}

// buildRouteGroups creates service-level proxy groups from routing config.
// @auto has already been expanded at startup; Route only expands @all.
// Service group strategy is always "select".
func buildRouteGroups(routing []config.PreparedRouteGroup, gr *GroupResult) ([]model.RouteGroup, []ResolvedRouteGroup) {
	modelGroups := make([]model.RouteGroup, 0, len(routing))
	resolvedGroups := make([]ResolvedRouteGroup, 0, len(routing))
	nodeGroupNames := make(map[string]bool, len(gr.NodeGroups))
	for _, group := range gr.NodeGroups {
		nodeGroupNames[group.Name] = true
	}
	routeGroupNames := make(map[string]bool, len(routing))
	for _, group := range routing {
		routeGroupNames[group.Name] = true
	}
	for _, group := range routing {
		resolved := expandResolvedMembers(group.ExpandedMembers, gr.AllProxies, nodeGroupNames, routeGroupNames)
		refs := make([]model.PolicyRef, 0, len(resolved))
		for _, member := range resolved {
			refs = append(refs, member.Ref)
		}
		modelGroups = append(modelGroups, model.RouteGroup{
			Name:     group.Name,
			Strategy: "select",
			Members:  refs,
		})
		resolvedGroups = append(resolvedGroups, ResolvedRouteGroup{
			Name:    group.Name,
			Members: resolved,
		})
	}
	return modelGroups, resolvedGroups
}

func expandResolvedMembers(
	members []config.PreparedRouteMember,
	allProxies []model.ProxyID,
	nodeGroupNames map[string]bool,
	routeGroupNames map[string]bool,
) []ResolvedRouteMember {
	result := make([]ResolvedRouteMember, 0, len(members))
	for _, member := range members {
		if member.Raw != "@all" {
			result = append(result, ResolvedRouteMember{
				Ref:    resolveNamedPolicyRef(member.Raw, nodeGroupNames, routeGroupNames),
				Raw:    member.Raw,
				Origin: member.Origin,
			})
			continue
		}
		for _, proxyID := range allProxies {
			result = append(result, ResolvedRouteMember{
				Ref: model.PolicyRef{
					Kind:    model.PolicyRefProxy,
					ProxyID: proxyID,
				},
				Raw:    member.Raw,
				Origin: config.RouteMemberOriginAllExpanded,
			})
		}
	}
	return result
}

func resolveNamedPolicyRef(name string, nodeGroupNames, routeGroupNames map[string]bool) model.PolicyRef {
	switch {
	case config.IsReservedPolicyName(name):
		return model.PolicyRef{Kind: model.PolicyRefReserved, Name: name}
	case nodeGroupNames[name]:
		return model.PolicyRef{Kind: model.PolicyRefNodeGroup, Name: name}
	case routeGroupNames[name]:
		return model.PolicyRef{Kind: model.PolicyRefRouteGroup, Name: name}
	default:
		return model.PolicyRef{Name: name}
	}
}

// buildRulesets creates Ruleset entries from rulesets config, preserving
// declaration order.
func buildRulesets(rulesets []config.PreparedRuleset) []model.Ruleset {
	result := make([]model.Ruleset, 0, len(rulesets))
	for _, ruleset := range rulesets {
		result = append(result, model.Ruleset{
			Policy: ruleset.Policy,
			URLs:   append([]string(nil), ruleset.URLs...),
		})
	}
	return result
}

func buildRules(prepared []config.PreparedRule) []model.Rule {
	rules := make([]model.Rule, 0, len(prepared))
	for _, rule := range prepared {
		rules = append(rules, model.Rule{
			Raw:    rule.Raw,
			Policy: rule.Policy,
		})
	}
	return rules
}
