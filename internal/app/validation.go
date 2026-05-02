package app

import (
	"errors"
	"fmt"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/errtype"
)

var requiredFetchOrder = []string{"subscriptions", "snell", "vless"}

func prepareAdminConfig(cfg *config.Config, validateFetchOrder bool) (*config.RuntimeConfig, error) {
	var errs []error
	if cfg != nil && cfg.Groups.Len() == 0 {
		errs = append(errs, &errtype.ConfigError{
			Code:    errtype.CodeConfigValidationFailed,
			Section: "groups",
			Message: "至少需要一个地区节点组",
		})
	}
	if validateFetchOrder {
		errs = append(errs, validateFetchOrderForAPI(cfg)...)
	}

	rt, err := config.Prepare(cfg)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return rt, nil
}

func validateFetchOrderForAPI(cfg *config.Config) []error {
	if cfg == nil || len(cfg.Sources.FetchOrder) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(cfg.Sources.FetchOrder))
	for _, key := range cfg.Sources.FetchOrder {
		if !isRequiredFetchKey(key) {
			return []error{invalidFetchOrder(fmt.Sprintf("fetch_order 包含未知来源类型 %q", key))}
		}
		if seen[key] {
			return []error{invalidFetchOrder(fmt.Sprintf("fetch_order 中来源类型 %q 重复", key))}
		}
		seen[key] = true
	}
	for _, key := range requiredFetchOrder {
		if !seen[key] {
			return []error{invalidFetchOrder(fmt.Sprintf("fetch_order 缺少来源类型 %q", key))}
		}
	}
	return nil
}

func isRequiredFetchKey(key string) bool {
	for _, required := range requiredFetchOrder {
		if key == required {
			return true
		}
	}
	return false
}

func invalidFetchOrder(message string) error {
	return &errtype.ConfigError{
		Code:      errtype.CodeConfigInvalidFetchOrder,
		Section:   "sources",
		ValuePath: "fetch_order",
		Message:   message,
	}
}
