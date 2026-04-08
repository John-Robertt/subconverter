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
	// RawRouteMembers preserves the original routing declarations before @all
	// expansion so ValidateGraph can enforce routing's allowed reference types.
	RawRouteMembers map[string][]string
}

// Route executes pipeline stage 6: build service groups, rulesets,
// parse inline rules, and record fallback.
func Route(cfg *config.Config, allProxies []string) (*RouteResult, error) {
	routeGroups := buildRouteGroups(&cfg.Routing, allProxies)
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
// @all tokens in members are expanded to the allProxies list.
// Service group strategy is always "select".
func buildRouteGroups(routing *config.OrderedMap[[]string], allProxies []string) []model.ProxyGroup {
	result := make([]model.ProxyGroup, 0, routing.Len())
	for name, members := range routing.Entries() {
		result = append(result, model.ProxyGroup{
			Name:     name,
			Scope:    model.ScopeRoute,
			Strategy: "select",
			Members:  expandMembers(members, allProxies),
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
