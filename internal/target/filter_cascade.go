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
	Parent model.ProxyID
	Label  string
}

type cascadeOptions struct {
	unsupportedProxy  func(model.Proxy) (string, bool)
	formatName        string
	formatDisplayName string
	nativeFormat      model.NativeProxyFormat
	rootLabel         string
	emptyCode         errtype.Code
	internalCode      errtype.Code
	emptyReasonClause string
}

type proxyCascade struct {
	dropped map[model.ProxyID]struct{}
	reasons map[model.ProxyID]proxyDropReason
}

type droppedMember struct {
	proxyID   model.ProxyID
	groupName string
}

type groupCascade struct {
	nodeGroups  []model.NodeGroup
	routeGroups []model.RouteGroup
	dropped     map[string]struct{}
	reasons     map[string][]droppedMember
}

func filterUnsupportedProxies(p *model.Pipeline, droppedTypes []string, opts cascadeOptions) (*model.Pipeline, error) {
	if p == nil {
		p = &model.Pipeline{}
	}
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
		return nil, fallbackEmptyError(p, groupDrops.reasons, proxyDrops.reasons, opts)
	}

	return projectPipeline(p, proxyDrops, groupDrops), nil
}

func buildProxyCascade(proxies []model.Proxy, droppedTypes []string, opts cascadeOptions) (*proxyCascade, error) {
	result := &proxyCascade{
		dropped: make(map[model.ProxyID]struct{}),
		reasons: make(map[model.ProxyID]proxyDropReason),
	}
	for _, proxy := range proxies {
		if proxy.ID == 0 {
			return nil, internalFilterError(opts, "代理 %q 缺少内部 ID", proxy.Name)
		}
		label, drop := rootProxyDropLabel(proxy, droppedTypes, opts)
		if !drop {
			continue
		}
		result.dropped[proxy.ID] = struct{}{}
		result.reasons[proxy.ID] = proxyDropReason{
			Kind:  proxyDropReasonProtoRoot,
			Label: label,
		}
	}

	if err := cascadeDroppedDialers(proxies, result, opts); err != nil {
		return nil, err
	}
	return result, nil
}

func rootProxyDropLabel(proxy model.Proxy, droppedTypes []string, opts cascadeOptions) (string, bool) {
	if proxy.Native != nil && proxy.Native.Format != opts.nativeFormat {
		return subscriptionFormatDropLabel(proxy.Native.Format), true
	}
	if proxy.Native == nil && opts.unsupportedProxy != nil {
		if label, drop := opts.unsupportedProxy(proxy); drop {
			return label, true
		}
	}
	for _, droppedType := range droppedTypes {
		if proxy.Type == droppedType {
			return opts.rootLabel, true
		}
	}
	return "", false
}

func subscriptionFormatDropLabel(format model.NativeProxyFormat) string {
	switch format {
	case model.NativeProxyFormatClash:
		return "Clash YAML 专属"
	case model.NativeProxyFormatSurge:
		return "Surge CONF 专属"
	case model.NativeProxyFormatLoon:
		return "Loon CONF 专属"
	default:
		return "目标格式不兼容"
	}
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
	for _, proxy := range proxies {
		if proxy.DialerID == 0 {
			continue
		}
		if _, isDropped := drops.dropped[proxy.ID]; isDropped {
			continue
		}
		if _, upstreamDropped := drops.dropped[proxy.DialerID]; upstreamDropped {
			drops.dropped[proxy.ID] = struct{}{}
			drops.reasons[proxy.ID] = proxyDropReason{
				Kind:   proxyDropReasonDialerCascade,
				Parent: proxy.DialerID,
			}
			grew = true
		}
	}
	return grew
}

func buildGroupCascade(
	nodeGroups []model.NodeGroup,
	routeGroups []model.RouteGroup,
	droppedProxies map[model.ProxyID]struct{},
	opts cascadeOptions,
) (*groupCascade, error) {
	result := &groupCascade{
		nodeGroups:  cloneNodeGroups(nodeGroups),
		routeGroups: cloneRouteGroups(routeGroups),
		dropped:     make(map[string]struct{}),
		reasons:     make(map[string][]droppedMember),
	}

	maxGroupIter := len(result.nodeGroups) + len(result.routeGroups) + 1
	for i := 0; ; i++ {
		if i >= maxGroupIter {
			return nil, internalFilterError(opts, "组级联剔除未收敛（超过 %d 轮），疑似相互引用", maxGroupIter)
		}
		if !dropEmptyGroups(result, droppedProxies) {
			return result, nil
		}
	}
}

func dropEmptyGroups(groups *groupCascade, droppedProxies map[model.ProxyID]struct{}) bool {
	grew := pruneNodeGroups(groups, droppedProxies)
	if pruneRouteGroups(groups, droppedProxies) {
		grew = true
	}
	return grew
}

func pruneNodeGroups(groups *groupCascade, droppedProxies map[model.ProxyID]struct{}) bool {
	grew := false
	for index, group := range groups.nodeGroups {
		if _, alreadyDropped := groups.dropped[group.Name]; alreadyDropped {
			continue
		}
		previous := groups.nodeGroups[index].Members
		pruned := make([]model.ProxyID, 0, len(previous))
		for _, id := range previous {
			if _, dropped := droppedProxies[id]; !dropped {
				pruned = append(pruned, id)
			}
		}
		groups.nodeGroups[index].Members = pruned
		if len(pruned) == 0 {
			groups.dropped[group.Name] = struct{}{}
			reasons := make([]droppedMember, 0, len(previous))
			for _, id := range previous {
				reasons = append(reasons, droppedMember{proxyID: id})
			}
			groups.reasons[group.Name] = reasons
			grew = true
		}
	}
	return grew
}

func pruneRouteGroups(groups *groupCascade, droppedProxies map[model.ProxyID]struct{}) bool {
	grew := false
	for index, group := range groups.routeGroups {
		if _, alreadyDropped := groups.dropped[group.Name]; alreadyDropped {
			continue
		}
		previous := groups.routeGroups[index].Members
		pruned := make([]model.PolicyRef, 0, len(previous))
		for _, ref := range previous {
			if !policyRefDropped(ref, droppedProxies, groups.dropped) {
				pruned = append(pruned, ref)
			}
		}
		groups.routeGroups[index].Members = pruned
		if len(pruned) == 0 {
			groups.dropped[group.Name] = struct{}{}
			reasons := make([]droppedMember, 0, len(previous))
			for _, ref := range previous {
				reasons = append(reasons, droppedMemberFromPolicyRef(ref))
			}
			groups.reasons[group.Name] = reasons
			grew = true
		}
	}
	return grew
}

func policyRefDropped(ref model.PolicyRef, droppedProxies map[model.ProxyID]struct{}, droppedGroups map[string]struct{}) bool {
	switch ref.Kind {
	case model.PolicyRefProxy:
		_, dropped := droppedProxies[ref.ProxyID]
		return dropped
	case model.PolicyRefNodeGroup, model.PolicyRefRouteGroup:
		_, dropped := droppedGroups[ref.Name]
		return dropped
	default:
		return false
	}
}

func droppedMemberFromPolicyRef(ref model.PolicyRef) droppedMember {
	if ref.Kind == model.PolicyRefProxy {
		return droppedMember{proxyID: ref.ProxyID}
	}
	return droppedMember{groupName: ref.Name}
}

func fallbackEmptyError(
	p *model.Pipeline,
	groupReasons map[string][]droppedMember,
	proxyReasons map[model.ProxyID]proxyDropReason,
	opts cascadeOptions,
) error {
	proxyNames := make(map[model.ProxyID]string, len(p.Proxies))
	for _, proxy := range p.Proxies {
		proxyNames[proxy.ID] = proxy.Name
	}
	path := buildGroupDropPath(p.Fallback, groupReasons, proxyReasons, proxyNames, opts.rootLabel, make(map[string]bool))
	return &errtype.TargetError{
		Code:    opts.emptyCode,
		Format:  opts.formatName,
		Message: fmt.Sprintf("fallback 服务组 %q 在 %s 输出中成员为空（%s）。清空路径：%s", p.Fallback, opts.formatDisplayName, opts.emptyReasonClause, path),
	}
}

func projectPipeline(p *model.Pipeline, proxyDrops *proxyCascade, groupDrops *groupCascade) *model.Pipeline {
	nodeGroupsOut := make([]model.NodeGroup, 0, len(groupDrops.nodeGroups))
	for _, group := range groupDrops.nodeGroups {
		if _, drop := groupDrops.dropped[group.Name]; !drop {
			nodeGroupsOut = append(nodeGroupsOut, group)
		}
	}
	routeGroupsOut := make([]model.RouteGroup, 0, len(groupDrops.routeGroups))
	for _, group := range groupDrops.routeGroups {
		if _, drop := groupDrops.dropped[group.Name]; !drop {
			routeGroupsOut = append(routeGroupsOut, group)
		}
	}

	rulesetsOut := make([]model.Ruleset, 0, len(p.Rulesets))
	for _, ruleset := range p.Rulesets {
		if _, drop := groupDrops.dropped[ruleset.Policy]; !drop {
			rulesetsOut = append(rulesetsOut, ruleset)
		}
	}

	rulesOut := make([]model.Rule, 0, len(p.Rules))
	for _, rule := range p.Rules {
		if _, drop := groupDrops.dropped[rule.Policy]; !drop {
			rulesOut = append(rulesOut, rule)
		}
	}

	proxiesOut := make([]model.Proxy, 0, len(p.Proxies))
	for _, proxy := range p.Proxies {
		if _, drop := proxyDrops.dropped[proxy.ID]; !drop {
			proxiesOut = append(proxiesOut, proxy)
		}
	}

	allOut := make([]model.ProxyID, 0, len(p.AllProxies))
	for _, id := range p.AllProxies {
		if _, drop := proxyDrops.dropped[id]; !drop {
			allOut = append(allOut, id)
		}
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

func buildGroupDropPath(
	name string,
	groupReasons map[string][]droppedMember,
	proxyReasons map[model.ProxyID]proxyDropReason,
	proxyNames map[model.ProxyID]string,
	rootLabel string,
	visited map[string]bool,
) string {
	if visited[name] {
		return name + "(cycle)"
	}
	members, isGroup := groupReasons[name]
	if !isGroup {
		return name
	}

	visited[name] = true
	parts := make([]string, 0, len(members))
	for _, member := range members {
		if member.proxyID != 0 {
			parts = append(parts, buildProxyDropPath(member.proxyID, proxyReasons, proxyNames, rootLabel, make(map[model.ProxyID]bool)))
			continue
		}
		parts = append(parts, buildGroupDropPath(member.groupName, groupReasons, proxyReasons, proxyNames, rootLabel, visited))
	}
	delete(visited, name)
	return fmt.Sprintf("%s ← [%s]", name, strings.Join(parts, ", "))
}

func buildProxyDropPath(
	id model.ProxyID,
	proxyReasons map[model.ProxyID]proxyDropReason,
	proxyNames map[model.ProxyID]string,
	rootLabel string,
	visited map[model.ProxyID]bool,
) string {
	display := fmt.Sprintf("%s#%d", proxyNames[id], id)
	if visited[id] {
		return display + "(cycle)"
	}
	reason, exists := proxyReasons[id]
	if !exists {
		return display
	}
	switch reason.Kind {
	case proxyDropReasonProtoRoot:
		label := reason.Label
		if label == "" {
			label = rootLabel
		}
		return display + "(" + label + ")"
	case proxyDropReasonDialerCascade:
		visited[id] = true
		parentPath := buildProxyDropPath(reason.Parent, proxyReasons, proxyNames, rootLabel, visited)
		delete(visited, id)
		return fmt.Sprintf("%s(chained) ← [%s]", display, parentPath)
	default:
		return display
	}
}

func cloneNodeGroups(groups []model.NodeGroup) []model.NodeGroup {
	result := make([]model.NodeGroup, len(groups))
	for index, group := range groups {
		result[index] = group
		result[index].Members = append([]model.ProxyID(nil), group.Members...)
	}
	return result
}

func cloneRouteGroups(groups []model.RouteGroup) []model.RouteGroup {
	result := make([]model.RouteGroup, len(groups))
	for index, group := range groups {
		result[index] = group
		result[index].Members = append([]model.PolicyRef(nil), group.Members...)
	}
	return result
}

func internalFilterError(opts cascadeOptions, format string, args ...any) error {
	return &errtype.TargetError{
		Code:    opts.internalCode,
		Format:  opts.formatName,
		Message: "内部不变量异常：" + fmt.Sprintf(format, args...),
	}
}
