package render

import (
	"fmt"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

type proxyDropReasonKind string

const (
	proxyDropReasonSnellRoot     proxyDropReasonKind = "snell_root"
	proxyDropReasonDialerCascade proxyDropReasonKind = "dialer_cascade"
)

type proxyDropReason struct {
	Kind   proxyDropReasonKind
	Parent string
}

// filterForClash returns a Pipeline view with Snell-originated proxies and
// their cascading consequences removed, so Clash Meta output never references
// Snell nodes (Clash Meta mainline does not support Snell v4/v5, which is
// the version jinqians/snell.sh produces by default).
//
// Cascade rules:
//  1. Any proxy with Type=="snell" is dropped.
//  2. Any chained proxy whose Dialer resolves to a dropped proxy is also
//     dropped (its upstream is gone).
//  3. Any proxy group (node or route) whose Members set becomes empty after
//     removing dropped proxy names and dropped group names is itself dropped.
//     This rule iterates until the set of dropped groups is stable.
//  4. Rulesets whose Policy is a dropped group are removed.
//  5. Rules whose Policy is a dropped group are removed.
//  6. If the Fallback group is dropped, Clash has no valid final policy and
//     this function returns a RenderError (CodeRenderClashFallbackEmpty)
//     whose message includes the cascade path so users can find the root cause.
//
// The input Pipeline is not mutated; a shallow copy with filtered slices is
// returned. Pipelines with no Snell proxies return the original pointer.
func filterForClash(p *model.Pipeline) (*model.Pipeline, error) {
	// Step 1 & 2: compute the set of dropped proxy names.
	dropped := make(map[string]struct{})
	proxyReasons := make(map[string]proxyDropReason)
	for _, px := range p.Proxies {
		if px.Type == "snell" {
			dropped[px.Name] = struct{}{}
			proxyReasons[px.Name] = proxyDropReason{Kind: proxyDropReasonSnellRoot}
		}
	}
	// Chained proxies whose dialer is dropped: iterate until stable. Each
	// round must add at least one entry to `dropped` to continue, so the
	// loop is bounded by len(p.Proxies). The maxIter guard below asserts
	// termination in case a future refactor breaks that invariant.
	maxChainIter := len(p.Proxies) + 1
	for i := 0; ; i++ {
		if i >= maxChainIter {
			return nil, internalFilterError("链式剔除未收敛（超过 %d 轮），疑似循环 dialer 引用", maxChainIter)
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
		// Nothing to do — return the original Pipeline.
		return p, nil
	}

	// Step 3: filter groups iteratively. A group is dropped when its Members
	// become empty after removing dropped proxy names *and* already-dropped
	// group names. Each group carries a reason (the list of names whose drop
	// triggered its clearance), so we can reconstruct the cascade path if
	// the fallback ends up empty.
	droppedGroups := make(map[string]struct{})
	groupReasons := make(map[string][]string)
	filteredNodeGroups := make([]model.ProxyGroup, len(p.NodeGroups))
	copy(filteredNodeGroups, p.NodeGroups)
	filteredRouteGroups := make([]model.ProxyGroup, len(p.RouteGroups))
	copy(filteredRouteGroups, p.RouteGroups)

	maxGroupIter := len(filteredNodeGroups) + len(filteredRouteGroups) + 1
	for i := 0; ; i++ {
		if i >= maxGroupIter {
			return nil, internalFilterError("组级联剔除未收敛（超过 %d 轮），疑似相互引用", maxGroupIter)
		}
		grew := false
		// Node groups.
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
		// Route (service) groups.
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

	// Step 6: fallback must survive. If it didn't, print the cascade path
	// so users can trace the clearance from fallback back to the Snell
	// proxies that triggered it.
	if _, fallbackDropped := droppedGroups[p.Fallback]; fallbackDropped {
		path := buildDropPath(p.Fallback, groupReasons, proxyReasons, make(map[string]bool))
		return nil, &errtype.RenderError{
			Code:    errtype.CodeRenderClashFallbackEmpty,
			Format:  "clash",
			Message: fmt.Sprintf("fallback 服务组 %q 在 Clash 输出中成员为空（被 snell 过滤级联清空）。清空路径：%s", p.Fallback, path),
		}
	}

	// Step 3 (final pass): compact the surviving groups into new slices.
	nodeGroupsOut := make([]model.ProxyGroup, 0, len(filteredNodeGroups))
	for _, g := range filteredNodeGroups {
		if _, dropped := droppedGroups[g.Name]; dropped {
			continue
		}
		nodeGroupsOut = append(nodeGroupsOut, g)
	}
	routeGroupsOut := make([]model.ProxyGroup, 0, len(filteredRouteGroups))
	for _, g := range filteredRouteGroups {
		if _, dropped := droppedGroups[g.Name]; dropped {
			continue
		}
		routeGroupsOut = append(routeGroupsOut, g)
	}

	// Step 4: rulesets whose Policy points at a dropped group are removed.
	rulesetsOut := make([]model.Ruleset, 0, len(p.Rulesets))
	for _, rs := range p.Rulesets {
		if _, dropped := droppedGroups[rs.Policy]; dropped {
			continue
		}
		rulesetsOut = append(rulesetsOut, rs)
	}

	// Step 5: inline rules whose Policy points at a dropped group are removed.
	rulesOut := make([]model.Rule, 0, len(p.Rules))
	for _, r := range p.Rules {
		if _, dropped := droppedGroups[r.Policy]; dropped {
			continue
		}
		rulesOut = append(rulesOut, r)
	}

	// Filter proxies.
	proxiesOut := make([]model.Proxy, 0, len(p.Proxies))
	for _, px := range p.Proxies {
		if _, d := dropped[px.Name]; d {
			continue
		}
		proxiesOut = append(proxiesOut, px)
	}

	// Filter AllProxies (cosmetic: renderers don't emit it directly, but
	// keep the Pipeline internally consistent).
	allOut := make([]string, 0, len(p.AllProxies))
	for _, name := range p.AllProxies {
		if _, d := dropped[name]; d {
			continue
		}
		allOut = append(allOut, name)
	}

	filtered := &model.Pipeline{
		Proxies:     proxiesOut,
		NodeGroups:  nodeGroupsOut,
		RouteGroups: routeGroupsOut,
		Rulesets:    rulesetsOut,
		Rules:       rulesOut,
		Fallback:    p.Fallback,
		AllProxies:  allOut,
	}
	return filtered, nil
}

// pruneMembers returns a new slice with members removed whose names are in
// either droppedProxies or droppedGroups. Order is preserved.
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

// buildDropPath renders a human-readable chain explaining why a group was
// cleared. Groups recurse into the members that caused them to empty. Dropped
// proxies use an explicit reason graph: root Snell nodes render as `(snell)`,
// while chained proxies render as `(chained)` and point at the upstream drop
// path that cascaded into them.
//
// visited prevents infinite recursion if the graph contains a cycle
// (shouldn't happen given ValidateGraph's checks, but the guard is cheap).
func buildDropPath(name string, groupReasons map[string][]string, proxyReasons map[string]proxyDropReason, visited map[string]bool) string {
	if visited[name] {
		return name + "(cycle)"
	}

	if reason, isProxy := proxyReasons[name]; isProxy {
		switch reason.Kind {
		case proxyDropReasonSnellRoot:
			return name + "(snell)"
		case proxyDropReasonDialerCascade:
			visited[name] = true
			parentPath := reason.Parent
			if reason.Parent != "" {
				parentPath = buildDropPath(reason.Parent, groupReasons, proxyReasons, visited)
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
		parts = append(parts, buildDropPath(m, groupReasons, proxyReasons, visited))
	}
	delete(visited, name)
	return fmt.Sprintf("%s ← [%s]", name, strings.Join(parts, ", "))
}

// internalFilterError wraps an invariant-violation error. Reaching these
// paths means the cascade loop failed to converge, which indicates a bug
// in filterForClash or a malformed Pipeline the earlier stages should have
// rejected.
func internalFilterError(format string, args ...any) error {
	return &errtype.RenderError{
		Code:    errtype.CodeRenderClashFallbackEmpty,
		Format:  "clash",
		Message: "内部不变量异常：" + fmt.Sprintf(format, args...),
	}
}
