package pipeline

import (
	"context"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
)

// GroupPreviewStageResult carries the intermediate outputs needed by the
// group preview API without exposing pipeline internals to HTTP handlers.
type GroupPreviewStageResult struct {
	Filter   *FilterResult
	Group    *GroupResult
	Route    *RouteResult
	Pipeline *model.Pipeline
}

// SourceAndFilter executes the Source + Filter stages and keeps both included
// and excluded nodes for preview diagnostics.
func SourceAndFilter(ctx context.Context, cfg *config.RuntimeConfig, fetcher fetch.Fetcher) (*FilterResult, error) {
	source, err := sourcePrepared(ctx, cfg.SourceInput(), cfg.StaticNamespace(), fetcher)
	if err != nil {
		return nil, err
	}
	return filterCompiledDetailed(source.Proxies, cfg.FilterInput().ExcludePattern)
}

// SourceFilterGroupRouteValidate executes the preview path through graph
// validation. It returns no partial group result when ValidateGraph fails.
func SourceFilterGroupRouteValidate(ctx context.Context, cfg *config.RuntimeConfig, fetcher fetch.Fetcher) (*GroupPreviewStageResult, error) {
	source, err := sourcePrepared(ctx, cfg.SourceInput(), cfg.StaticNamespace(), fetcher)
	if err != nil {
		return nil, err
	}

	filter, err := filterCompiledDetailed(source.Proxies, cfg.FilterInput().ExcludePattern)
	if err != nil {
		return nil, err
	}
	source.Proxies = filter.Included

	gr, err := Group(source, cfg.GroupInput())
	if err != nil {
		return nil, err
	}

	routing, rulesets, rules, fallback := cfg.RouteInput()
	rr, err := Route(routing, rulesets, rules, fallback, gr)
	if err != nil {
		return nil, err
	}

	p, err := ValidateGraph(gr, rr)
	if err != nil {
		return nil, err
	}

	return &GroupPreviewStageResult{
		Filter:   filter,
		Group:    gr,
		Route:    rr,
		Pipeline: p,
	}, nil
}
