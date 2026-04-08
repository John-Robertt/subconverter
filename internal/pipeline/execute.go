package pipeline

import (
	"context"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
)

// Execute runs the full pipeline: Source → Filter → Group → Route → ValidateGraph.
// It returns the assembled Pipeline ready for rendering, or the first stage error.
func Execute(ctx context.Context, cfg *config.Config, fetcher fetch.Fetcher) (*model.Pipeline, error) {
	proxies, err := Source(ctx, cfg, fetcher)
	if err != nil {
		return nil, err
	}

	filtered, err := Filter(proxies, cfg.Filters.Exclude)
	if err != nil {
		return nil, err
	}

	gr, err := Group(filtered, cfg)
	if err != nil {
		return nil, err
	}

	rr, err := Route(cfg, gr.AllProxies)
	if err != nil {
		return nil, err
	}

	return ValidateGraph(gr, rr)
}
