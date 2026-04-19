package pipeline

import (
	"errors"
	"fmt"

	"github.com/John-Robertt/subconverter/internal/config"
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
//  4. Declared route members only use allowed reference kinds
//  5. Resolved route members only reference existing objects
//  6. Circular references among route groups
//
// Ruleset/rule policy existence and fallback existence are guaranteed by
// config.Prepare at startup and not re-checked here.
func ValidateGraph(gr *GroupResult, rr *RouteResult) (*model.Pipeline, error) {
	if gr == nil {
		gr = &GroupResult{}
	}
	if rr == nil {
		rr = &RouteResult{}
	}

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

	// 4. Declared route member contract.
	for _, g := range preparedRouteGroups(rr) {
		for _, member := range g.DeclaredMembers {
			switch {
			case member.Raw == "@all", member.Raw == "@auto", reservedPolicies[member.Raw], index.nodeGroupNames[member.Raw], index.routeGroupNames[member.Raw]:
				continue
			case index.proxyNames[member.Raw]:
				c.add(fmt.Sprintf("服务组 %q 的成员 %q 必须引用节点组、服务组、DIRECT、REJECT、@all 或 @auto", g.Name, member.Raw))
			default:
				c.add(fmt.Sprintf("服务组 %q 的成员 %q 不存在", g.Name, member.Raw))
			}
		}
	}

	// 5. Resolved route member resolution with provenance.
	for _, g := range resolvedRouteGroups(rr) {
		for _, member := range g.Members {
			if member.Origin == config.RouteMemberOriginLiteral {
				continue
			}
			switch {
			case reservedPolicies[member.Raw], index.nodeGroupNames[member.Raw], index.routeGroupNames[member.Raw]:
				continue
			case index.proxyNames[member.Raw]:
				if allowsProxyReference(member.Origin) {
					continue
				}
				c.add(fmt.Sprintf("服务组 %q 的成员 %q 必须引用节点组、服务组、DIRECT、REJECT、@all 或 @auto", g.Name, member.Raw))
			default:
				c.add(fmt.Sprintf("服务组 %q 的成员 %q 不存在", g.Name, member.Raw))
			}
		}
	}

	// 6. Circular references among route groups.
	if cycle := detectCycle(rr.RouteGroups, index.routeGroupNames); cycle != "" {
		c.add(cycle)
	}

	// Checks 7-9 (ruleset/rule policy existence, fallback existence) are
	// guaranteed by config.Prepare at startup and omitted here.

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

func detectCycle(groups []model.ProxyGroup, routeGroupNames map[string]bool) string {
	adj := make(map[string][]string)
	order := make([]string, 0, len(groups))
	for _, g := range groups {
		order = append(order, g.Name)
		for _, m := range g.Members {
			if routeGroupNames[m] {
				adj[g.Name] = append(adj[g.Name], m)
			}
		}
	}
	return config.DetectRouteCycle(adj, order)
}

func preparedRouteGroups(rr *RouteResult) []config.PreparedRouteGroup {
	if len(rr.PreparedRouteGroups) > 0 {
		return rr.PreparedRouteGroups
	}
	result := make([]config.PreparedRouteGroup, 0, len(rr.RouteGroups))
	for _, group := range rr.RouteGroups {
		members := make([]config.PreparedRouteMember, 0, len(group.Members))
		for _, member := range group.Members {
			members = append(members, config.PreparedRouteMember{
				Raw:    member,
				Origin: config.RouteMemberOriginLiteral,
			})
		}
		result = append(result, config.PreparedRouteGroup{
			Name:            group.Name,
			DeclaredMembers: members,
			ExpandedMembers: members,
		})
	}
	return result
}

func resolvedRouteGroups(rr *RouteResult) []ResolvedRouteGroup {
	if len(rr.ResolvedRouteGroups) > 0 {
		return rr.ResolvedRouteGroups
	}
	result := make([]ResolvedRouteGroup, 0, len(rr.RouteGroups))
	for _, group := range rr.RouteGroups {
		members := make([]config.PreparedRouteMember, 0, len(group.Members))
		for _, member := range group.Members {
			members = append(members, config.PreparedRouteMember{
				Raw:    member,
				Origin: config.RouteMemberOriginLiteral,
			})
		}
		result = append(result, ResolvedRouteGroup{Name: group.Name, Members: members})
	}
	return result
}

func allowsProxyReference(origin config.RouteMemberOrigin) bool {
	return origin == config.RouteMemberOriginAutoExpanded || origin == config.RouteMemberOriginAllExpanded
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
