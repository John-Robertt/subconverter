package pipeline

import (
	"errors"
	"fmt"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// reservedPolicies are built-in policy names that need no group/proxy resolution.
var reservedPolicies = map[string]bool{
	"DIRECT": true,
	"REJECT": true,
}

// ValidateGraph performs graph-level semantic validation on the outputs of the
// Group and Route stages, then assembles a Pipeline.
//
// Checks (in order):
//  1. Node group / route group name collision
//  2. Empty node groups (region and chained)
//  3. Route group member reference resolution
//  4. Circular references among route groups
//  5. Ruleset policy existence
//  6. Rule policy existence
//  7. Fallback existence
func ValidateGraph(gr *GroupResult, rr *RouteResult) (*model.Pipeline, error) {
	var c graphCollector

	proxyNames := buildNameSet(gr.Proxies)
	nodeGroupNames := buildGroupNameSet(gr.NodeGroups)
	routeGroupNames := buildGroupNameSet(rr.RouteGroups)

	// 1. Name collision between node groups and route groups.
	for name := range nodeGroupNames {
		if routeGroupNames[name] {
			c.add(fmt.Sprintf("name %q used by both node group and route group", name))
		}
	}

	// 2. Empty node groups.
	for _, g := range gr.NodeGroups {
		if len(g.Members) == 0 {
			if strings.HasPrefix(g.Name, chainedGroupPrefix) {
				c.add(fmt.Sprintf("chained group %q has no members", g.Name))
			} else {
				c.add(fmt.Sprintf("node group %q has no members", g.Name))
			}
		}
	}

	// 3. Route group member resolution.
	for _, g := range rr.RouteGroups {
		for _, member := range g.Members {
			if !proxyNames[member] && !nodeGroupNames[member] && !routeGroupNames[member] && !reservedPolicies[member] {
				c.add(fmt.Sprintf("route group %q: member %q not found", g.Name, member))
			}
		}
	}

	// 4. Circular references among route groups.
	if cycle := detectCycle(rr.RouteGroups, routeGroupNames); cycle != "" {
		c.add(cycle)
	}

	// 5. Ruleset policy existence.
	for _, rs := range rr.Rulesets {
		if !routeGroupNames[rs.Policy] {
			c.add(fmt.Sprintf("ruleset policy %q not found in routing", rs.Policy))
		}
	}

	// 6. Rule policy existence.
	for _, r := range rr.Rules {
		if !routeGroupNames[r.Policy] && !reservedPolicies[r.Policy] {
			c.add(fmt.Sprintf("rule policy %q not found in routing", r.Policy))
		}
	}

	// 7. Fallback existence.
	if !routeGroupNames[rr.Fallback] {
		c.add(fmt.Sprintf("fallback %q not found in routing", rr.Fallback))
	}

	if err := c.result(); err != nil {
		return nil, err
	}

	return &model.Pipeline{
		Proxies:     gr.Proxies,
		NodeGroups:  gr.NodeGroups,
		RouteGroups: rr.RouteGroups,
		Rulesets:    rr.Rulesets,
		Rules:       rr.Rules,
		Fallback:    rr.Fallback,
		AllProxies:  gr.AllProxies,
	}, nil
}

// detectCycle checks for circular references among route groups using
// DFS with white/gray/black coloring. Returns an error message on cycle,
// or empty string if no cycle exists.
func detectCycle(groups []model.ProxyGroup, routeGroupNames map[string]bool) string {
	adj := make(map[string][]string)
	for _, g := range groups {
		for _, m := range g.Members {
			if routeGroupNames[m] {
				adj[g.Name] = append(adj[g.Name], m)
			}
		}
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)

	var dfs func(node string) string
	dfs = func(node string) string {
		color[node] = gray
		for _, neighbor := range adj[node] {
			switch color[neighbor] {
			case gray:
				return fmt.Sprintf("circular reference among route groups: %s -> %s", node, neighbor)
			case white:
				if msg := dfs(neighbor); msg != "" {
					return msg
				}
			}
		}
		color[node] = black
		return ""
	}

	for _, g := range groups {
		if color[g.Name] == white {
			if msg := dfs(g.Name); msg != "" {
				return msg
			}
		}
	}
	return ""
}

// buildNameSet creates a set of proxy names.
func buildNameSet(proxies []model.Proxy) map[string]bool {
	s := make(map[string]bool, len(proxies))
	for _, p := range proxies {
		s[p.Name] = true
	}
	return s
}

// buildGroupNameSet creates a set of group names.
func buildGroupNameSet(groups []model.ProxyGroup) map[string]bool {
	s := make(map[string]bool, len(groups))
	for _, g := range groups {
		s[g.Name] = true
	}
	return s
}

// graphCollector accumulates graph validation errors.
type graphCollector struct {
	errs []error
}

func (c *graphCollector) add(msg string) {
	c.errs = append(c.errs, &errtype.BuildError{Phase: "validate", Message: msg})
}

func (c *graphCollector) result() error {
	return errors.Join(c.errs...)
}
