package pipeline

import (
	"regexp"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// FilteredProxy 是一个节点加上它的过滤状态，过滤状态只供预览使用。
type FilteredProxy struct {
	Proxy    model.Proxy
	Filtered bool
}

// FilterResult 同时保留过滤判定的两侧结果。生成配置只使用 Included；
// 预览接口还会展示 Excluded 和 All。
type FilterResult struct {
	Included []model.Proxy
	Excluded []model.Proxy
	All      []FilteredProxy
}

// Filter 执行流水线第 4 阶段：对拉取类节点应用 exclude 正则。
//
// 过滤覆盖所有通过远程拉取得到的节点（KindSubscription / KindSnell /
// KindVLess，见 isFetchedKind）。自定义节点和其他类型一律直接放行。
//
// excludePattern 为空时，所有节点原样通过。
func Filter(proxies []model.Proxy, excludePattern string) ([]model.Proxy, error) {
	re, err := regexp.Compile(excludePattern)
	if excludePattern == "" {
		re = nil
	} else if err != nil {
		return nil, &errtype.BuildError{
			Code:    errtype.CodeBuildFilterRegexInvalid,
			Phase:   "filter",
			Message: "exclude 正则无效：" + err.Error(),
		}
	}

	result, err := filterCompiledDetailed(proxies, re)
	if err != nil {
		return nil, err
	}
	return result.Included, nil
}

func filterCompiled(proxies []model.Proxy, excludePattern *regexp.Regexp) ([]model.Proxy, error) {
	result, err := filterCompiledDetailed(proxies, excludePattern)
	if err != nil {
		return nil, err
	}
	return result.Included, nil
}

func filterCompiledDetailed(proxies []model.Proxy, excludePattern *regexp.Regexp) (*FilterResult, error) {
	result := &FilterResult{
		Included: make([]model.Proxy, 0, len(proxies)),
		Excluded: []model.Proxy{},
		All:      make([]FilteredProxy, 0, len(proxies)),
	}
	for _, p := range proxies {
		if excludePattern != nil && isFetchedKind(p.Kind) && excludePattern.MatchString(p.Name) {
			result.Excluded = append(result.Excluded, p)
			result.All = append(result.All, FilteredProxy{Proxy: p, Filtered: true})
			continue
		}
		result.Included = append(result.Included, p)
		result.All = append(result.All, FilteredProxy{Proxy: p})
	}
	return result, nil
}

// isFetchedKind 判断某个节点 Kind 是否来自远程拉取（订阅、Snell 或 VLESS
// 来源）。这些类型参与名称过滤、区域分组正则匹配，并可作为链式上游候选。
// @all 另行从非链式的原始节点集合计算。
func isFetchedKind(k model.ProxyKind) bool {
	return k == model.KindSubscription || k == model.KindSnell || k == model.KindVLess
}
