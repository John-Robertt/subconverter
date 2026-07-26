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
//  1. Proxy identity integrity and shared group/output namespace collisions
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
	validateProxyReferences(&c, gr.Proxies, gr.NodeGroups, index)

	// 2. @all must exclude chained proxies.
	chainedProxyIDs := buildChainedProxyIDSet(gr.Proxies)
	for _, id := range gr.AllProxies {
		if !index.proxyIDs[id] {
			c.add(fmt.Sprintf("@all 引用了不存在的代理 ID %d", id))
		} else if chainedProxyIDs[id] {
			c.add(fmt.Sprintf("@all 包含了链式节点 ID %d", id))
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
			validateResolvedRouteMember(&c, g.Name, member, index)
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
	proxyIDs        map[model.ProxyID]bool
	proxyKinds      map[model.ProxyID]model.ProxyKind
	proxyNames      map[string]bool
	nodeGroupNames  map[string]bool
	routeGroupNames map[string]bool
}

func buildNamespaceIndex(c *graphCollector, proxies []model.Proxy, nodeGroups []model.NodeGroup, routeGroups []model.RouteGroup) namespaceIndex {
	idx := namespaceIndex{
		proxyIDs:        make(map[model.ProxyID]bool, len(proxies)),
		proxyKinds:      make(map[model.ProxyID]model.ProxyKind, len(proxies)),
		proxyNames:      make(map[string]bool, len(proxies)),
		nodeGroupNames:  make(map[string]bool, len(nodeGroups)),
		routeGroupNames: make(map[string]bool, len(routeGroups)),
	}
	registry := map[string]string{
		"DIRECT": "保留策略",
		"REJECT": "保留策略",
	}

	for _, g := range nodeGroups {
		registerName(c, idx.nodeGroupNames, registry, g.Name, "节点组")
	}
	for _, g := range routeGroups {
		registerName(c, idx.routeGroupNames, registry, g.Name, "服务组")
	}
	for _, p := range proxies {
		if p.ID == 0 {
			c.add(fmt.Sprintf("代理 %q 的 ID 不能为空", p.Name))
		} else if idx.proxyIDs[p.ID] {
			c.add(fmt.Sprintf("代理 ID %d 重复声明", p.ID))
		} else {
			idx.proxyIDs[p.ID] = true
			idx.proxyKinds[p.ID] = p.Kind
		}
		idx.proxyNames[p.Name] = true
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

func validateProxyReferences(c *graphCollector, proxies []model.Proxy, nodeGroups []model.NodeGroup, index namespaceIndex) {
	for _, proxy := range proxies {
		if proxy.DialerID == 0 {
			continue
		}
		upstreamKind, exists := index.proxyKinds[proxy.DialerID]
		if !exists {
			c.add(fmt.Sprintf("链式代理 %q 引用了不存在的上游 ID %d", proxy.Name, proxy.DialerID))
		} else if !isFetchedKind(upstreamKind) {
			c.add(fmt.Sprintf("链式代理 %q 的上游 ID %d 必须是拉取类节点，当前 kind=%q", proxy.Name, proxy.DialerID, upstreamKind))
		}
	}
	for _, group := range nodeGroups {
		for _, id := range group.Members {
			if !index.proxyIDs[id] {
				c.add(fmt.Sprintf("节点组 %q 引用了不存在的代理 ID %d", group.Name, id))
			}
		}
	}
}

func validateResolvedRouteMember(c *graphCollector, groupName string, member ResolvedRouteMember, index namespaceIndex) {
	switch member.Ref.Kind {
	case model.PolicyRefProxy:
		if !index.proxyIDs[member.Ref.ProxyID] {
			c.add(fmt.Sprintf("服务组 %q 引用了不存在的代理 ID %d", groupName, member.Ref.ProxyID))
		} else if !allowsProxyReference(member.Origin) {
			c.add(fmt.Sprintf("服务组 %q 的成员 %q 不能直接引用代理", groupName, member.Raw))
		}
	case model.PolicyRefNodeGroup:
		if !index.nodeGroupNames[member.Ref.Name] {
			c.add(fmt.Sprintf("服务组 %q 引用了不存在的节点组 %q", groupName, member.Ref.Name))
		}
	case model.PolicyRefRouteGroup:
		if !index.routeGroupNames[member.Ref.Name] {
			c.add(fmt.Sprintf("服务组 %q 引用了不存在的服务组 %q", groupName, member.Ref.Name))
		}
	case model.PolicyRefReserved:
		if !reservedPolicies[member.Ref.Name] {
			c.add(fmt.Sprintf("服务组 %q 引用了不存在的保留策略 %q", groupName, member.Ref.Name))
		}
	default:
		c.add(fmt.Sprintf("服务组 %q 的成员 %q 不存在", groupName, member.Raw))
	}
}

func buildChainedProxyIDSet(proxies []model.Proxy) map[model.ProxyID]bool {
	s := make(map[model.ProxyID]bool)
	for _, p := range proxies {
		if p.Kind == model.KindChained {
			s[p.ID] = true
		}
	}
	return s
}

func detectCycle(groups []model.RouteGroup, routeGroupNames map[string]bool) string {
	adj := make(map[string][]string)
	order := make([]string, 0, len(groups))
	for _, g := range groups {
		order = append(order, g.Name)
		for _, member := range g.Members {
			if member.Kind == model.PolicyRefRouteGroup && routeGroupNames[member.Name] {
				adj[g.Name] = append(adj[g.Name], member.Name)
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
				Raw:    policyRefRaw(member),
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
		members := make([]ResolvedRouteMember, 0, len(group.Members))
		for _, member := range group.Members {
			members = append(members, ResolvedRouteMember{
				Ref:    member,
				Raw:    policyRefRaw(member),
				Origin: config.RouteMemberOriginLiteral,
			})
		}
		result = append(result, ResolvedRouteGroup{Name: group.Name, Members: members})
	}
	return result
}

func policyRefRaw(ref model.PolicyRef) string {
	if ref.Kind == model.PolicyRefProxy {
		return fmt.Sprintf("proxy:%d", ref.ProxyID)
	}
	return ref.Name
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
