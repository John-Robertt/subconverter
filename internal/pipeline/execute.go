package pipeline

import (
	"context"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
)

// Build 运行与输出格式无关的构建流水线：
// Source → Filter → Group → Route → ValidateGraph。
// 它返回组装好的 Pipeline，供 target 包做目标格式投影。渲染器只使用
// target 包产出的 TargetPipeline。
func Build(ctx context.Context, cfg *config.RuntimeConfig, fetcher fetch.Fetcher) (*model.Pipeline, error) {
	source, err := sourcePrepared(ctx, cfg.SourceInput(), fetcher)
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
