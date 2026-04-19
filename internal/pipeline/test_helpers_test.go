package pipeline

import (
	"context"
	"testing"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
)

// Execute is kept as a test-only alias so existing stage-composition tests can
// stay focused on behavior while production code uses the renamed Build API.
func Execute(ctx context.Context, cfg *config.Config, fetcher fetch.Fetcher) (*model.Pipeline, error) {
	return Build(ctx, cfg, fetcher)
}

func groupFromConfig(cfg *config.Config, proxies []model.Proxy) (*GroupResult, error) {
	standalone, chainTemplates, err := convertCustomProxies(cfg.Sources.CustomProxies)
	if err != nil {
		return nil, err
	}
	allProxies := make([]model.Proxy, 0, len(proxies)+len(standalone))
	allProxies = append(allProxies, proxies...)
	allProxies = append(allProxies, standalone...)
	return Group(&SourceResult{
		Proxies:        allProxies,
		ChainTemplates: chainTemplates,
	}, &cfg.Groups)
}

func routeFromConfig(cfg *config.Config, gr *GroupResult) (*RouteResult, error) {
	return Route(&cfg.Routing, &cfg.Rulesets, cfg.Rules, cfg.Fallback, gr)
}

func customProxy(name, rawURL string, rt *config.RelayThrough) config.CustomProxy {
	return config.CustomProxy{
		Name:         name,
		URL:          rawURL,
		RelayThrough: rt,
	}
}

func requireGroupResult(t *testing.T, cfg *config.Config, proxies []model.Proxy) *GroupResult {
	t.Helper()
	result, err := groupFromConfig(cfg, proxies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}
