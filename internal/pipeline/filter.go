package pipeline

import (
	"regexp"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/model"
)

// Filter executes pipeline stage 4: apply exclude regex to subscription nodes.
//
// Only nodes with Kind=KindSubscription are subject to filtering. Custom proxies
// and any other kinds pass through unconditionally.
//
// If excludePattern is empty, all proxies pass through unchanged.
func Filter(proxies []model.Proxy, excludePattern string) ([]model.Proxy, error) {
	if excludePattern == "" {
		return proxies, nil
	}

	re, err := regexp.Compile(excludePattern)
	if err != nil {
		return nil, &errtype.BuildError{
			Phase:   "filter",
			Message: "invalid exclude regex: " + err.Error(),
		}
	}

	result := make([]model.Proxy, 0, len(proxies))
	for _, p := range proxies {
		if p.Kind == model.KindSubscription && re.MatchString(p.Name) {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}
