package config

import (
	"fmt"
	"maps"
	"regexp"

	"github.com/John-Robertt/subconverter/internal/proxyparse"
)

// RuntimeConfig 是启动时预处理好的配置，供请求阶段的流水线各站使用。
// Prepare 之后视为只读；请求阶段的代码应派生新的动态结果，而不是修改预处理好的输入。
type RuntimeConfig struct {
	sources   PreparedSources
	filters   PreparedFilters
	groups    []PreparedGroup
	routing   []PreparedRouteGroup
	rulesets  []PreparedRuleset
	rules     []PreparedRule
	fallback  string
	baseURL   string
	templates Templates
}

// PreparedSources 是启动时预处理好的子集，供 Source 阶段使用。
type PreparedSources struct {
	Subscriptions []Source
	Snell         []Source
	VLess         []Source
	CustomProxies []PreparedCustomProxy
	FetchOrder    []string
}

// PreparedFilters 存放启动时编译好的过滤正则。
type PreparedFilters struct {
	RawExclude     string
	ExcludePattern *regexp.Regexp
}

// PreparedGroup 存放编译好的区域组定义。
type PreparedGroup struct {
	Name     string
	Strategy string
	RawMatch string
	Match    *regexp.Regexp
}

// RouteMemberOrigin 记录服务组成员是如何进入运行时关系图的。
type RouteMemberOrigin string

const (
	RouteMemberOriginLiteral      RouteMemberOrigin = "literal"
	RouteMemberOriginAutoExpanded RouteMemberOrigin = "auto_expanded"
	RouteMemberOriginAllExpanded  RouteMemberOrigin = "all_expanded"
)

// PreparedRouteMember 存放一个服务组成员标记及其来源。
type PreparedRouteMember struct {
	Raw    string
	Origin RouteMemberOrigin
}

// PreparedRouteGroup 同时存放用户声明的成员和启动时展开后的成员。
// ExpandedMembers 中的 @auto 已经展开；@all 可能保留，留给 Route 阶段处理。
type PreparedRouteGroup struct {
	Name            string
	DeclaredMembers []PreparedRouteMember
	ExpandedMembers []PreparedRouteMember
}

// PreparedRuleset 存放一条校验过的规则集绑定。
type PreparedRuleset struct {
	Policy string
	URLs   []string
}

// PreparedRule 存放一条启动时解析好的内联规则。
type PreparedRule struct {
	Raw    string
	Policy string
}

// PreparedCustomProxy 存放一条启动时解析好的自定义代理声明。
type PreparedCustomProxy struct {
	Name         string
	Parsed       proxyparse.Result
	RelayThrough *PreparedRelayThrough
}

// PreparedRelayThrough 存放一条启动时校验过的 relay_through 声明。
type PreparedRelayThrough struct {
	Type     string
	Strategy string
	Name     string
	RawMatch string
	Match    *regexp.Regexp
}

// SourceInput 返回 Source 阶段的输入。
func (rt *RuntimeConfig) SourceInput() PreparedSources {
	if rt == nil {
		return PreparedSources{}
	}
	return rt.sources
}

// FilterInput 返回 Filter 阶段的输入。
func (rt *RuntimeConfig) FilterInput() PreparedFilters {
	if rt == nil {
		return PreparedFilters{}
	}
	return rt.filters
}

// GroupInput 返回 Group 阶段的输入。
func (rt *RuntimeConfig) GroupInput() []PreparedGroup {
	if rt == nil {
		return nil
	}
	return rt.groups
}

// RouteInput 返回 Route 阶段的输入。
func (rt *RuntimeConfig) RouteInput() ([]PreparedRouteGroup, []PreparedRuleset, []PreparedRule, string) {
	if rt == nil {
		return nil, nil, nil, ""
	}
	return rt.routing, rt.rulesets, rt.rules, rt.fallback
}

// BaseURL 返回预处理好的 base_url 值。
func (rt *RuntimeConfig) BaseURL() string {
	if rt == nil {
		return ""
	}
	return rt.baseURL
}

// Templates 返回预处理好的模板配置的副本。
func (rt *RuntimeConfig) Templates() Templates {
	if rt == nil {
		return Templates{}
	}
	return cloneTemplates(rt.templates)
}

// IsReservedPolicyName 判断 name 是否为内置策略名。
func IsReservedPolicyName(name string) bool {
	switch name {
	case "DIRECT", "REJECT":
		return true
	default:
		return false
	}
}

// ClonePreparedRouteMembers 返回 PreparedRouteMember 切片的浅拷贝。
func ClonePreparedRouteMembers(src []PreparedRouteMember) []PreparedRouteMember {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]PreparedRouteMember, len(src))
	copy(cloned, src)
	return cloned
}

// DetectRouteCycle 在由 adj 定义的有向图上用 DFS 检测环。
// order 决定遍历顺序，使结果确定。
// 存在环时返回错误信息，无环时返回空字符串。
func DetectRouteCycle(adj map[string][]string, order []string) string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(order))

	var dfs func(string) string
	dfs = func(node string) string {
		color[node] = gray
		for _, next := range adj[node] {
			switch color[next] {
			case gray:
				return fmt.Sprintf("服务组存在循环引用：%s -> %s", node, next)
			case white:
				if msg := dfs(next); msg != "" {
					return msg
				}
			}
		}
		color[node] = black
		return ""
	}

	for _, node := range order {
		if color[node] != white {
			continue
		}
		if msg := dfs(node); msg != "" {
			return msg
		}
	}
	return ""
}

func cloneTemplates(t Templates) Templates {
	t.Inline = maps.Clone(t.Inline)
	return t
}
