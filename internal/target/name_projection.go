package target

import (
	"fmt"

	"github.com/John-Robertt/subconverter/internal/model"
)

func projectTargetPipeline(p *model.Pipeline, opts cascadeOptions) (*model.TargetPipeline, error) {
	if p == nil {
		p = &model.Pipeline{}
	}

	staticNames := targetStaticNames(p)
	outputNames, err := allocateTargetProxyNames(p.Proxies, staticNames, opts)
	if err != nil {
		return nil, err
	}

	proxies, err := buildTargetProxies(p.Proxies, outputNames, opts)
	if err != nil {
		return nil, err
	}
	nodeGroups, err := buildTargetNodeGroups(p.NodeGroups, outputNames, opts)
	if err != nil {
		return nil, err
	}
	routeGroups, err := buildTargetRouteGroups(p.RouteGroups, outputNames, opts)
	if err != nil {
		return nil, err
	}

	return &model.TargetPipeline{
		Proxies:     proxies,
		NodeGroups:  nodeGroups,
		RouteGroups: routeGroups,
		Rulesets:    append([]model.Ruleset(nil), p.Rulesets...),
		Rules:       append([]model.Rule(nil), p.Rules...),
		Fallback:    p.Fallback,
	}, nil
}

func targetStaticNames(p *model.Pipeline) map[string]string {
	names := map[string]string{
		"DIRECT": "保留策略",
		"REJECT": "保留策略",
	}
	for _, group := range p.NodeGroups {
		names[group.Name] = "节点组"
	}
	for _, group := range p.RouteGroups {
		names[group.Name] = "服务组"
	}
	return names
}

func allocateTargetProxyNames(
	proxies []model.Proxy,
	staticNames map[string]string,
	opts cascadeOptions,
) (map[model.ProxyID]string, error) {
	names := make(map[model.ProxyID]string, len(proxies))
	allocator := newTargetNameAllocator(staticNames, opts)

	for _, proxy := range proxies {
		if proxy.Kind == model.KindChained {
			continue
		}
		name, err := allocator.allocate(proxy.ID, proxy.Name)
		if err != nil {
			return nil, err
		}
		names[proxy.ID] = name
	}
	for _, proxy := range proxies {
		if proxy.Kind != model.KindChained {
			continue
		}
		dialerName, exists := names[proxy.DialerID]
		if !exists {
			return nil, internalFilterError(opts, "链式代理 %q 的上游 ID %d 未进入目标输出", proxy.Name, proxy.DialerID)
		}
		baseName := dialerName + "→" + proxy.ChainNameSuffix
		name, err := allocator.allocate(proxy.ID, baseName)
		if err != nil {
			return nil, err
		}
		names[proxy.ID] = name
	}

	return names, nil
}

type targetNameAllocator struct {
	nextSequence map[string]int
	used         map[string]model.ProxyID
	opts         cascadeOptions
}

func newTargetNameAllocator(staticNames map[string]string, opts cascadeOptions) *targetNameAllocator {
	used := make(map[string]model.ProxyID, len(staticNames))
	for name := range staticNames {
		used[name] = 0
	}
	return &targetNameAllocator{
		nextSequence: make(map[string]int),
		used:         used,
		opts:         opts,
	}
}

func (a *targetNameAllocator) allocate(id model.ProxyID, baseName string) (string, error) {
	if id == 0 {
		return "", internalFilterError(a.opts, "目标名称分配遇到空代理 ID")
	}
	if baseName == "" {
		return "", internalFilterError(a.opts, "代理 ID %d 的候选名称为空", id)
	}

	sequence := a.nextSequence[baseName] + 1
	candidate := targetDeduplicatedName(baseName, sequence)
	for {
		if _, exists := a.used[candidate]; !exists {
			break
		}
		sequence++
		candidate = targetDeduplicatedName(baseName, sequence)
	}
	a.nextSequence[baseName] = sequence
	a.used[candidate] = id
	return candidate, nil
}

func targetDeduplicatedName(base string, sequence int) string {
	if sequence <= 1 {
		return base
	}
	circled := []string{"②", "③", "④", "⑤", "⑥", "⑦", "⑧", "⑨", "⑩"}
	if sequence-2 < len(circled) {
		return base + circled[sequence-2]
	}
	return fmt.Sprintf("%s(%d)", base, sequence)
}

func buildTargetProxies(
	proxies []model.Proxy,
	outputNames map[model.ProxyID]string,
	opts cascadeOptions,
) ([]model.TargetProxy, error) {
	result := make([]model.TargetProxy, 0, len(proxies))
	for _, proxy := range proxies {
		name, exists := outputNames[proxy.ID]
		if !exists {
			return nil, internalFilterError(opts, "代理 ID %d 缺少目标输出名", proxy.ID)
		}
		dialerName := ""
		if proxy.DialerID != 0 {
			dialerName, exists = outputNames[proxy.DialerID]
			if !exists {
				return nil, internalFilterError(opts, "代理 ID %d 的上游 ID %d 缺少目标输出名", proxy.ID, proxy.DialerID)
			}
		}
		result = append(result, model.TargetProxy{
			Name:   name,
			Type:   proxy.Type,
			Server: proxy.Server,
			Port:   proxy.Port,
			Params: proxy.Params,
			Plugin: proxy.Plugin,
			Kind:   proxy.Kind,
			Dialer: dialerName,
			Native: proxy.Native,
		})
	}
	return result, nil
}

func buildTargetNodeGroups(
	groups []model.NodeGroup,
	outputNames map[model.ProxyID]string,
	opts cascadeOptions,
) ([]model.TargetGroup, error) {
	result := make([]model.TargetGroup, 0, len(groups))
	for _, group := range groups {
		members := make([]string, 0, len(group.Members))
		for _, id := range group.Members {
			name, exists := outputNames[id]
			if !exists {
				return nil, internalFilterError(opts, "节点组 %q 的成员 ID %d 缺少目标输出名", group.Name, id)
			}
			members = append(members, name)
		}
		result = append(result, model.TargetGroup{
			Name:     group.Name,
			Strategy: group.Strategy,
			Members:  members,
		})
	}
	return result, nil
}

func buildTargetRouteGroups(
	groups []model.RouteGroup,
	outputNames map[model.ProxyID]string,
	opts cascadeOptions,
) ([]model.TargetGroup, error) {
	result := make([]model.TargetGroup, 0, len(groups))
	for _, group := range groups {
		members := make([]string, 0, len(group.Members))
		for _, ref := range group.Members {
			name, err := targetPolicyRefName(ref, outputNames, opts)
			if err != nil {
				return nil, err
			}
			members = append(members, name)
		}
		result = append(result, model.TargetGroup{
			Name:     group.Name,
			Strategy: group.Strategy,
			Members:  members,
		})
	}
	return result, nil
}

func targetPolicyRefName(
	ref model.PolicyRef,
	outputNames map[model.ProxyID]string,
	opts cascadeOptions,
) (string, error) {
	switch ref.Kind {
	case model.PolicyRefProxy:
		name, exists := outputNames[ref.ProxyID]
		if !exists {
			return "", internalFilterError(opts, "服务组成员代理 ID %d 缺少目标输出名", ref.ProxyID)
		}
		return name, nil
	case model.PolicyRefNodeGroup, model.PolicyRefRouteGroup, model.PolicyRefReserved:
		if ref.Name == "" {
			return "", internalFilterError(opts, "服务组成员 %q 缺少名称", ref.Kind)
		}
		return ref.Name, nil
	default:
		return "", internalFilterError(opts, "服务组成员引用类型 %q 无效", ref.Kind)
	}
}
