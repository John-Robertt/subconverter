package target

import (
	"fmt"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

type proxyDropReasonKind string

const (
	proxyDropReasonProtoRoot     proxyDropReasonKind = "proto_root"
	proxyDropReasonDialerCascade proxyDropReasonKind = "dialer_cascade"
)

type proxyDropReason struct {
	Kind   proxyDropReasonKind
	Parent string
}

type cascadeOptions struct {
	formatName        string
	formatDisplayName string
	rootLabel         string
	emptyCode         errtype.Code
	internalCode      errtype.Code
	emptyReasonClause string
}

func filterByDroppedTypes(p *model.Pipeline, droppedTypes []string, opts cascadeOptions) (*model.Pipeline, error) {
	typeSet := make(map[string]struct{}, len(droppedTypes))
	for _, t := range droppedTypes {
		typeSet[t] = struct{}{}
	}

	dropped := make(map[string]struct{})
	proxyReasons := make(map[string]proxyDropReason)
	for _, px := range p.Proxies {
		if _, hit := typeSet[px.Type]; hit {
			dropped[px.Name] = struct{}{}
			proxyReasons[px.Name] = proxyDropReason{Kind: proxyDropReasonProtoRoot}
		}
	}

	maxChainIter := len(p.Proxies) + 1
	for i := 0; ; i++ {
		if i >= maxChainIter {
			return nil, internalFilterError(opts, "链式剔除未收敛（超过 %d 轮），疑似循环 dialer 引用", maxChainIter)
		}
		grew := false
		for _, px := range p.Proxies {
			if px.Dialer == "" {
				continue
			}
			if _, isDropped := dropped[px.Name]; isDropped {
				continue
			}
			if _, upstreamDropped := dropped[px.Dialer]; upstreamDropped {
				dropped[px.Name] = struct{}{}
				proxyReasons[px.Name] = proxyDropReason{
					Kind:   proxyDropReasonDialerCascade,
					Parent: px.Dialer,
				}
				grew = true
			}
		}
		if !grew {
			break
		}
	}

	if len(dropped) == 0 {
		return p, nil
	}

	droppedGroups := make(map[string]struct{})
	groupReasons := make(map[string][]string)
	filteredNodeGroups := make([]model.ProxyGroup, len(p.NodeGroups))
	copy(filteredNodeGroups, p.NodeGroups)
	filteredRouteGroups := make([]model.ProxyGroup, len(p.RouteGroups))
	copy(filteredRouteGroups, p.RouteGroups)

	maxGroupIter := len(filteredNodeGroups) + len(filteredRouteGroups) + 1
	for i := 0; ; i++ {
		if i >= maxGroupIter {
			return nil, internalFilterError(opts, "组级联剔除未收敛（超过 %d 轮），疑似相互引用", maxGroupIter)
		}
		grew := false
		for i, g := range filteredNodeGroups {
			if _, alreadyDropped := droppedGroups[g.Name]; alreadyDropped {
				continue
			}
			prevMembers := filteredNodeGroups[i].Members
			pruned := pruneMembers(prevMembers, dropped, droppedGroups)
			filteredNodeGroups[i].Members = pruned
			if len(pruned) == 0 {
				droppedGroups[g.Name] = struct{}{}
				groupReasons[g.Name] = append([]string(nil), prevMembers...)
				grew = true
			}
		}
		for i, g := range filteredRouteGroups {
			if _, alreadyDropped := droppedGroups[g.Name]; alreadyDropped {
				continue
			}
			prevMembers := filteredRouteGroups[i].Members
			pruned := pruneMembers(prevMembers, dropped, droppedGroups)
			filteredRouteGroups[i].Members = pruned
			if len(pruned) == 0 {
				droppedGroups[g.Name] = struct{}{}
				groupReasons[g.Name] = append([]string(nil), prevMembers...)
				grew = true
			}
		}
		if !grew {
			break
		}
	}

	if _, fallbackDropped := droppedGroups[p.Fallback]; fallbackDropped {
		path := buildDropPath(p.Fallback, groupReasons, proxyReasons, opts.rootLabel, make(map[string]bool))
		return nil, &errtype.RenderError{
			Code:    opts.emptyCode,
			Format:  opts.formatName,
			Message: fmt.Sprintf("fallback 服务组 %q 在 %s 输出中成员为空（%s）。清空路径：%s", p.Fallback, opts.formatDisplayName, opts.emptyReasonClause, path),
		}
	}

	nodeGroupsOut := make([]model.ProxyGroup, 0, len(filteredNodeGroups))
	for _, g := range filteredNodeGroups {
		if _, drop := droppedGroups[g.Name]; drop {
			continue
		}
		nodeGroupsOut = append(nodeGroupsOut, g)
	}
	routeGroupsOut := make([]model.ProxyGroup, 0, len(filteredRouteGroups))
	for _, g := range filteredRouteGroups {
		if _, drop := droppedGroups[g.Name]; drop {
			continue
		}
		routeGroupsOut = append(routeGroupsOut, g)
	}

	rulesetsOut := make([]model.Ruleset, 0, len(p.Rulesets))
	for _, rs := range p.Rulesets {
		if _, drop := droppedGroups[rs.Policy]; drop {
			continue
		}
		rulesetsOut = append(rulesetsOut, rs)
	}

	rulesOut := make([]model.Rule, 0, len(p.Rules))
	for _, r := range p.Rules {
		if _, drop := droppedGroups[r.Policy]; drop {
			continue
		}
		rulesOut = append(rulesOut, r)
	}

	proxiesOut := make([]model.Proxy, 0, len(p.Proxies))
	for _, px := range p.Proxies {
		if _, drop := dropped[px.Name]; drop {
			continue
		}
		proxiesOut = append(proxiesOut, px)
	}

	allOut := make([]string, 0, len(p.AllProxies))
	for _, name := range p.AllProxies {
		if _, drop := dropped[name]; drop {
			continue
		}
		allOut = append(allOut, name)
	}

	return &model.Pipeline{
		Proxies:     proxiesOut,
		NodeGroups:  nodeGroupsOut,
		RouteGroups: routeGroupsOut,
		Rulesets:    rulesetsOut,
		Rules:       rulesOut,
		Fallback:    p.Fallback,
		AllProxies:  allOut,
	}, nil
}

func pruneMembers(members []string, droppedProxies, droppedGroups map[string]struct{}) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		if _, d := droppedProxies[m]; d {
			continue
		}
		if _, d := droppedGroups[m]; d {
			continue
		}
		out = append(out, m)
	}
	return out
}

func buildDropPath(name string, groupReasons map[string][]string, proxyReasons map[string]proxyDropReason, rootLabel string, visited map[string]bool) string {
	if visited[name] {
		return name + "(cycle)"
	}

	if reason, isProxy := proxyReasons[name]; isProxy {
		switch reason.Kind {
		case proxyDropReasonProtoRoot:
			return name + "(" + rootLabel + ")"
		case proxyDropReasonDialerCascade:
			visited[name] = true
			parentPath := reason.Parent
			if reason.Parent != "" {
				parentPath = buildDropPath(reason.Parent, groupReasons, proxyReasons, rootLabel, visited)
			}
			delete(visited, name)
			if parentPath == "" {
				return name + "(chained)"
			}
			return fmt.Sprintf("%s(chained) ← [%s]", name, parentPath)
		}
		return name
	}

	members, isGroup := groupReasons[name]
	if !isGroup {
		return name
	}

	visited[name] = true
	parts := make([]string, 0, len(members))
	for _, m := range members {
		parts = append(parts, buildDropPath(m, groupReasons, proxyReasons, rootLabel, visited))
	}
	delete(visited, name)
	return fmt.Sprintf("%s ← [%s]", name, strings.Join(parts, ", "))
}

func internalFilterError(opts cascadeOptions, format string, args ...any) error {
	return &errtype.RenderError{
		Code:    opts.internalCode,
		Format:  opts.formatName,
		Message: "内部不变量异常：" + fmt.Sprintf(format, args...),
	}
}
