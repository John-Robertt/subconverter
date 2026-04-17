package pipeline

import (
	"errors"
	"fmt"

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
//  1. Shared namespace collisions and duplicate declarations
//  2. @all expansion excludes chained proxies
//  3. Empty node groups (region and chained)
//  4. Raw routing members only reference allowed object types
//  5. Expanded route group member reference resolution
//  6. Circular references among route groups
//  7. Ruleset policy existence
//  8. Rule policy existence
//  9. Fallback existence
func ValidateGraph(gr *GroupResult, rr *RouteResult) (*model.Pipeline, error) {
	var c graphCollector

	index := buildNamespaceIndex(&c, gr.Proxies, gr.NodeGroups, rr.RouteGroups)

	// 2. @all must exclude chained proxies.
	chainedProxyNames := buildChainedProxyNameSet(gr.Proxies)
	for _, name := range gr.AllProxies {
		if chainedProxyNames[name] {
			c.add(fmt.Sprintf("@all 包含了链式节点 %q", name))
		}
	}

	// 3. Empty node groups.
	for _, g := range gr.NodeGroups {
		if len(g.Members) == 0 {
			c.add(fmt.Sprintf("节点组 %q 没有成员", g.Name))
		}
	}

	// 4. Raw routing members may only reference node groups, route groups,
	// reserved policies, @all, or @auto.
	for _, g := range rr.RouteGroups {
		rawMembers := g.Members
		if rr.RawRouteMembers != nil {
			if members, ok := rr.RawRouteMembers[g.Name]; ok {
				rawMembers = members
			}
		}

		for _, member := range rawMembers {
			if member == "@all" || member == "@auto" || reservedPolicies[member] || index.nodeGroupNames[member] || index.routeGroupNames[member] {
				continue
			}
			if index.proxyNames[member] {
				c.add(fmt.Sprintf(
					"服务组 %q 的成员 %q 必须引用节点组、服务组、DIRECT、REJECT、@all 或 @auto",
					g.Name,
					member,
				))
				continue
			}
			c.add(fmt.Sprintf("服务组 %q 的成员 %q 不存在", g.Name, member))
		}
	}

	// 5. Expanded @all member resolution.
	for _, g := range rr.RouteGroups {
		rawMembers := g.Members
		if rr.RawRouteMembers != nil {
			if members, ok := rr.RawRouteMembers[g.Name]; ok {
				rawMembers = members
			}
		}
		if !containsMember(rawMembers, "@all") {
			continue
		}

		rawMemberSet := make(map[string]struct{}, len(rawMembers))
		for _, member := range rawMembers {
			rawMemberSet[member] = struct{}{}
		}

		for _, member := range g.Members {
			if _, presentInRaw := rawMemberSet[member]; presentInRaw {
				continue
			}
			if !index.proxyNames[member] {
				c.add(fmt.Sprintf("服务组 %q 的成员 %q 不存在", g.Name, member))
			}
		}
	}

	// 6. Circular references among route groups.
	if cycle := detectCycle(rr.RouteGroups, index.routeGroupNames); cycle != "" {
		c.add(cycle)
	}

	// 7. Ruleset policy existence.
	for _, rs := range rr.Rulesets {
		if !index.routeGroupNames[rs.Policy] {
			c.add(fmt.Sprintf("ruleset 策略 %q 未在 routing 中定义", rs.Policy))
		}
	}

	// 8. Rule policy existence.
	for _, r := range rr.Rules {
		if !index.routeGroupNames[r.Policy] && !reservedPolicies[r.Policy] {
			c.add(fmt.Sprintf("规则策略 %q 未在 routing 中定义", r.Policy))
		}
	}

	// 9. Fallback existence.
	if !index.routeGroupNames[rr.Fallback] {
		c.add(fmt.Sprintf("fallback %q 未在 routing 中定义", rr.Fallback))
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

type namespaceIndex struct {
	proxyNames      map[string]bool
	nodeGroupNames  map[string]bool
	routeGroupNames map[string]bool
}

func buildNamespaceIndex(c *graphCollector, proxies []model.Proxy, nodeGroups, routeGroups []model.ProxyGroup) namespaceIndex {
	idx := namespaceIndex{
		proxyNames:      make(map[string]bool, len(proxies)),
		nodeGroupNames:  make(map[string]bool, len(nodeGroups)),
		routeGroupNames: make(map[string]bool, len(routeGroups)),
	}
	registry := make(map[string]string, len(proxies)+len(nodeGroups)+len(routeGroups))

	for _, p := range proxies {
		registerName(c, idx.proxyNames, registry, p.Name, "代理")
	}
	for _, g := range nodeGroups {
		registerName(c, idx.nodeGroupNames, registry, g.Name, "节点组")
	}
	for _, g := range routeGroups {
		registerName(c, idx.routeGroupNames, registry, g.Name, "服务组")
	}

	return idx
}

func registerName(c *graphCollector, names map[string]bool, registry map[string]string, name, kind string) {
	if names[name] {
		c.add(fmt.Sprintf("%s %q 重复声明", kind, name))
		return
	}
	names[name] = true

	if other, ok := registry[name]; ok {
		c.add(fmt.Sprintf("名称 %q 同时被 %s 和 %s 使用", name, other, kind))
		return
	}
	registry[name] = kind
}

func buildChainedProxyNameSet(proxies []model.Proxy) map[string]bool {
	s := make(map[string]bool)
	for _, p := range proxies {
		if p.Kind == model.KindChained {
			s[p.Name] = true
		}
	}
	return s
}

func containsMember(members []string, target string) bool {
	for _, member := range members {
		if member == target {
			return true
		}
	}
	return false
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
				return fmt.Sprintf("服务组存在循环引用：%s -> %s", node, neighbor)
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

// graphCollector accumulates graph validation errors.
type graphCollector struct {
	errs []error
}

func (c *graphCollector) add(msg string) {
	c.errs = append(c.errs, &errtype.BuildError{Code: errtype.CodeBuildValidationFailed, Phase: "validate", Message: msg})
}

func (c *graphCollector) result() error {
	return errors.Join(c.errs...)
}
