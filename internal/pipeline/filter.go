package pipeline

import (
	"regexp"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// Filter executes pipeline stage 4: apply exclude regex to fetched nodes.
//
// Filtering covers all nodes sourced via remote fetch (Kind=KindSubscription
// or Kind=KindSnell). Custom proxies and any other kinds pass through
// unconditionally.
//
// If excludePattern is empty, all proxies pass through unchanged.
func Filter(proxies []model.Proxy, excludePattern string) ([]model.Proxy, error) {
	if excludePattern == "" {
		return proxies, nil
	}

	re, err := regexp.Compile(excludePattern)
	if err != nil {
		return nil, &errtype.BuildError{
			Code:    errtype.CodeBuildFilterRegexInvalid,
			Phase:   "filter",
			Message: "exclude 正则无效：" + err.Error(),
		}
	}

	result := make([]model.Proxy, 0, len(proxies))
	for _, p := range proxies {
		if isFetchedKind(p.Kind) && re.MatchString(p.Name) {
			continue
		}
		result = append(result, p)
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
