package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
)

// T-PRV-001: SourceAndFilter returns included/excluded with ordered view
func TestSourceAndFilterReturnsIncludedExcludedAndOrderedView(t *testing.T) {
	subURL := "https://sub.example.com/api"
	cfg := previewRuntimeConfig(t, subURL, "(过期)")
	fetcher := &fakeFetcher{responses: map[string][]byte{subURL: makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@expire.example.com:8388#过期提醒",
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01",
	)}}

	result, err := SourceAndFilter(context.Background(), cfg, fetcher)
	if err != nil {
		t.Fatalf("SourceAndFilter: %v", err)
	}

	if got := len(result.Included); got != 2 {
		t.Fatalf("Included = %d, want 2", got)
	}
	if got := len(result.Excluded); got != 1 {
		t.Fatalf("Excluded = %d, want 1", got)
	}
	if got := len(result.All); got != 3 {
		t.Fatalf("All = %d, want 3", got)
	}
	if result.All[0].Proxy.Name != "HK-01" || result.All[0].Filtered {
		t.Fatalf("All[0] = %+v, want active HK-01", result.All[0])
	}
	if result.All[1].Proxy.Name != "过期提醒" || !result.All[1].Filtered {
		t.Fatalf("All[1] = %+v, want filtered 过期提醒", result.All[1])
	}
	if result.All[2].Proxy.Name != "SG-01" || result.All[2].Filtered {
		t.Fatalf("All[2] = %+v, want active SG-01", result.All[2])
	}
}

// T-PRV-002: full preview pipeline returns separated groups and expanded members
func TestSourceFilterGroupRouteValidateReturnsPreviewStages(t *testing.T) {
	subURL := "https://sub.example.com/api"
	cfg := previewRuntimeConfig(t, subURL, "")
	fetcher := &fakeFetcher{responses: map[string][]byte{subURL: makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk.example.com:8388#HK-01",
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@hk2.example.com:8388#HK-02",
	)}}

	result, err := SourceFilterGroupRouteValidate(context.Background(), cfg, fetcher)
	if err != nil {
		t.Fatalf("SourceFilterGroupRouteValidate: %v", err)
	}

	if len(result.Group.RegionGroups) != 1 || result.Group.RegionGroups[0].Name != "HK" {
		t.Fatalf("RegionGroups = %+v, want HK", result.Group.RegionGroups)
	}
	if len(result.Group.ChainedGroups) != 1 || result.Group.ChainedGroups[0].Name != "SS-Chain" {
		t.Fatalf("ChainedGroups = %+v, want SS-Chain", result.Group.ChainedGroups)
	}
	if len(result.Group.NodeGroups) != 2 {
		t.Fatalf("NodeGroups = %+v, want region + chained groups", result.Group.NodeGroups)
	}
	if len(result.Route.ResolvedRouteGroups) != 1 {
		t.Fatalf("ResolvedRouteGroups = %+v, want one service group", result.Route.ResolvedRouteGroups)
	}
	var hasAllExpanded bool
	for _, member := range result.Route.ResolvedRouteGroups[0].Members {
		if member.Raw == "HK-01" && member.Origin == config.RouteMemberOriginAllExpanded {
			hasAllExpanded = true
		}
	}
	if !hasAllExpanded {
		t.Fatalf("resolved members missing @all-expanded HK-01: %+v", result.Route.ResolvedRouteGroups[0].Members)
	}
}

// T-PRV-009: preview pipeline stops on ValidateGraph error
func TestSourceFilterGroupRouteValidateStopsOnGraphError(t *testing.T) {
	subURL := "https://sub.example.com/api"
	cfg := previewRuntimeConfig(t, subURL, "")
	fetcher := &fakeFetcher{responses: map[string][]byte{subURL: makeSubResponse(
		"ss://YWVzLTI1Ni1jZmI6cGFzcw@sg.example.com:8388#SG-01",
	)}}

	_, err := SourceFilterGroupRouteValidate(context.Background(), cfg, fetcher)
	if err == nil {
		t.Fatal("SourceFilterGroupRouteValidate error = nil, want graph validation error")
	}
	var buildErr *errtype.BuildError
	if !errors.As(err, &buildErr) || buildErr.Code != errtype.CodeBuildValidationFailed {
		t.Fatalf("error = %T %[1]v, want build validation error", err)
	}
}

func previewRuntimeConfig(t *testing.T, subURL, exclude string) *config.RuntimeConfig {
	t.Helper()
	cfg := &config.Config{
		Sources: config.Sources{
			Subscriptions: []config.Subscription{{URL: subURL}},
			CustomProxies: []config.CustomProxy{
				customProxy("SS-Chain", "ss://YWVzLTI1Ni1nY206Y2hhaW5wYXNz@1.2.3.4:8388", &config.RelayThrough{Type: "all", Strategy: "select"}),
			},
		},
		Filters:  config.Filters{Exclude: exclude},
		Groups:   mustGroupsMap(t, `"HK": { match: "(HK)", strategy: select }`),
		Routing:  mustRoutingMap(t, `"proxy": ["HK", "SS-Chain", "@all", "DIRECT"]`),
		Rulesets: config.OrderedMap[[]string]{},
		Rules:    []string{},
		Fallback: "proxy",
	}
	rt, err := config.Prepare(cfg)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return rt
}
