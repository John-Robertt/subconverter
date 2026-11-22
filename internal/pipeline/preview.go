package pipeline

import (
	"context"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
)

// GroupPreviewStageResult 携带分组预览接口所需的中间输出，
// 同时不把流水线内部结构暴露给 HTTP 处理器。
type GroupPreviewStageResult struct {
	Filter   *FilterResult
	Group    *GroupResult
	Route    *RouteResult
	Pipeline *model.Pipeline
}

// SourceAndFilter 执行 Source + Filter 阶段，同时保留保留下来的节点和被排除的节点，
// 供预览诊断使用。
func SourceAndFilter(ctx context.Context, cfg *config.RuntimeConfig, fetcher fetch.Fetcher) (*FilterResult, error) {
	source, err := sourcePrepared(ctx, cfg.SourceInput(), fetcher)
	if err != nil {
		return nil, err
	}
	return filterCompiledDetailed(source.Proxies, cfg.FilterInput().ExcludePattern)
}

// SourceFilterGroupRouteValidate 执行预览路径，一直到图校验为止。
// ValidateGraph 失败时不返回部分分组结果。
func SourceFilterGroupRouteValidate(ctx context.Context, cfg *config.RuntimeConfig, fetcher fetch.Fetcher) (*GroupPreviewStageResult, error) {
	source, err := sourcePrepared(ctx, cfg.SourceInput(), fetcher)
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
