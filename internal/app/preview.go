package app

import (
	"context"
	"encoding/json"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/generate"
	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/pipeline"
)

type NodePreviewResult struct {
	Nodes         []NodePreviewItem `json:"nodes"`
	Total         int               `json:"total"`
	ActiveCount   int               `json:"active_count"`
	FilteredCount int               `json:"filtered_count"`
}

type NodePreviewItem struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Kind     string `json:"kind"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Filtered bool   `json:"filtered"`
}

type GroupPreviewResult struct {
	NodeGroups    []GroupItem        `json:"node_groups"`
	ChainedGroups []GroupItem        `json:"chained_groups"`
	ServiceGroups []ServiceGroupItem `json:"service_groups"`
	AllProxies    []string           `json:"all_proxies"`
}

type GroupItem struct {
	Name     string   `json:"name"`
	Strategy string   `json:"strategy"`
	Members  []string `json:"members"`
}

type ServiceGroupItem struct {
	Name            string               `json:"name"`
	Strategy        string               `json:"strategy"`
	Members         []string             `json:"members"`
	ExpandedMembers []ExpandedMemberItem `json:"expanded_members"`
}

type ExpandedMemberItem struct {
	Value  string `json:"value"`
	Origin string `json:"origin"`
}

func (s *Service) PreviewNodes(ctx context.Context) (*NodePreviewResult, error) {
	cfg := s.runtimeSnapshot()
	filter, err := pipeline.SourceAndFilter(ctx, cfg, s.fetcher)
	if err != nil {
		return nil, err
	}
	return nodePreviewFromFilter(filter), nil
}

func (s *Service) PreviewNodesFromDraft(ctx context.Context, configJSON json.RawMessage) (*NodePreviewResult, error) {
	cfg, err := runtimeFromDraft(configJSON)
	if err != nil {
		return nil, err
	}
	filter, err := pipeline.SourceAndFilter(ctx, cfg, s.fetcher)
	if err != nil {
		return nil, err
	}
	return nodePreviewFromFilter(filter), nil
}

func (s *Service) PreviewGroups(ctx context.Context) (*GroupPreviewResult, error) {
	cfg := s.runtimeSnapshot()
	result, err := pipeline.SourceFilterGroupRouteValidate(ctx, cfg, s.fetcher)
	if err != nil {
		return nil, err
	}
	return groupPreviewFromStage(result), nil
}

func (s *Service) PreviewGroupsFromDraft(ctx context.Context, configJSON json.RawMessage) (*GroupPreviewResult, error) {
	cfg, err := runtimeFromDraft(configJSON)
	if err != nil {
		return nil, err
	}
	result, err := pipeline.SourceFilterGroupRouteValidate(ctx, cfg, s.fetcher)
	if err != nil {
		return nil, err
	}
	return groupPreviewFromStage(result), nil
}

func (s *Service) GenerateFromDraft(ctx context.Context, req generate.Request, configJSON json.RawMessage) (*generate.Result, error) {
	cfg, err := runtimeFromDraft(configJSON)
	if err != nil {
		return nil, err
	}
	return s.generator.Generate(ctx, cfg, req)
}

func runtimeFromDraft(configJSON json.RawMessage) (*config.RuntimeConfig, error) {
	cfg, err := parseConfigJSON(configJSON)
	if err != nil {
		return nil, err
	}
	return prepareAdminConfig(cfg, true)
}

func nodePreviewFromFilter(filter *pipeline.FilterResult) *NodePreviewResult {
	if filter == nil {
		return &NodePreviewResult{Nodes: []NodePreviewItem{}}
	}
	nodes := make([]NodePreviewItem, 0, len(filter.All))
	for _, item := range filter.All {
		nodes = append(nodes, nodePreviewItem(item.Proxy, item.Filtered))
	}
	return &NodePreviewResult{
		Nodes:         nodes,
		Total:         len(nodes),
		ActiveCount:   len(filter.Included),
		FilteredCount: len(filter.Excluded),
	}
}

func nodePreviewItem(proxy model.Proxy, filtered bool) NodePreviewItem {
	return NodePreviewItem{
		Name:     proxy.Name,
		Type:     proxy.Type,
		Kind:     string(proxy.Kind),
		Server:   proxy.Server,
		Port:     proxy.Port,
		Filtered: filtered,
	}
}

func groupPreviewFromStage(stage *pipeline.GroupPreviewStageResult) *GroupPreviewResult {
	if stage == nil || stage.Group == nil || stage.Route == nil {
		return &GroupPreviewResult{
			NodeGroups:    []GroupItem{},
			ChainedGroups: []GroupItem{},
			ServiceGroups: []ServiceGroupItem{},
			AllProxies:    []string{},
		}
	}
	return &GroupPreviewResult{
		NodeGroups:    groupItems(stage.Group.RegionGroups),
		ChainedGroups: groupItems(stage.Group.ChainedGroups),
		ServiceGroups: serviceGroupItems(stage.Route),
		AllProxies:    append([]string(nil), stage.Group.AllProxies...),
	}
}

func groupItems(groups []model.ProxyGroup) []GroupItem {
	result := make([]GroupItem, 0, len(groups))
	for _, group := range groups {
		result = append(result, GroupItem{
			Name:     group.Name,
			Strategy: group.Strategy,
			Members:  append([]string(nil), group.Members...),
		})
	}
	return result
}

func serviceGroupItems(route *pipeline.RouteResult) []ServiceGroupItem {
	result := make([]ServiceGroupItem, 0, len(route.RouteGroups))
	resolved := resolvedMembersByName(route.ResolvedRouteGroups)
	for _, group := range route.RouteGroups {
		result = append(result, ServiceGroupItem{
			Name:            group.Name,
			Strategy:        group.Strategy,
			Members:         append([]string(nil), group.Members...),
			ExpandedMembers: expandedMemberItems(resolved[group.Name]),
		})
	}
	return result
}

func resolvedMembersByName(groups []pipeline.ResolvedRouteGroup) map[string][]config.PreparedRouteMember {
	result := make(map[string][]config.PreparedRouteMember, len(groups))
	for _, group := range groups {
		result[group.Name] = group.Members
	}
	return result
}

func expandedMemberItems(members []config.PreparedRouteMember) []ExpandedMemberItem {
	result := make([]ExpandedMemberItem, 0, len(members))
	for _, member := range members {
		result = append(result, ExpandedMemberItem{
			Value:  member.Raw,
			Origin: string(member.Origin),
		})
	}
	return result
}
