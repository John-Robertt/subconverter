package pipeline

import (
	"fmt"
	"regexp"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// chainedGroupPrefix is prepended to the custom proxy name to form
// the auto-generated chained group name.
// Defined in config-schema.md: "组名为 🔗 <custom_proxy.name>"
const chainedGroupPrefix = "🔗 "

// GroupResult holds the output of the Group stage (pipeline stage 5).
type GroupResult struct {
	Proxies    []model.Proxy      // all proxies: original + chained
	NodeGroups []model.ProxyGroup // region groups + chain groups, declaration order
	AllProxies []string           // @all expansion: original proxy names only
}

// Group executes pipeline stage 5: build region node groups, chained
// nodes/groups, and compute @all.
//
// Three sub-steps in order:
//  1. Build region node groups from groups config
//  2. Build chained nodes and groups from relay_through definitions
//  3. Compute @all (original proxy names, excluding chained)
func Group(proxies []model.Proxy, cfg *config.Config) (*GroupResult, error) {
	regionGroups, err := buildRegionGroups(proxies, &cfg.Groups)
	if err != nil {
		return nil, err
	}

	chainedProxies, chainGroups, err := buildChainedNodesAndGroups(
		proxies, cfg.Sources.CustomProxies, regionGroups,
	)
	if err != nil {
		return nil, err
	}

	allProxies := computeAllProxies(proxies)

	nodeGroups := make([]model.ProxyGroup, 0, len(regionGroups)+len(chainGroups))
	nodeGroups = append(nodeGroups, regionGroups...)
	nodeGroups = append(nodeGroups, chainGroups...)

	combined := make([]model.Proxy, 0, len(proxies)+len(chainedProxies))
	combined = append(combined, proxies...)
	combined = append(combined, chainedProxies...)

	return &GroupResult{
		Proxies:    combined,
		NodeGroups: nodeGroups,
		AllProxies: allProxies,
	}, nil
}

// buildRegionGroups creates node groups by matching fetched proxies
// (KindSubscription + KindSnell + KindVLess, see isFetchedKind) against
// each group's regex pattern, in groups declaration order. KindCustom and
// KindChained are excluded from matching: custom proxies are already named
// exactly by the user, and chained proxies are derived from upstreams.
func buildRegionGroups(proxies []model.Proxy, groups *config.OrderedMap[config.Group]) ([]model.ProxyGroup, error) {
	fetched := fetchedProxies(proxies)
	result := make([]model.ProxyGroup, 0, groups.Len())

	for name, g := range groups.Entries() {
		re, err := regexp.Compile(g.Match)
		if err != nil {
			return nil, &errtype.BuildError{
				Code:    errtype.CodeBuildGroupRegexInvalid,
				Phase:   "group",
				Message: fmt.Sprintf("节点组 %q 的 match 正则无效：%v", name, err),
			}
		}

		var members []string
		for _, p := range fetched {
			if re.MatchString(p.Name) {
				members = append(members, p.Name)
			}
		}

		result = append(result, model.ProxyGroup{
			Name:     name,
			Scope:    model.ScopeNode,
			Strategy: g.Strategy,
			Members:  members,
		})
	}
	return result, nil
}

// buildChainedNodesAndGroups generates chained proxies and their groups
// from custom proxies that have relay_through definitions. Upstream
// candidates are drawn from fetched proxies (KindSubscription + KindSnell
// + KindVLess, see isFetchedKind).
func buildChainedNodesAndGroups(
	proxies []model.Proxy,
	customProxies []config.CustomProxy,
	regionGroups []model.ProxyGroup,
) ([]model.Proxy, []model.ProxyGroup, error) {
	fetched := fetchedProxies(proxies)

	var chainedProxies []model.Proxy
	var chainGroups []model.ProxyGroup

	for _, cp := range customProxies {
		if cp.RelayThrough == nil {
			continue
		}
		rt := cp.RelayThrough

		upstreams, err := resolveUpstreams(fetched, regionGroups, rt)
		if err != nil {
			return nil, nil, err
		}

		var members []string
		for _, upstream := range upstreams {
			chained := model.Proxy{
				Name:   upstream.Name + "→" + cp.Name,
				Type:   cp.Type,
				Server: cp.Server,
				Port:   cp.Port,
				Params: buildCustomParams(cp),
				Kind:   model.KindChained,
				Dialer: upstream.Name,
			}
			chainedProxies = append(chainedProxies, chained)
			members = append(members, chained.Name)
		}

		chainGroups = append(chainGroups, model.ProxyGroup{
			Name:     chainedGroupPrefix + cp.Name,
			Scope:    model.ScopeNode,
			Strategy: rt.Strategy,
			Members:  members,
		})
	}

	return chainedProxies, chainGroups, nil
}

// resolveUpstreams determines the upstream proxies for a relay_through
// definition. The candidate pool is fetched proxies (KindSubscription +
// KindSnell + KindVLess, see isFetchedKind); custom and chained proxies
// are never valid upstreams.
func resolveUpstreams(
	fetched []model.Proxy,
	regionGroups []model.ProxyGroup,
	rt *config.RelayThrough,
) ([]model.Proxy, error) {
	switch rt.Type {
	case "group":
		group, ok := findGroupByName(regionGroups, rt.Name)
		if !ok {
			return nil, &errtype.BuildError{
				Code:    errtype.CodeBuildRelayGroupMissing,
				Phase:   "group",
				Message: fmt.Sprintf("relay_through type=group 引用了不存在的节点组 %q", rt.Name),
			}
		}
		return resolveMembers(fetched, group.Members), nil

	case "select":
		re, err := regexp.Compile(rt.Match)
		if err != nil {
			return nil, &errtype.BuildError{
				Code:    errtype.CodeBuildRelayRegexInvalid,
				Phase:   "group",
				Message: fmt.Sprintf("relay_through type=select 的正则无效：%v", err),
			}
		}
		var matched []model.Proxy
		for _, p := range fetched {
			if re.MatchString(p.Name) {
				matched = append(matched, p)
			}
		}
		return matched, nil

	case "all":
		result := make([]model.Proxy, len(fetched))
		copy(result, fetched)
		return result, nil

	default:
		return nil, &errtype.BuildError{
			Code:    errtype.CodeBuildRelayTypeInvalid,
			Phase:   "group",
			Message: fmt.Sprintf("relay_through 的 type %q 无效", rt.Type),
		}
	}
}

// resolveMembers looks up proxy objects by name from a proxy pool.
func resolveMembers(pool []model.Proxy, names []string) []model.Proxy {
	index := make(map[string]model.Proxy, len(pool))
	for _, p := range pool {
		index[p.Name] = p
	}

	result := make([]model.Proxy, 0, len(names))
	for _, name := range names {
		if p, ok := index[name]; ok {
			result = append(result, p)
		}
	}
	return result
}

// computeAllProxies collects names of original proxies (KindSubscription +
// KindSnell + KindVLess + KindCustom), excluding chained proxies. Called
// before chained nodes are generated so `@all` never includes chained
// derivatives.
func computeAllProxies(proxies []model.Proxy) []string {
	result := make([]string, 0, len(proxies))
	for _, p := range proxies {
		if p.Kind != model.KindChained {
			result = append(result, p.Name)
		}
	}
	return result
}

// fetchedProxies returns proxies sourced via remote fetch:
// KindSubscription (SS subscriptions), KindSnell (Snell sources), and
// KindVLess (VLESS sources). These are the original proxies that
// participate in region-group regex matching and serve as valid chain
// upstreams.
//
// Custom proxies are excluded because they are already named exactly and
// should not be matched by region regexes. Chained proxies are excluded
// because they derive from other nodes.
func fetchedProxies(proxies []model.Proxy) []model.Proxy {
	result := make([]model.Proxy, 0, len(proxies))
	for _, p := range proxies {
		if isFetchedKind(p.Kind) {
			result = append(result, p)
		}
	}
	return result
}

// findGroupByName looks up a proxy group by name.
func findGroupByName(groups []model.ProxyGroup, name string) (model.ProxyGroup, bool) {
	for _, g := range groups {
		if g.Name == name {
			return g, true
		}
	}
	return model.ProxyGroup{}, false
}
