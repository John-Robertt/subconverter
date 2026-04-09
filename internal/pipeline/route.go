package pipeline

import (
	"fmt"
	"strings"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// RouteResult holds the output of the Route stage (pipeline stage 6).
type RouteResult struct {
	RouteGroups []model.ProxyGroup
	Rulesets    []model.Ruleset
	Rules       []model.Rule
	Fallback    string
	// RawRouteMembers preserves the original routing declarations before @auto
	// and @all expansion so ValidateGraph can enforce routing's allowed reference types.
	RawRouteMembers map[string][]string
}

// Route executes pipeline stage 6: build service groups, rulesets,
// parse inline rules, and record fallback.
func Route(cfg *config.Config, gr *GroupResult) (*RouteResult, error) {
	if gr == nil {
		gr = &GroupResult{}
	}

	routeGroups := buildRouteGroups(&cfg.Routing, gr)
	rulesets := buildRulesets(&cfg.Rulesets)

	rules, err := parseRules(cfg.Rules)
	if err != nil {
		return nil, err
	}

	return &RouteResult{
		RouteGroups:     routeGroups,
		Rulesets:        rulesets,
		Rules:           rules,
		Fallback:        cfg.Fallback,
		RawRouteMembers: copyRouteMembers(&cfg.Routing),
	}, nil
}

// buildRouteGroups creates service-level proxy groups from routing config.
// @auto tokens are expanded first (using node groups from GroupResult),
// then @all tokens are expanded to the allProxies list.
// Service group strategy is always "select".
func buildRouteGroups(routing *config.OrderedMap[[]string], gr *GroupResult) []model.ProxyGroup {
	result := make([]model.ProxyGroup, 0, routing.Len())
	for name, members := range routing.Entries() {
		expanded := expandAutoFill(members, name, routing, gr.NodeGroups)
		expanded = expandMembers(expanded, gr.AllProxies)
		result = append(result, model.ProxyGroup{
			Name:     name,
			Scope:    model.ScopeRoute,
			Strategy: "select",
			Members:  expanded,
		})
	}
	return result
}

func copyRouteMembers(routing *config.OrderedMap[[]string]) map[string][]string {
	result := make(map[string][]string, routing.Len())
	for name, members := range routing.Entries() {
		result[name] = append([]string(nil), members...)
	}
	return result
}

// expandMembers replaces "@all" tokens with the allProxies list.
func expandMembers(members []string, allProxies []string) []string {
	result := make([]string, 0, len(members))
	for _, m := range members {
		if m == "@all" {
			result = append(result, allProxies...)
		} else {
			result = append(result, m)
		}
	}
	return result
}

// expandAutoFill replaces the "@auto" token with auto-filled members.
// The auto-fill pool (in order): all node group names (region + chained,
// declaration order), route groups containing @all (declaration order),
// DIRECT. REJECT is never auto-filled and must be declared explicitly.
// Items already present in members and the group's own name are excluded
// from the pool.
func expandAutoFill(
	members []string,
	groupName string,
	routing *config.OrderedMap[[]string],
	nodeGroups []model.ProxyGroup,
) []string {
	if !containsMember(members, "@auto") {
		return members
	}

	// Collect items already declared (excluding @auto itself).
	seen := make(map[string]struct{})
	seen[groupName] = struct{}{} // exclude self-reference
	for _, m := range members {
		if m != "@auto" {
			seen[m] = struct{}{}
		}
	}

	// Build auto-fill pool.
	var pool []string

	// 1. All node group names (region groups first, then chained groups).
	for _, g := range nodeGroups {
		if _, ok := seen[g.Name]; !ok {
			pool = append(pool, g.Name)
		}
	}

	// 2. Route groups that contain @all in their raw members.
	for name, rMembers := range routing.Entries() {
		if _, ok := seen[name]; ok {
			continue
		}
		if containsMember(rMembers, "@all") {
			pool = append(pool, name)
		}
	}

	// 3. DIRECT.
	for _, reserved := range []string{"DIRECT"} {
		if _, ok := seen[reserved]; !ok {
			pool = append(pool, reserved)
		}
	}

	// Replace @auto with the pool.
	result := make([]string, 0, len(members)+len(pool))
	for _, m := range members {
		if m == "@auto" {
			result = append(result, pool...)
		} else {
			result = append(result, m)
		}
	}
	return result
}

// buildRulesets creates Ruleset entries from rulesets config, preserving
// declaration order.
func buildRulesets(rulesets *config.OrderedMap[[]string]) []model.Ruleset {
	result := make([]model.Ruleset, 0, rulesets.Len())
	for policy, urls := range rulesets.Entries() {
		result = append(result, model.Ruleset{
			Policy: policy,
			URLs:   urls,
		})
	}
	return result
}

// parseRules extracts the policy from each raw rule string.
// Policy is the substring after the last comma.
func parseRules(rawRules []string) ([]model.Rule, error) {
	rules := make([]model.Rule, 0, len(rawRules))
	for i, raw := range rawRules {
		idx := strings.LastIndex(raw, ",")
		if idx < 0 {
			return nil, &errtype.BuildError{
				Phase:   "route",
				Message: fmt.Sprintf("rules[%d]: no comma found in %q", i, raw),
			}
		}
		rules = append(rules, model.Rule{
			Raw:    raw,
			Policy: raw[idx+1:],
		})
	}
	return rules, nil
}
