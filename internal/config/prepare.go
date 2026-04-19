package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/John-Robertt/subconverter/internal/proxyparse"
)

const (
	staticKindReserved   = "保留策略"
	staticKindNodeGroup  = "节点组"
	staticKindRouteGroup = "服务组"
	staticKindCustom     = "自定义代理"
	staticKindChainGroup = "链式组"
)

type staticDecl struct {
	kind string
}

// Prepare validates the raw YAML config and produces the immutable runtime
// configuration consumed by the request-time pipeline.
func Prepare(cfg *Config) (*RuntimeConfig, error) {
	var c collector
	if cfg == nil {
		c.add("", "配置不能为空")
		return nil, c.result()
	}

	rt := &RuntimeConfig{
		sources: PreparedSources{
			Subscriptions: append([]Subscription(nil), cfg.Sources.Subscriptions...),
			Snell:         append([]SnellSource(nil), cfg.Sources.Snell...),
			VLess:         append([]VLessSource(nil), cfg.Sources.VLess...),
			FetchOrder:    append([]string(nil), cfg.Sources.FetchOrder...),
		},
		fallback:  cfg.Fallback,
		baseURL:   cfg.BaseURL,
		templates: cfg.Templates,
	}

	// sources.subscriptions
	for i, sub := range cfg.Sources.Subscriptions {
		field := fmt.Sprintf("sources.subscriptions[%d].url", i)
		if sub.URL == "" {
			c.add(field, "必填")
			continue
		}
		c.validateHTTPURL(field, sub.URL)
	}

	// sources.snell
	for i, s := range cfg.Sources.Snell {
		field := fmt.Sprintf("sources.snell[%d].url", i)
		if s.URL == "" {
			c.add(field, "必填")
			continue
		}
		c.validateHTTPURL(field, s.URL)
	}

	// sources.vless
	for i, s := range cfg.Sources.VLess {
		field := fmt.Sprintf("sources.vless[%d].url", i)
		if s.URL == "" {
			c.add(field, "必填")
			continue
		}
		c.validateHTTPURL(field, s.URL)
	}

	// filters
	if cfg.Filters.Exclude != "" {
		re, err := regexp.Compile(cfg.Filters.Exclude)
		if err != nil {
			c.add("filters.exclude", fmt.Sprintf("正则表达式无效：%v", err))
		} else {
			rt.filters = PreparedFilters{
				RawExclude:     cfg.Filters.Exclude,
				ExcludePattern: re,
			}
		}
	}

	registry := map[string]staticDecl{
		"DIRECT": {kind: staticKindReserved},
		"REJECT": {kind: staticKindReserved},
	}
	groupNames := make([]string, 0, cfg.Groups.Len())

	// groups
	for name, g := range cfg.Groups.Entries() {
		field := fmt.Sprintf("groups.%s", name)
		registerStaticName(&c, registry, field, name, staticKindNodeGroup)
		groupNames = append(groupNames, name)

		prepared := PreparedGroup{Name: name, Strategy: g.Strategy, RawMatch: g.Match}
		if g.Match == "" {
			c.add(field+".match", "必填")
		} else {
			re, err := regexp.Compile(g.Match)
			if err != nil {
				c.add(field+".match", fmt.Sprintf("正则表达式无效：%v", err))
			} else {
				prepared.Match = re
			}
		}
		if g.Strategy == "" {
			c.add(field+".strategy", "必填")
		} else if g.Strategy != "select" && g.Strategy != "url-test" {
			c.add(field+".strategy", fmt.Sprintf("必须为 select 或 url-test，当前为 %q", g.Strategy))
		}
		rt.groups = append(rt.groups, prepared)
	}

	chainGroupNames := make([]string, 0, len(cfg.Sources.CustomProxies))
	standaloneCustomNames := make(map[string]struct{})
	seenCustomNames := make(map[string]bool)

	// sources.custom_proxies
	for i, cp := range cfg.Sources.CustomProxies {
		prefix := fmt.Sprintf("sources.custom_proxies[%d]", i)

		if cp.Name == "" {
			c.add(prefix+".name", "必填")
		} else if seenCustomNames[cp.Name] {
			c.add(prefix+".name", fmt.Sprintf("自定义代理名 %q 重复", cp.Name))
		} else {
			seenCustomNames[cp.Name] = true
			kind := staticKindCustom
			if cp.RelayThrough != nil {
				kind = staticKindChainGroup
				chainGroupNames = append(chainGroupNames, cp.Name)
			} else {
				standaloneCustomNames[cp.Name] = struct{}{}
			}
			registerStaticName(&c, registry, prefix+".name", cp.Name, kind)
		}

		prepared := PreparedCustomProxy{Name: cp.Name}
		if cp.URL == "" {
			c.add(prefix+".url", "必填")
		} else {
			parsed, err := proxyparse.ParseURL(cp.URL)
			if err != nil {
				c.add(prefix+".url", err.Error())
			} else {
				if parsed.Type == "ss" {
					if parsed.Params["cipher"] == "" {
						c.add(prefix+".url", "SS URI 缺少加密方式（cipher）")
					}
					if parsed.Params["password"] == "" {
						c.add(prefix+".url", "SS URI 缺少密码")
					}
				}
				prepared.Parsed = parsed
			}
		}

		if cp.RelayThrough != nil {
			prepared.RelayThrough = prepareRelayThrough(&c, cp.RelayThrough, prefix+".relay_through")
		}
		rt.sources.CustomProxies = append(rt.sources.CustomProxies, prepared)
	}

	routeGroupNames := make([]string, 0, cfg.Routing.Len())
	rawRouting := make([]PreparedRouteGroup, 0, cfg.Routing.Len())

	// routing keys
	for name := range cfg.Routing.Entries() {
		field := fmt.Sprintf("routing.%s", name)
		registerStaticName(&c, registry, field, name, staticKindRouteGroup)
		routeGroupNames = append(routeGroupNames, name)
	}

	routeNameSet := make(map[string]bool, len(routeGroupNames))
	for _, name := range routeGroupNames {
		routeNameSet[name] = true
	}
	nodeGroupNames := make([]string, 0, len(groupNames)+len(chainGroupNames))
	nodeGroupNames = append(nodeGroupNames, groupNames...)
	nodeGroupNames = append(nodeGroupNames, chainGroupNames...)
	nodeGroupNameSet := make(map[string]bool, len(nodeGroupNames))
	for _, name := range nodeGroupNames {
		nodeGroupNameSet[name] = true
	}

	// routing members
	for name, members := range cfg.Routing.Entries() {
		field := fmt.Sprintf("routing.%s", name)
		hasAll, autoCount := false, 0
		declaredMembers := make([]PreparedRouteMember, 0, len(members))
		for _, member := range members {
			declaredMembers = append(declaredMembers, newLiteralRouteMember(member))
			if member == "@all" {
				hasAll = true
			}
			if member == "@auto" {
				autoCount++
			}
			if member == "@all" || member == "@auto" || IsReservedPolicyName(member) || nodeGroupNameSet[member] || routeNameSet[member] {
				continue
			}
			if _, ok := standaloneCustomNames[member]; ok {
				c.add(field, fmt.Sprintf("成员 %q 必须引用节点组、服务组、DIRECT、REJECT、@all 或 @auto", member))
				continue
			}
			c.add(field, fmt.Sprintf("成员 %q 不存在", member))
		}
		if hasAll && autoCount > 0 {
			c.add(field, "@all 和 @auto 不能同时使用")
		}
		if autoCount > 1 {
			c.add(field, "@auto 不能重复使用")
		}
		rawRouting = append(rawRouting, PreparedRouteGroup{
			Name:            name,
			DeclaredMembers: declaredMembers,
			ExpandedMembers: ClonePreparedRouteMembers(declaredMembers),
		})
	}

	for _, raw := range rawRouting {
		rt.routing = append(rt.routing, PreparedRouteGroup{
			Name:            raw.Name,
			DeclaredMembers: ClonePreparedRouteMembers(raw.DeclaredMembers),
			ExpandedMembers: expandPreparedAutoFill(raw.DeclaredMembers, raw.Name, rawRouting, nodeGroupNames),
		})
	}
	if cycle := detectPreparedRouteCycle(rt.routing); cycle != "" {
		c.add("routing", cycle)
	}

	// rulesets
	for policy, urls := range cfg.Rulesets.Entries() {
		field := fmt.Sprintf("rulesets.%s", policy)
		if len(urls) == 0 {
			c.add(field, "至少需要一个 URL")
		}
		for i, rawURL := range urls {
			urlField := fmt.Sprintf("%s[%d]", field, i)
			if rawURL == "" {
				c.add(urlField, "必填")
				continue
			}
			c.validateHTTPURL(urlField, rawURL)
		}
		if !routeNameSet[policy] {
			c.add(field, fmt.Sprintf("策略 %q 未在 routing 中定义", policy))
		}
		rt.rulesets = append(rt.rulesets, PreparedRuleset{
			Policy: policy,
			URLs:   append([]string(nil), urls...),
		})
	}

	// rules
	for i, raw := range cfg.Rules {
		field := fmt.Sprintf("rules[%d]", i)
		idx := strings.LastIndex(raw, ",")
		if idx < 0 {
			c.add(field, fmt.Sprintf("缺少逗号分隔：%q", raw))
			continue
		}
		policy := raw[idx+1:]
		if !routeNameSet[policy] && !IsReservedPolicyName(policy) {
			c.add(field, fmt.Sprintf("规则策略 %q 未在 routing 中定义", policy))
		}
		rt.rules = append(rt.rules, PreparedRule{Raw: raw, Policy: policy})
	}

	// fallback
	if cfg.Fallback == "" {
		c.add("fallback", "必填")
	} else if !routeNameSet[cfg.Fallback] {
		c.add("fallback", fmt.Sprintf("%q 未在 routing 中定义", cfg.Fallback))
	}

	// base_url
	if cfg.BaseURL != "" {
		c.validateBaseURL("base_url", cfg.BaseURL)
	}

	// templates
	if cfg.Templates.Clash != "" {
		c.validateTemplatePath("templates.clash", cfg.Templates.Clash)
	}
	if cfg.Templates.Surge != "" {
		c.validateTemplatePath("templates.surge", cfg.Templates.Surge)
	}

	if err := c.result(); err != nil {
		return nil, err
	}

	rt.staticNamespace = newStaticNamespace(registry)
	return rt, nil
}

func prepareRelayThrough(c *collector, rt *RelayThrough, prefix string) *PreparedRelayThrough {
	if rt == nil {
		return nil
	}

	prepared := &PreparedRelayThrough{
		Type:     rt.Type,
		Strategy: rt.Strategy,
		Name:     rt.Name,
		RawMatch: rt.Match,
	}

	switch rt.Type {
	case "group":
		if rt.Name == "" {
			c.add(prefix+".name", "type=group 时必填")
		}
	case "select":
		if rt.Match == "" {
			c.add(prefix+".match", "type=select 时必填")
		} else {
			re, err := regexp.Compile(rt.Match)
			if err != nil {
				c.add(prefix+".match", fmt.Sprintf("正则表达式无效：%v", err))
			} else {
				prepared.Match = re
			}
		}
	case "all":
		// no-op
	case "":
		c.add(prefix+".type", "必填")
	default:
		c.add(prefix+".type", fmt.Sprintf("必须为 group、select 或 all，当前为 %q", rt.Type))
	}

	if rt.Strategy == "" {
		c.add(prefix+".strategy", "必填")
	} else if rt.Strategy != "select" && rt.Strategy != "url-test" {
		c.add(prefix+".strategy", fmt.Sprintf("必须为 select 或 url-test，当前为 %q", rt.Strategy))
	}

	return prepared
}

func registerStaticName(c *collector, registry map[string]staticDecl, field, name, kind string) {
	if name == "" {
		return
	}
	if other, ok := registry[name]; ok {
		c.add(field, fmt.Sprintf("名称 %q 同时被 %s 和 %s 使用", name, other.kind, kind))
		return
	}
	registry[name] = staticDecl{kind: kind}
}

func newStaticNamespace(registry map[string]staticDecl) StaticNamespace {
	labels := make(map[string]string, len(registry))
	for name, decl := range registry {
		labels[name] = decl.kind
	}
	return StaticNamespace{labels: labels}
}

func newLiteralRouteMember(raw string) PreparedRouteMember {
	return PreparedRouteMember{Raw: raw, Origin: RouteMemberOriginLiteral}
}

func expandPreparedAutoFill(members []PreparedRouteMember, groupName string, rawRouting []PreparedRouteGroup, nodeGroupNames []string) []PreparedRouteMember {
	if !containsRouteMember(members, "@auto") {
		return ClonePreparedRouteMembers(members)
	}

	seen := make(map[string]struct{}, len(members)+1)
	seen[groupName] = struct{}{}
	for _, member := range members {
		if member.Raw == "@auto" {
			continue
		}
		seen[member.Raw] = struct{}{}
	}

	pool := make([]PreparedRouteMember, 0, len(nodeGroupNames)+len(rawRouting)+1)
	for _, name := range nodeGroupNames {
		if _, ok := seen[name]; ok {
			continue
		}
		pool = append(pool, PreparedRouteMember{Raw: name, Origin: RouteMemberOriginAutoExpanded})
	}
	for _, group := range rawRouting {
		if _, ok := seen[group.Name]; ok {
			continue
		}
		if containsRouteMember(group.DeclaredMembers, "@all") {
			pool = append(pool, PreparedRouteMember{Raw: group.Name, Origin: RouteMemberOriginAutoExpanded})
		}
	}
	if _, ok := seen["DIRECT"]; !ok {
		pool = append(pool, PreparedRouteMember{Raw: "DIRECT", Origin: RouteMemberOriginAutoExpanded})
	}

	result := make([]PreparedRouteMember, 0, len(members)+len(pool))
	for _, member := range members {
		if member.Raw == "@auto" {
			result = append(result, ClonePreparedRouteMembers(pool)...)
			continue
		}
		result = append(result, member)
	}
	return result
}

func detectPreparedRouteCycle(groups []PreparedRouteGroup) string {
	routeNames := make(map[string]bool, len(groups))
	for _, group := range groups {
		routeNames[group.Name] = true
	}

	adj := make(map[string][]string, len(groups))
	order := make([]string, 0, len(groups))
	for _, group := range groups {
		order = append(order, group.Name)
		for _, member := range group.ExpandedMembers {
			if routeNames[member.Raw] {
				adj[group.Name] = append(adj[group.Name], member.Raw)
			}
		}
	}

	return DetectRouteCycle(adj, order)
}

func containsRouteMember(items []PreparedRouteMember, target string) bool {
	for _, item := range items {
		if item.Raw == target {
			return true
		}
	}
	return false
}
