package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/John-Robertt/subconverter/internal/config"
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
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Kind     string `json:"kind"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Filtered bool   `json:"filtered"`
}

type GroupPreviewResult struct {
	NodeGroups      []GroupItem         `json:"node_groups"`
	ChainedGroups   []GroupItem         `json:"chained_groups"`
	ServiceGroups   []ServiceGroupItem  `json:"service_groups"`
	AllProxies      []string            `json:"all_proxies"`
	AllProxyDetails []PreviewMemberItem `json:"all_proxy_details"`
}

type GroupItem struct {
	Name          string              `json:"name"`
	Match         string              `json:"match,omitempty"`
	Strategy      string              `json:"strategy"`
	Members       []string            `json:"members"`
	MemberDetails []PreviewMemberItem `json:"member_details"`
}

type ServiceGroupItem struct {
	Name                  string               `json:"name"`
	Strategy              string               `json:"strategy"`
	Members               []string             `json:"members"`
	MemberDetails         []PreviewMemberItem  `json:"member_details"`
	ExpandedMembers       []ExpandedMemberItem `json:"expanded_members"`
	ExpandedMemberDetails []PreviewMemberItem  `json:"expanded_member_details"`
}

type ExpandedMemberItem struct {
	Value  string `json:"value"`
	Origin string `json:"origin"`
}

type PreviewMemberItem struct {
	Kind   string `json:"kind"`
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	Origin string `json:"origin,omitempty"`
}

func (s *Service) PreviewNodes(ctx context.Context) (*NodePreviewResult, error) {
	cfg, err := s.runtimeSnapshot()
	if err != nil {
		return nil, err
	}
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
	cfg, err := s.runtimeSnapshot()
	if err != nil {
		return nil, err
	}
	result, err := pipeline.SourceFilterGroupRouteValidate(ctx, cfg, s.fetcher)
	if err != nil {
		return nil, err
	}
	return groupPreviewFromStage(result, cfg), nil
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
	return groupPreviewFromStage(result, cfg), nil
}

func (s *Service) GenerateFromDraft(ctx context.Context, req GenerateInput, configJSON json.RawMessage) (*GenerateResult, error) {
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
		ID:       previewProxyID(proxy.ID),
		Name:     proxy.Name,
		Type:     proxy.Type,
		Kind:     string(proxy.Kind),
		Server:   proxy.Server,
		Port:     proxy.Port,
		Filtered: filtered,
	}
}

func groupPreviewFromStage(stage *pipeline.GroupPreviewStageResult, cfg *config.RuntimeConfig) *GroupPreviewResult {
	if stage == nil || stage.Group == nil || stage.Route == nil {
		return &GroupPreviewResult{
			NodeGroups:      []GroupItem{},
			ChainedGroups:   []GroupItem{},
			ServiceGroups:   []ServiceGroupItem{},
			AllProxies:      []string{},
			AllProxyDetails: []PreviewMemberItem{},
		}
	}
	matchByName := groupMatchMap(cfg)
	proxiesByID := proxyPreviewIndex(stage.Group.Proxies)
	allProxyDetails := proxyPreviewItems(stage.Group.AllProxies, proxiesByID)
	return &GroupPreviewResult{
		NodeGroups:      groupItems(stage.Group.RegionGroups, matchByName, proxiesByID),
		ChainedGroups:   groupItems(stage.Group.ChainedGroups, matchByName, proxiesByID),
		ServiceGroups:   serviceGroupItems(stage.Route, proxiesByID),
		AllProxies:      previewMemberNames(allProxyDetails),
		AllProxyDetails: allProxyDetails,
	}
}

func groupMatchMap(cfg *config.RuntimeConfig) map[string]string {
	if cfg == nil {
		return nil
	}
	groups := cfg.GroupInput()
	m := make(map[string]string, len(groups))
	for _, g := range groups {
		m[g.Name] = g.RawMatch
	}
	return m
}

func groupItems(groups []model.NodeGroup, matchByName map[string]string, proxiesByID map[model.ProxyID]model.Proxy) []GroupItem {
	result := make([]GroupItem, 0, len(groups))
	for _, group := range groups {
		memberDetails := proxyPreviewItems(group.Members, proxiesByID)
		result = append(result, GroupItem{
			Name:          group.Name,
			Match:         matchByName[group.Name],
			Strategy:      group.Strategy,
			Members:       previewMemberNames(memberDetails),
			MemberDetails: memberDetails,
		})
	}
	return result
}

func serviceGroupItems(route *pipeline.RouteResult, proxiesByID map[model.ProxyID]model.Proxy) []ServiceGroupItem {
	result := make([]ServiceGroupItem, 0, len(route.RouteGroups))
	resolved := resolvedMembersByName(route.ResolvedRouteGroups)
	for _, group := range route.RouteGroups {
		memberDetails := policyPreviewItems(group.Members, proxiesByID, "")
		expanded := resolved[group.Name]
		expandedDetails := resolvedPreviewItems(expanded, proxiesByID)
		result = append(result, ServiceGroupItem{
			Name:                  group.Name,
			Strategy:              group.Strategy,
			Members:               previewMemberNames(memberDetails),
			MemberDetails:         memberDetails,
			ExpandedMembers:       expandedMemberItems(expandedDetails),
			ExpandedMemberDetails: expandedDetails,
		})
	}
	return result
}

func resolvedMembersByName(groups []pipeline.ResolvedRouteGroup) map[string][]pipeline.ResolvedRouteMember {
	result := make(map[string][]pipeline.ResolvedRouteMember, len(groups))
	for _, group := range groups {
		result[group.Name] = group.Members
	}
	return result
}

func resolvedPreviewItems(members []pipeline.ResolvedRouteMember, proxiesByID map[model.ProxyID]model.Proxy) []PreviewMemberItem {
	result := make([]PreviewMemberItem, 0, len(members))
	for _, member := range members {
		result = append(result, policyPreviewItem(member.Ref, proxiesByID, string(member.Origin)))
	}
	return result
}

func expandedMemberItems(members []PreviewMemberItem) []ExpandedMemberItem {
	result := make([]ExpandedMemberItem, 0, len(members))
	for _, member := range members {
		result = append(result, ExpandedMemberItem{
			Value:  member.Name,
			Origin: member.Origin,
		})
	}
	return result
}

func previewMemberNames(members []PreviewMemberItem) []string {
	result := make([]string, 0, len(members))
	for _, member := range members {
		result = append(result, member.Name)
	}
	return result
}

func proxyPreviewIndex(proxies []model.Proxy) map[model.ProxyID]model.Proxy {
	result := make(map[model.ProxyID]model.Proxy, len(proxies))
	for _, proxy := range proxies {
		result[proxy.ID] = proxy
	}
	return result
}

func proxyPreviewItems(ids []model.ProxyID, proxiesByID map[model.ProxyID]model.Proxy) []PreviewMemberItem {
	result := make([]PreviewMemberItem, 0, len(ids))
	for _, id := range ids {
		proxy, exists := proxiesByID[id]
		name := ""
		if exists {
			name = proxy.Name
		}
		result = append(result, PreviewMemberItem{
			Kind: "proxy",
			ID:   previewProxyID(id),
			Name: name,
		})
	}
	return result
}

func policyPreviewItems(refs []model.PolicyRef, proxiesByID map[model.ProxyID]model.Proxy, origin string) []PreviewMemberItem {
	result := make([]PreviewMemberItem, 0, len(refs))
	for _, ref := range refs {
		result = append(result, policyPreviewItem(ref, proxiesByID, origin))
	}
	return result
}

func policyPreviewItem(ref model.PolicyRef, proxiesByID map[model.ProxyID]model.Proxy, origin string) PreviewMemberItem {
	item := PreviewMemberItem{Kind: string(ref.Kind), Name: ref.Name, Origin: origin}
	if ref.Kind == model.PolicyRefProxy {
		item.ID = previewProxyID(ref.ProxyID)
		if proxy, exists := proxiesByID[ref.ProxyID]; exists {
			item.Name = proxy.Name
		}
	}
	return item
}

func previewProxyID(id model.ProxyID) string {
	return fmt.Sprintf("proxy-%d", id)
}
