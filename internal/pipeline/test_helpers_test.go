package pipeline

import (
	"context"
	"strings"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
	"gopkg.in/yaml.v3"
)

// Execute is kept as a test-only alias so existing stage-composition tests can
// stay focused on behavior while production code uses the renamed Build API.
func Execute(ctx context.Context, cfg *config.Config, fetcher fetch.Fetcher) (*model.Pipeline, error) {
	rt, err := prepareRuntimeForStage(cfg)
	if err != nil {
		return nil, err
	}
	return Build(ctx, rt, fetcher)
}

// Source is kept as a test-only helper so stage tests can still start from raw
// Config while production code always goes through config.Prepare first.
func Source(ctx context.Context, cfg *config.Config, fetcher fetch.Fetcher) (*SourceResult, error) {
	rt, err := prepareRuntimeForStage(cfg)
	if err != nil {
		return nil, err
	}
	return sourcePrepared(ctx, rt.SourceInput(), rt.StaticNamespace(), fetcher)
}

func groupFromConfig(cfg *config.Config, proxies []model.Proxy) (*GroupResult, error) {
	rt, err := prepareRuntimeForStage(cfg)
	if err != nil {
		return nil, err
	}

	sources := rt.SourceInput()
	standalone, chainTemplates := convertCustomProxies(sources.CustomProxies)

	allProxies := make([]model.Proxy, 0, len(proxies)+len(standalone))
	allProxies = append(allProxies, proxies...)
	allProxies = append(allProxies, standalone...)
	return Group(&SourceResult{
		Proxies:        allProxies,
		ChainTemplates: chainTemplates,
	}, rt.GroupInput())
}

func routeFromConfig(cfg *config.Config, gr *GroupResult) (*RouteResult, error) {
	rt, err := prepareRuntimeForRouteStage(cfg, gr)
	if err != nil {
		return nil, err
	}

	routing, rulesets, rules, fallback := rt.RouteInput()
	if cfg != nil && cfg.Routing.Len() > 0 {
		originalNames := make(map[string]bool, cfg.Routing.Len())
		for name := range cfg.Routing.Entries() {
			originalNames[name] = true
		}
		filtered := make([]config.PreparedRouteGroup, 0, len(routing))
		for _, group := range routing {
			if originalNames[group.Name] {
				filtered = append(filtered, group)
			}
		}
		routing = filtered
	} else {
		routing = nil
	}
	return Route(routing, rulesets, rules, fallback, gr)
}

func prepareRuntimeForStage(cfg *config.Config) (*config.RuntimeConfig, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	stageCfg, err := makeStageReadyConfig(cfg)
	if err != nil {
		return nil, err
	}
	return config.Prepare(stageCfg)
}

func prepareRuntimeForRouteStage(cfg *config.Config, gr *GroupResult) (*config.RuntimeConfig, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	stageCfg, err := makeStageReadyConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err := augmentGroupsForRouteStage(stageCfg, gr); err != nil {
		return nil, err
	}
	return config.Prepare(stageCfg)
}

func makeStageReadyConfig(cfg *config.Config) (*config.Config, error) {
	stageCfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: append([]config.Subscription(nil), cfg.Sources.Subscriptions...),
			Snell:         append([]config.SnellSource(nil), cfg.Sources.Snell...),
			VLess:         append([]config.VLessSource(nil), cfg.Sources.VLess...),
			CustomProxies: append([]config.CustomProxy(nil), cfg.Sources.CustomProxies...),
			FetchOrder:    append([]string(nil), cfg.Sources.FetchOrder...),
		},
		Filters:   cfg.Filters,
		Rules:     append([]string(nil), cfg.Rules...),
		Fallback:  cfg.Fallback,
		BaseURL:   cfg.BaseURL,
		Templates: cfg.Templates,
	}

	var err error
	stageCfg.Groups, err = cloneGroupMap(cfg.Groups)
	if err != nil {
		return nil, err
	}
	stageCfg.Routing, stageCfg.Fallback, err = stageReadyRouting(cfg)
	if err != nil {
		return nil, err
	}
	stageCfg.Rulesets, err = cloneStringSliceMap(cfg.Rulesets)
	if err != nil {
		return nil, err
	}

	return stageCfg, nil
}

func augmentGroupsForRouteStage(cfg *config.Config, gr *GroupResult) error {
	if cfg == nil {
		return nil
	}

	routeNames := make(map[string]bool, cfg.Routing.Len())
	for name := range cfg.Routing.Entries() {
		routeNames[name] = true
	}

	extraNames := make([]string, 0)
	appendExtraName := func(name string) {
		if name == "" || config.IsReservedPolicyName(name) || name == "@all" || name == "@auto" || routeNames[name] {
			return
		}
		extraNames = append(extraNames, name)
	}

	if gr != nil {
		for _, group := range gr.NodeGroups {
			appendExtraName(group.Name)
		}
	}
	for _, members := range cfg.Routing.Entries() {
		for _, member := range members {
			appendExtraName(member)
		}
	}

	if len(extraNames) == 0 && cfg.Groups.Len() == 0 {
		return nil
	}

	entries := make([]groupEntry, 0, cfg.Groups.Len()+len(extraNames))
	seen := make(map[string]bool, cfg.Groups.Len()+len(extraNames))
	for name, group := range cfg.Groups.Entries() {
		entries = append(entries, groupEntry{name: name, group: group})
		seen[name] = true
	}

	for _, name := range extraNames {
		if seen[name] {
			continue
		}
		entries = append(entries, groupEntry{
			name: name,
			group: config.Group{
				Match:    "^$",
				Strategy: "select",
			},
		})
		seen[name] = true
	}

	groups, err := buildGroupMap(entries)
	if err != nil {
		return err
	}
	cfg.Groups = groups
	return nil
}

func stageReadyRouting(cfg *config.Config) (config.OrderedMap[[]string], string, error) {
	entries := make([]stringSliceEntry, 0, cfg.Routing.Len())
	seen := make(map[string]bool, cfg.Routing.Len()+len(cfg.Rules)+len(cfg.Fallback))
	for name, members := range cfg.Routing.Entries() {
		entries = append(entries, stringSliceEntry{name: name, values: append([]string(nil), members...)})
		seen[name] = true
	}

	addEntry := func(name string) {
		if name == "" || config.IsReservedPolicyName(name) || seen[name] {
			return
		}
		entries = append(entries, stringSliceEntry{name: name, values: []string{"DIRECT"}})
		seen[name] = true
	}

	for policy := range cfg.Rulesets.Entries() {
		addEntry(policy)
	}

	for _, raw := range cfg.Rules {
		if idx := strings.LastIndex(raw, ","); idx >= 0 {
			addEntry(raw[idx+1:])
		}
	}

	addEntry(cfg.Fallback)

	if len(entries) == 0 {
		entries = append(entries, stringSliceEntry{name: "__test_route__", values: []string{"DIRECT"}})
	}

	routing, err := buildStringSliceMap(entries)
	if err != nil {
		return config.OrderedMap[[]string]{}, "", err
	}

	fallback := cfg.Fallback
	if fallback == "" {
		fallback = entries[0].name
	}
	return routing, fallback, nil
}

func cloneGroupMap(src config.OrderedMap[config.Group]) (config.OrderedMap[config.Group], error) {
	entries := make([]groupEntry, 0, src.Len())
	for name, group := range src.Entries() {
		entries = append(entries, groupEntry{name: name, group: group})
	}
	return buildGroupMap(entries)
}

func cloneStringSliceMap(src config.OrderedMap[[]string]) (config.OrderedMap[[]string], error) {
	entries := make([]stringSliceEntry, 0, src.Len())
	for name, values := range src.Entries() {
		entries = append(entries, stringSliceEntry{name: name, values: append([]string(nil), values...)})
	}
	return buildStringSliceMap(entries)
}

type groupEntry struct {
	name  string
	group config.Group
}

func buildGroupMap(entries []groupEntry) (config.OrderedMap[config.Group], error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, entry := range entries {
		node.Content = append(node.Content, scalarNode(entry.name))
		value := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		value.Content = append(value.Content,
			scalarNode("match"), scalarNode(entry.group.Match),
			scalarNode("strategy"), scalarNode(entry.group.Strategy),
		)
		node.Content = append(node.Content, value)
	}

	var result config.OrderedMap[config.Group]
	if err := node.Decode(&result); err != nil {
		return config.OrderedMap[config.Group]{}, err
	}
	return result, nil
}

type stringSliceEntry struct {
	name   string
	values []string
}

func buildStringSliceMap(entries []stringSliceEntry) (config.OrderedMap[[]string], error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, entry := range entries {
		node.Content = append(node.Content, scalarNode(entry.name))
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, value := range entry.values {
			seq.Content = append(seq.Content, scalarNode(value))
		}
		node.Content = append(node.Content, seq)
	}

	var result config.OrderedMap[[]string]
	if err := node.Decode(&result); err != nil {
		return config.OrderedMap[[]string]{}, err
	}
	return result, nil
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func customProxy(name, rawURL string, rt *config.RelayThrough) config.CustomProxy {
	return config.CustomProxy{
		Name:         name,
		URL:          rawURL,
		RelayThrough: rt,
	}
}
