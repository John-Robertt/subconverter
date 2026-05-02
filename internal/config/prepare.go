package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
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

// customProxyResult 是 prepareCustomProxies 的跨阶段产物：
// 后续 prepareRouting 需要区分链式组名与独立自定义代理名。
type customProxyResult struct {
	chainGroupNames       []string
	standaloneCustomNames map[string]struct{}
}

// routingScope 是 prepareRouting 的跨阶段产物：
// 后续 prepareRulesets / prepareRules / prepareFallback 需要它做策略名存在性校验。
type routingScope struct {
	routeNameSet map[string]bool
}

// Prepare validates the raw YAML config and produces the startup-prepared
// configuration consumed by the request-time pipeline.
//
// Stage ordering is enforced by function signatures: each stage that depends
// on a prior stage's output takes that output as an explicit parameter.
// Swapping stages in the main body causes a compile error rather than a
// silent nil-map read.
func Prepare(cfg *Config) (*RuntimeConfig, error) {
	if cfg == nil {
		var c collector
		c.add(sectionPath(""), "配置不能为空")
		return nil, c.result()
	}

	var c collector
	rt := newEmptyRuntimeConfig(cfg)
	registry := newRegistry()

	prepareSourceURLs(&c, cfg)
	prepareFilters(&c, rt, cfg)
	groupNames := prepareGroups(&c, rt, registry, cfg)
	cp := prepareCustomProxies(&c, rt, registry, cfg)
	scope := prepareRouting(&c, rt, registry, cfg, groupNames, cp)
	prepareRulesets(&c, rt, cfg, scope)
	prepareRules(&c, rt, cfg, scope)
	prepareFallback(&c, cfg, scope)
	prepareBaseURL(&c, cfg)
	prepareTemplates(&c, cfg)

	if err := c.result(); err != nil {
		return nil, err
	}

	rt.staticNamespace = newStaticNamespace(registry)
	return rt, nil
}

func newEmptyRuntimeConfig(cfg *Config) *RuntimeConfig {
	return &RuntimeConfig{
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
}

func newRegistry() map[string]staticDecl {
	return map[string]staticDecl{
		"DIRECT": {kind: staticKindReserved},
		"REJECT": {kind: staticKindReserved},
	}
}

func prepareSourceURLs(c *collector, cfg *Config) {
	for i, sub := range cfg.Sources.Subscriptions {
		loc := valuePath("sources", fmt.Sprintf("subscriptions[%d].url", i))
		if sub.URL == "" {
			c.addCode(loc, errtype.CodeConfigRequired, "必填")
			continue
		}
		c.validateHTTPURL(loc, sub.URL)
	}

	for i, s := range cfg.Sources.Snell {
		loc := valuePath("sources", fmt.Sprintf("snell[%d].url", i))
		if s.URL == "" {
			c.addCode(loc, errtype.CodeConfigRequired, "必填")
			continue
		}
		c.validateHTTPURL(loc, s.URL)
	}

	for i, s := range cfg.Sources.VLess {
		loc := valuePath("sources", fmt.Sprintf("vless[%d].url", i))
		if s.URL == "" {
			c.addCode(loc, errtype.CodeConfigRequired, "必填")
			continue
		}
		c.validateHTTPURL(loc, s.URL)
	}
}

func prepareFilters(c *collector, rt *RuntimeConfig, cfg *Config) {
	if cfg.Filters.Exclude == "" {
		return
	}
	re, err := regexp.Compile(cfg.Filters.Exclude)
	if err != nil {
		c.addCode(valuePath("filters", "exclude"), errtype.CodeConfigInvalidRegex, fmt.Sprintf("正则表达式无效：%v", err))
		return
	}
	rt.filters = PreparedFilters{
		RawExclude:     cfg.Filters.Exclude,
		ExcludePattern: re,
	}
}

func prepareGroups(c *collector, rt *RuntimeConfig, registry map[string]staticDecl, cfg *Config) []string {
	groupNames := make([]string, 0, cfg.Groups.Len())
	index := 0
	for name, g := range cfg.Groups.Entries() {
		registerStaticName(c, registry, keyedIndexedPath("groups", index, name, "key"), name, staticKindNodeGroup)
		groupNames = append(groupNames, name)

		prepared := PreparedGroup{Name: name, Strategy: g.Strategy, RawMatch: g.Match}
		if g.Match == "" {
			c.addCode(keyedIndexedPath("groups", index, name, "match"), errtype.CodeConfigRequired, "必填")
		} else {
			re, err := regexp.Compile(g.Match)
			if err != nil {
				c.addCode(keyedIndexedPath("groups", index, name, "match"), errtype.CodeConfigInvalidRegex, fmt.Sprintf("正则表达式无效：%v", err))
			} else {
				prepared.Match = re
			}
		}
		if g.Strategy == "" {
			c.addCode(keyedIndexedPath("groups", index, name, "strategy"), errtype.CodeConfigRequired, "必填")
		} else if g.Strategy != "select" && g.Strategy != "url-test" {
			c.addCode(keyedIndexedPath("groups", index, name, "strategy"), errtype.CodeConfigInvalidEnum, fmt.Sprintf("必须为 select 或 url-test，当前为 %q", g.Strategy))
		}
		rt.groups = append(rt.groups, prepared)
		index++
	}
	return groupNames
}

func prepareCustomProxies(c *collector, rt *RuntimeConfig, registry map[string]staticDecl, cfg *Config) customProxyResult {
	result := customProxyResult{
		chainGroupNames:       make([]string, 0, len(cfg.Sources.CustomProxies)),
		standaloneCustomNames: make(map[string]struct{}),
	}
	seenCustomNames := make(map[string]bool)

	for i, cp := range cfg.Sources.CustomProxies {
		baseLoc := valuePath("sources", fmt.Sprintf("custom_proxies[%d]", i))

		if cp.Name == "" {
			c.addCode(valuePath("sources", fmt.Sprintf("custom_proxies[%d].name", i)), errtype.CodeConfigRequired, "必填")
		} else if seenCustomNames[cp.Name] {
			c.addCode(valuePath("sources", fmt.Sprintf("custom_proxies[%d].name", i)), errtype.CodeConfigDuplicateName, fmt.Sprintf("自定义代理名 %q 重复", cp.Name))
		} else {
			seenCustomNames[cp.Name] = true
			kind := staticKindCustom
			if cp.RelayThrough != nil {
				kind = staticKindChainGroup
				result.chainGroupNames = append(result.chainGroupNames, cp.Name)
			} else {
				result.standaloneCustomNames[cp.Name] = struct{}{}
			}
			registerStaticName(c, registry, valuePath("sources", fmt.Sprintf("custom_proxies[%d].name", i)), cp.Name, kind)
		}

		prepared := PreparedCustomProxy{Name: cp.Name}
		if cp.URL == "" {
			c.addCode(valuePath("sources", fmt.Sprintf("custom_proxies[%d].url", i)), errtype.CodeConfigRequired, "必填")
		} else {
			parsed, err := proxyparse.ParseURL(cp.URL)
			if err != nil {
				c.add(valuePath("sources", fmt.Sprintf("custom_proxies[%d].url", i)), err.Error())
			} else {
				if parsed.Type == "ss" {
					if parsed.Params["cipher"] == "" {
						c.add(valuePath("sources", fmt.Sprintf("custom_proxies[%d].url", i)), "SS URI 缺少加密方式（cipher）")
					}
					if parsed.Params["password"] == "" {
						c.add(valuePath("sources", fmt.Sprintf("custom_proxies[%d].url", i)), "SS URI 缺少密码")
					}
				}
				prepared.Parsed = parsed
			}
		}

		if cp.RelayThrough != nil {
			prepared.RelayThrough = prepareRelayThrough(c, cp.RelayThrough, baseLoc, "relay_through")
		}
		rt.sources.CustomProxies = append(rt.sources.CustomProxies, prepared)
	}
	return result
}

func prepareRouting(c *collector, rt *RuntimeConfig, registry map[string]staticDecl, cfg *Config, groupNames []string, cp customProxyResult) routingScope {
	routeGroupNames := make([]string, 0, cfg.Routing.Len())
	routeIndex := 0
	for name := range cfg.Routing.Entries() {
		registerStaticName(c, registry, keyedIndexedPath("routing", routeIndex, name, "key"), name, staticKindRouteGroup)
		routeGroupNames = append(routeGroupNames, name)
		routeIndex++
	}

	routeNameSet := make(map[string]bool, len(routeGroupNames))
	for _, name := range routeGroupNames {
		routeNameSet[name] = true
	}
	nodeGroupNames := make([]string, 0, len(groupNames)+len(cp.chainGroupNames))
	nodeGroupNames = append(nodeGroupNames, groupNames...)
	nodeGroupNames = append(nodeGroupNames, cp.chainGroupNames...)
	nodeGroupNameSet := make(map[string]bool, len(nodeGroupNames))
	for _, name := range nodeGroupNames {
		nodeGroupNameSet[name] = true
	}

	rawRouting := make([]PreparedRouteGroup, 0, cfg.Routing.Len())
	routeIndex = 0
	for name, members := range cfg.Routing.Entries() {
		loc := keyedIndexedPath("routing", routeIndex, name, "")
		if len(members) == 0 {
			c.add(loc, "至少需要一个成员")
			routeIndex++
			continue
		}
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
			if _, ok := cp.standaloneCustomNames[member]; ok {
				c.addCode(loc, errtype.CodeConfigInvalidReference, fmt.Sprintf("成员 %q 必须引用节点组、服务组、DIRECT、REJECT、@all 或 @auto", member))
				continue
			}
			c.addCode(loc, errtype.CodeConfigInvalidReference, fmt.Sprintf("成员 %q 不存在", member))
		}
		if hasAll && autoCount > 0 {
			c.add(loc, "@all 和 @auto 不能同时使用")
		}
		if autoCount > 1 {
			c.add(loc, "@auto 不能重复使用")
		}
		rawRouting = append(rawRouting, PreparedRouteGroup{
			Name:            name,
			DeclaredMembers: declaredMembers,
			ExpandedMembers: ClonePreparedRouteMembers(declaredMembers),
		})
		routeIndex++
	}

	for _, raw := range rawRouting {
		rt.routing = append(rt.routing, PreparedRouteGroup{
			Name:            raw.Name,
			DeclaredMembers: ClonePreparedRouteMembers(raw.DeclaredMembers),
			ExpandedMembers: expandPreparedAutoFill(raw.DeclaredMembers, raw.Name, rawRouting, nodeGroupNames),
		})
	}
	if cycle := detectPreparedRouteCycle(rt.routing); cycle != "" {
		c.add(sectionPath("routing"), cycle)
	}

	return routingScope{routeNameSet: routeNameSet}
}

func prepareRulesets(c *collector, rt *RuntimeConfig, cfg *Config, scope routingScope) {
	rulesetIndex := 0
	for policy, urls := range cfg.Rulesets.Entries() {
		loc := keyedIndexedPath("rulesets", rulesetIndex, policy, "")
		if len(urls) == 0 {
			c.add(loc, "至少需要一个 URL")
		}
		for i, rawURL := range urls {
			urlLoc := keyedIndexedPath("rulesets", rulesetIndex, policy, fmt.Sprintf("[%d]", i))
			if rawURL == "" {
				c.addCode(urlLoc, errtype.CodeConfigRequired, "必填")
				continue
			}
			c.validateHTTPURL(urlLoc, rawURL)
		}
		if !scope.routeNameSet[policy] {
			c.addCode(loc, errtype.CodeConfigInvalidReference, fmt.Sprintf("策略 %q 未在 routing 中定义", policy))
		}
		rt.rulesets = append(rt.rulesets, PreparedRuleset{
			Policy: policy,
			URLs:   append([]string(nil), urls...),
		})
		rulesetIndex++
	}
}

func prepareRules(c *collector, rt *RuntimeConfig, cfg *Config, scope routingScope) {
	for i, raw := range cfg.Rules {
		loc := indexedPath("rules", i, "")
		idx := strings.LastIndex(raw, ",")
		if idx < 0 {
			c.addCode(loc, errtype.CodeConfigInvalidRule, fmt.Sprintf("缺少逗号分隔：%q", raw))
			continue
		}
		policy := raw[idx+1:]
		if !scope.routeNameSet[policy] && !IsReservedPolicyName(policy) {
			c.addCode(loc, errtype.CodeConfigInvalidReference, fmt.Sprintf("规则策略 %q 未在 routing 中定义", policy))
		}
		rt.rules = append(rt.rules, PreparedRule{Raw: raw, Policy: policy})
	}
}

func prepareFallback(c *collector, cfg *Config, scope routingScope) {
	if cfg.Fallback == "" {
		c.addCode(sectionPath("fallback"), errtype.CodeConfigRequired, "必填")
	} else if !scope.routeNameSet[cfg.Fallback] {
		c.addCode(sectionPath("fallback"), errtype.CodeConfigInvalidReference, fmt.Sprintf("%q 未在 routing 中定义", cfg.Fallback))
	}
}

func prepareBaseURL(c *collector, cfg *Config) {
	if cfg.BaseURL != "" {
		c.validateBaseURL(sectionPath("base_url"), cfg.BaseURL)
	}
}

func prepareTemplates(c *collector, cfg *Config) {
	if cfg.Templates.Clash != "" {
		c.validateTemplatePath(valuePath("templates", "clash"), cfg.Templates.Clash)
	}
	if cfg.Templates.Surge != "" {
		c.validateTemplatePath(valuePath("templates", "surge"), cfg.Templates.Surge)
	}
}

func prepareRelayThrough(c *collector, rt *RelayThrough, parent diagnosticPath, prefix string) *PreparedRelayThrough {
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
			c.addCode(childPath(parent, prefix+".name"), errtype.CodeConfigRequired, "type=group 时必填")
		}
	case "select":
		if rt.Match == "" {
			c.addCode(childPath(parent, prefix+".match"), errtype.CodeConfigRequired, "type=select 时必填")
		} else {
			re, err := regexp.Compile(rt.Match)
			if err != nil {
				c.addCode(childPath(parent, prefix+".match"), errtype.CodeConfigInvalidRegex, fmt.Sprintf("正则表达式无效：%v", err))
			} else {
				prepared.Match = re
			}
		}
	case "all":
		// no-op
	case "":
		c.addCode(childPath(parent, prefix+".type"), errtype.CodeConfigRequired, "必填")
	default:
		c.addCode(childPath(parent, prefix+".type"), errtype.CodeConfigInvalidEnum, fmt.Sprintf("必须为 group、select 或 all，当前为 %q", rt.Type))
	}

	if rt.Strategy == "" {
		c.addCode(childPath(parent, prefix+".strategy"), errtype.CodeConfigRequired, "必填")
	} else if rt.Strategy != "select" && rt.Strategy != "url-test" {
		c.addCode(childPath(parent, prefix+".strategy"), errtype.CodeConfigInvalidEnum, fmt.Sprintf("必须为 select 或 url-test，当前为 %q", rt.Strategy))
	}

	return prepared
}

func childPath(parent diagnosticPath, suffix string) diagnosticPath {
	if parent.valuePath == "" {
		parent.valuePath = suffix
	} else {
		parent.valuePath += "." + suffix
	}
	return parent
}

func registerStaticName(c *collector, registry map[string]staticDecl, loc diagnosticPath, name, kind string) {
	if name == "" {
		return
	}
	if other, ok := registry[name]; ok {
		c.addCode(loc, errtype.CodeConfigDuplicateName, fmt.Sprintf("名称 %q 同时被 %s 和 %s 使用", name, other.kind, kind))
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
