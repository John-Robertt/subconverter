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
func Build(ctx context.Context, cfg *config.Config, fetcher fetch.Fetcher) (*model.Pipeline, error) {
	source, err := Source(ctx, cfg, fetcher)
	if err != nil {
		return nil, err
	}

	filtered, err := Filter(source.Proxies, cfg.Filters.Exclude)
	if err != nil {
		return nil, err
	}
	source.Proxies = filtered

	gr, err := Group(source, &cfg.Groups)
	if err != nil {
		return nil, err
	}

	rr, err := Route(&cfg.Routing, &cfg.Rulesets, cfg.Rules, cfg.Fallback, gr)
	if err != nil {
		return nil, err
	}

	return ValidateGraph(gr, rr)
}
