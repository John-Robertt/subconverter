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

// proxyCascade / groupCascade 都通过指针传递。它们内部的 map 会被 helper 原地增长，
// 指针签名显式传达"调用方看得到 mutation"的语义，避免按值传递时对 map header 共享的误读。
type proxyCascade struct {
	dropped map[string]struct{}
	reasons map[string]proxyDropReason
}

type groupCascade struct {
	nodeGroups  []model.ProxyGroup
	routeGroups []model.ProxyGroup
	dropped     map[string]struct{}
	reasons     map[string][]string
}

func filterByDroppedTypes(p *model.Pipeline, droppedTypes []string, opts cascadeOptions) (*model.Pipeline, error) {
	proxyDrops, err := buildProxyCascade(p.Proxies, droppedTypes, opts)
	if err != nil {
		return nil, err
	}
	if len(proxyDrops.dropped) == 0 {
		return p, nil
	}

	groupDrops, err := buildGroupCascade(p.NodeGroups, p.RouteGroups, proxyDrops.dropped, opts)
	if err != nil {
		return nil, err
	}

	if _, fallbackDropped := groupDrops.dropped[p.Fallback]; fallbackDropped {
		return nil, fallbackEmptyError(p.Fallback, groupDrops.reasons, proxyDrops.reasons, opts)
	}

	return projectPipeline(p, proxyDrops, groupDrops), nil
}

func buildProxyCascade(proxies []model.Proxy, droppedTypes []string, opts cascadeOptions) (*proxyCascade, error) {
	result := &proxyCascade{
		dropped: rootDroppedProxies(proxies, droppedTypes),
		reasons: make(map[string]proxyDropReason),
	}
	for name := range result.dropped {
		result.reasons[name] = proxyDropReason{Kind: proxyDropReasonProtoRoot}
	}

	if err := cascadeDroppedDialers(proxies, result, opts); err != nil {
		return nil, err
	}
	return result, nil
}

func rootDroppedProxies(proxies []model.Proxy, droppedTypes []string) map[string]struct{} {
	typeSet := make(map[string]struct{}, len(droppedTypes))
	for _, t := range droppedTypes {
		typeSet[t] = struct{}{}
	}

	dropped := make(map[string]struct{})
	for _, px := range proxies {
		if _, hit := typeSet[px.Type]; hit {
			dropped[px.Name] = struct{}{}
		}
	}
	return dropped
}

func cascadeDroppedDialers(proxies []model.Proxy, drops *proxyCascade, opts cascadeOptions) error {
	maxChainIter := len(proxies) + 1
	for i := 0; ; i++ {
		if i >= maxChainIter {
			return internalFilterError(opts, "链式剔除未收敛（超过 %d 轮），疑似循环 dialer 引用", maxChainIter)
		}
		if !dropDialerDependents(proxies, drops) {
			return nil
		}
	}
}

func dropDialerDependents(proxies []model.Proxy, drops *proxyCascade) bool {
	grew := false
	for _, px := range proxies {
		if px.Dialer == "" {
			continue
		}
		if _, isDropped := drops.dropped[px.Name]; isDropped {
			continue
		}
		if _, upstreamDropped := drops.dropped[px.Dialer]; upstreamDropped {
			drops.dropped[px.Name] = struct{}{}
			drops.reasons[px.Name] = proxyDropReason{
				Kind:   proxyDropReasonDialerCascade,
				Parent: px.Dialer,
			}
			grew = true
		}
	}
	return grew
}

func buildGroupCascade(nodeGroups, routeGroups []model.ProxyGroup, droppedProxies map[string]struct{}, opts cascadeOptions) (*groupCascade, error) {
	result := &groupCascade{
		nodeGroups:  append([]model.ProxyGroup(nil), nodeGroups...),
		routeGroups: append([]model.ProxyGroup(nil), routeGroups...),
		dropped:     make(map[string]struct{}),
		reasons:     make(map[string][]string),
	}

	maxGroupIter := len(result.nodeGroups) + len(result.routeGroups) + 1
	for i := 0; ; i++ {
		if i >= maxGroupIter {
			return nil, internalFilterError(opts, "组级联剔除未收敛（超过 %d 轮），疑似相互引用", maxGroupIter)
		}
		if !dropEmptyGroups(result.nodeGroups, result.routeGroups, droppedProxies, result.dropped, result.reasons) {
			return result, nil
		}
	}
}

func dropEmptyGroups(nodeGroups, routeGroups []model.ProxyGroup, droppedProxies, droppedGroups map[string]struct{}, groupReasons map[string][]string) bool {
	grew := pruneGroupSlice(nodeGroups, droppedProxies, droppedGroups, groupReasons)
	if pruneGroupSlice(routeGroups, droppedProxies, droppedGroups, groupReasons) {
		grew = true
	}
	return grew
}

// 原地修改 groups[i].Members。调用方必须先拷贝 slice——
// buildGroupCascade 在入口处对 nodeGroups / routeGroups 各自做了 append-copy，
// 所以外部 *model.Pipeline 的 ProxyGroup.Members 不会被触及。
func pruneGroupSlice(groups []model.ProxyGroup, droppedProxies, droppedGroups map[string]struct{}, groupReasons map[string][]string) bool {
	grew := false
	for i, g := range groups {
		if _, alreadyDropped := droppedGroups[g.Name]; alreadyDropped {
			continue
		}
		prevMembers := groups[i].Members
		pruned := pruneMembers(prevMembers, droppedProxies, droppedGroups)
		groups[i].Members = pruned
		if len(pruned) == 0 {
			droppedGroups[g.Name] = struct{}{}
			groupReasons[g.Name] = append([]string(nil), prevMembers...)
			grew = true
		}
	}
	return grew
}

func fallbackEmptyError(fallback string, groupReasons map[string][]string, proxyReasons map[string]proxyDropReason, opts cascadeOptions) error {
	path := buildDropPath(fallback, groupReasons, proxyReasons, opts.rootLabel, make(map[string]bool))
	return &errtype.TargetError{
		Code:    opts.emptyCode,
		Format:  opts.formatName,
		Message: fmt.Sprintf("fallback 服务组 %q 在 %s 输出中成员为空（%s）。清空路径：%s", fallback, opts.formatDisplayName, opts.emptyReasonClause, path),
	}
}

func projectPipeline(p *model.Pipeline, proxyDrops *proxyCascade, groupDrops *groupCascade) *model.Pipeline {
	nodeGroupsOut := make([]model.ProxyGroup, 0, len(groupDrops.nodeGroups))
	for _, g := range groupDrops.nodeGroups {
		if _, drop := groupDrops.dropped[g.Name]; drop {
			continue
		}
		nodeGroupsOut = append(nodeGroupsOut, g)
	}
	routeGroupsOut := make([]model.ProxyGroup, 0, len(groupDrops.routeGroups))
	for _, g := range groupDrops.routeGroups {
		if _, drop := groupDrops.dropped[g.Name]; drop {
			continue
		}
		routeGroupsOut = append(routeGroupsOut, g)
	}

	rulesetsOut := make([]model.Ruleset, 0, len(p.Rulesets))
	for _, rs := range p.Rulesets {
		if _, drop := groupDrops.dropped[rs.Policy]; drop {
			continue
		}
		rulesetsOut = append(rulesetsOut, rs)
	}

	rulesOut := make([]model.Rule, 0, len(p.Rules))
	for _, r := range p.Rules {
		if _, drop := groupDrops.dropped[r.Policy]; drop {
			continue
		}
		rulesOut = append(rulesOut, r)
	}

	proxiesOut := make([]model.Proxy, 0, len(p.Proxies))
	for _, px := range p.Proxies {
		if _, drop := proxyDrops.dropped[px.Name]; drop {
			continue
		}
		proxiesOut = append(proxiesOut, px)
	}

	allOut := make([]string, 0, len(p.AllProxies))
	for _, name := range p.AllProxies {
		if _, drop := proxyDrops.dropped[name]; drop {
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
	}
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
	return &errtype.TargetError{
		Code:    opts.internalCode,
		Format:  opts.formatName,
		Message: "内部不变量异常：" + fmt.Sprintf(format, args...),
	}
}
