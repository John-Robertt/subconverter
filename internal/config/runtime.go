package config

import (
	"fmt"
	"regexp"

	"github.com/John-Robertt/subconverter/internal/proxyparse"
)

// RuntimeConfig is the startup-prepared configuration consumed by request-time
// pipeline stages. It is treated as read-only after Prepare; request-time code
// should derive new dynamic results instead of mutating prepared inputs.
type RuntimeConfig struct {
	sources         PreparedSources
	filters         PreparedFilters
	groups          []PreparedGroup
	routing         []PreparedRouteGroup
	rulesets        []PreparedRuleset
	rules           []PreparedRule
	fallback        string
	baseURL         string
	templates       Templates
	staticNamespace StaticNamespace
}

// PreparedSources is the startup-prepared subset consumed by the Source stage.
type PreparedSources struct {
	Subscriptions []Subscription
	Snell         []SnellSource
	VLess         []VLessSource
	CustomProxies []PreparedCustomProxy
	FetchOrder    []string
}

// PreparedFilters stores startup-compiled filter regexes.
type PreparedFilters struct {
	RawExclude     string
	ExcludePattern *regexp.Regexp
}

// PreparedGroup stores a compiled region-group definition.
type PreparedGroup struct {
	Name     string
	Strategy string
	RawMatch string
	Match    *regexp.Regexp
}

// RouteMemberOrigin records how a route-group member entered the runtime graph.
type RouteMemberOrigin string

const (
	RouteMemberOriginLiteral      RouteMemberOrigin = "literal"
	RouteMemberOriginAutoExpanded RouteMemberOrigin = "auto_expanded"
	RouteMemberOriginAllExpanded  RouteMemberOrigin = "all_expanded"
)

// PreparedRouteMember stores one route-group member token plus its origin.
type PreparedRouteMember struct {
	Raw    string
	Origin RouteMemberOrigin
}

// PreparedRouteGroup stores both user-declared and startup-expanded route members.
// @auto has already been expanded in ExpandedMembers; @all may remain for Route.
type PreparedRouteGroup struct {
	Name            string
	DeclaredMembers []PreparedRouteMember
	ExpandedMembers []PreparedRouteMember
}

// PreparedRuleset stores a validated ruleset binding.
type PreparedRuleset struct {
	Policy string
	URLs   []string
}

// PreparedRule stores a startup-parsed inline rule.
type PreparedRule struct {
	Raw    string
	Policy string
}

// PreparedCustomProxy stores a startup-parsed custom proxy declaration.
type PreparedCustomProxy struct {
	Name         string
	Parsed       proxyparse.Result
	RelayThrough *PreparedRelayThrough
}

// PreparedRelayThrough stores a startup-validated relay_through declaration.
type PreparedRelayThrough struct {
	Type     string
	Strategy string
	Name     string
	RawMatch string
	Match    *regexp.Regexp
}

// StaticNamespace is the startup-known shared naming space used to reject
// fetched node names that collide with local static objects or reserved names.
type StaticNamespace struct {
	labels map[string]string
}

// Kind returns the static object kind registered for name.
func (ns StaticNamespace) Kind(name string) (string, bool) {
	if ns.labels == nil {
		return "", false
	}
	kind, ok := ns.labels[name]
	return kind, ok
}

// SourceInput returns the Source-stage inputs.
func (rt *RuntimeConfig) SourceInput() PreparedSources {
	if rt == nil {
		return PreparedSources{}
	}
	return rt.sources
}

// FilterInput returns the Filter-stage inputs.
func (rt *RuntimeConfig) FilterInput() PreparedFilters {
	if rt == nil {
		return PreparedFilters{}
	}
	return rt.filters
}

// GroupInput returns the Group-stage inputs.
func (rt *RuntimeConfig) GroupInput() []PreparedGroup {
	if rt == nil {
		return nil
	}
	return rt.groups
}

// RouteInput returns the Route-stage inputs.
func (rt *RuntimeConfig) RouteInput() ([]PreparedRouteGroup, []PreparedRuleset, []PreparedRule, string) {
	if rt == nil {
		return nil, nil, nil, ""
	}
	return rt.routing, rt.rulesets, rt.rules, rt.fallback
}

// StaticNamespace returns the startup-known static namespace.
func (rt *RuntimeConfig) StaticNamespace() StaticNamespace {
	if rt == nil {
		return StaticNamespace{}
	}
	return rt.staticNamespace
}

// BaseURL returns the prepared base_url value.
func (rt *RuntimeConfig) BaseURL() string {
	if rt == nil {
		return ""
	}
	return rt.baseURL
}

// Templates returns a copy of the prepared template configuration.
func (rt *RuntimeConfig) Templates() Templates {
	if rt == nil {
		return Templates{}
	}
	return rt.templates
}

// IsReservedPolicyName reports whether name is a built-in policy name.
func IsReservedPolicyName(name string) bool {
	switch name {
	case "DIRECT", "REJECT":
		return true
	default:
		return false
	}
}

// ClonePreparedRouteMembers returns a shallow copy of a PreparedRouteMember slice.
func ClonePreparedRouteMembers(src []PreparedRouteMember) []PreparedRouteMember {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]PreparedRouteMember, len(src))
	copy(cloned, src)
	return cloned
}

// DetectRouteCycle runs DFS cycle detection on a directed graph defined by adj.
// order determines the traversal sequence for deterministic results.
// Returns an error message on cycle, or empty string if acyclic.
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
