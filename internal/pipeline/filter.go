package pipeline

import (
	"regexp"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// FilteredProxy is a proxy plus its preview-only filtering status.
type FilteredProxy struct {
	Proxy    model.Proxy
	Filtered bool
}

// FilterResult keeps both sides of the filter decision. Generation consumes
// Included; preview APIs also expose Excluded and All.
type FilterResult struct {
	Included []model.Proxy
	Excluded []model.Proxy
	All      []FilteredProxy
}

// Filter executes pipeline stage 4: apply exclude regex to fetched nodes.
//
// Filtering covers all nodes sourced via remote fetch (KindSubscription /
// KindSnell / KindVLess — see isFetchedKind). Custom proxies and any other
// kinds pass through unconditionally.
//
// If excludePattern is empty, all proxies pass through unchanged.
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

// isFetchedKind reports whether a proxy Kind was sourced via remote fetch
// (subscription, Snell, or VLESS source). These kinds participate in name
// filtering, region-group regex matching, chain upstream candidacy, and
// @all expansion.
func isFetchedKind(k model.ProxyKind) bool {
	return k == model.KindSubscription || k == model.KindSnell || k == model.KindVLess
}
