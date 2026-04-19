package pipeline

import (
	"context"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
)

// Build runs the format-agnostic build pipeline:
// Source → Filter → Group → Route → ValidateGraph.
// It returns the assembled Pipeline ready for target projection or rendering.
func Build(ctx context.Context, cfg *config.RuntimeConfig, fetcher fetch.Fetcher) (*model.Pipeline, error) {
	source, err := sourcePrepared(ctx, cfg.SourceInput(), cfg.StaticNamespace(), fetcher)
	if err != nil {
		return nil, err
	}

	filtered, err := filterCompiled(source.Proxies, cfg.FilterInput().ExcludePattern)
	if err != nil {
		return nil, err
	}
	source.Proxies = filtered

	gr, err := Group(source, cfg.GroupInput())
	if err != nil {
		return nil, err
	}

	routing, rulesets, rules, fallback := cfg.RouteInput()
	rr, err := Route(routing, rulesets, rules, fallback, gr)
	if err != nil {
		return nil, err
	}

	return ValidateGraph(gr, rr)
}
