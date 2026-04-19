package pipeline

import (
	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/model"
)

// ResolvedRouteGroup stores the final request-time route members with provenance.
type ResolvedRouteGroup struct {
	Name    string
	Members []config.PreparedRouteMember
}

// RouteResult holds the output of the Route stage (pipeline stage 6).
type RouteResult struct {
	RouteGroups         []model.ProxyGroup
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
func buildRouteGroups(routing []config.PreparedRouteGroup, gr *GroupResult) ([]model.ProxyGroup, []ResolvedRouteGroup) {
	modelGroups := make([]model.ProxyGroup, 0, len(routing))
	resolvedGroups := make([]ResolvedRouteGroup, 0, len(routing))
	for _, group := range routing {
		resolved := expandResolvedMembers(group.ExpandedMembers, gr.AllProxies)
		memberNames := make([]string, 0, len(resolved))
		for _, member := range resolved {
			memberNames = append(memberNames, member.Raw)
		}
		modelGroups = append(modelGroups, model.ProxyGroup{
			Name:     group.Name,
			Scope:    model.ScopeRoute,
			Strategy: "select",
			Members:  memberNames,
		})
		resolvedGroups = append(resolvedGroups, ResolvedRouteGroup{
			Name:    group.Name,
			Members: resolved,
		})
	}
	return modelGroups, resolvedGroups
}

func expandResolvedMembers(members []config.PreparedRouteMember, allProxies []string) []config.PreparedRouteMember {
	result := make([]config.PreparedRouteMember, 0, len(members))
	for _, member := range members {
		if member.Raw != "@all" {
			result = append(result, member)
			continue
		}
		for _, proxyName := range allProxies {
			result = append(result, config.PreparedRouteMember{
				Raw:    proxyName,
				Origin: config.RouteMemberOriginAllExpanded,
			})
		}
	}
	return result
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
