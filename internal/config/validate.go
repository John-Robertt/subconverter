package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

func (c *collector) validateHTTPURL(field, rawURL string) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		c.add(field, fmt.Sprintf("URL 无效：%q", rawURL))
		return
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		c.add(field, fmt.Sprintf("必须以 http:// 或 https:// 开头，当前为 %q", rawURL))
	}
}

func (c *collector) validateBaseURL(field, rawURL string) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		c.add(field, fmt.Sprintf("URL 无效：%q", rawURL))
		return
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		c.add(field, fmt.Sprintf("必须以 http:// 或 https:// 开头，当前为 %q", rawURL))
		return
	}

	if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		c.add(field, "只能包含 scheme 和 host，不能包含 path、query 或 fragment")
	}
}

func (c *collector) validateTemplatePath(field, location string) {
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		c.validateHTTPURL(field, location)
	}
	// Local paths are not validated here; OS will report errors at load time.
}

// collector accumulates validation errors.
type collector struct {
	errs []error
}

func (c *collector) add(field, message string) {
	c.errs = append(c.errs, &errtype.ConfigError{Code: errtype.CodeConfigValidationFailed, Field: field, Message: message})
}

func (c *collector) result() error {
	return errors.Join(c.errs...)
}
