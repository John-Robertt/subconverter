package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/proxyparse"
)

// Validate performs static validation on a loaded Config.
// It collects all errors and returns them joined, or nil if valid.
func Validate(cfg *Config) error {
	var c collector

	// sources.subscriptions
	for i, sub := range cfg.Sources.Subscriptions {
		if sub.URL == "" {
			c.add(fmt.Sprintf("sources.subscriptions[%d].url", i), "必填")
		} else {
			c.validateHTTPURL(fmt.Sprintf("sources.subscriptions[%d].url", i), sub.URL)
		}
	}

	// sources.snell
	for i, s := range cfg.Sources.Snell {
		field := fmt.Sprintf("sources.snell[%d].url", i)
		if s.URL == "" {
			c.add(field, "必填")
		} else {
			c.validateHTTPURL(field, s.URL)
		}
	}

	// sources.vless
	for i, s := range cfg.Sources.VLess {
		field := fmt.Sprintf("sources.vless[%d].url", i)
		if s.URL == "" {
			c.add(field, "必填")
		} else {
			c.validateHTTPURL(field, s.URL)
		}
	}

	// sources.custom_proxies
	seen := make(map[string]bool)
	for i, cp := range cfg.Sources.CustomProxies {
		prefix := fmt.Sprintf("sources.custom_proxies[%d]", i)

		if cp.Name == "" {
			c.add(prefix+".name", "必填")
		}
		if cp.URL == "" {
			c.add(prefix+".url", "必填")
		} else {
			parsed, err := proxyparse.ParseURL(cp.URL)
			if err != nil {
				c.add(prefix+".url", err.Error())
			} else if parsed.Type == "ss" {
				if parsed.Params["cipher"] == "" {
					c.add(prefix+".url", "SS URI 缺少加密方式（cipher）")
				}
				if parsed.Params["password"] == "" {
					c.add(prefix+".url", "SS URI 缺少密码")
				}
			}
		}

		if cp.Name != "" {
			if seen[cp.Name] {
				c.add(prefix+".name", fmt.Sprintf("自定义代理名 %q 重复", cp.Name))
			}
			seen[cp.Name] = true
		}

		if cp.RelayThrough != nil {
			c.validateRelayThrough(cp.RelayThrough, prefix+".relay_through")
		}
	}

	// filters
	if cfg.Filters.Exclude != "" {
		c.compileRegex("filters.exclude", cfg.Filters.Exclude)
	}

	// groups
	for k, g := range cfg.Groups.Entries() {
		prefix := fmt.Sprintf("groups.%s", k)
		if g.Match == "" {
			c.add(prefix+".match", "必填")
		} else {
			c.compileRegex(prefix+".match", g.Match)
		}
		if g.Strategy == "" {
			c.add(prefix+".strategy", "必填")
		} else if g.Strategy != "select" && g.Strategy != "url-test" {
			c.add(prefix+".strategy", fmt.Sprintf("必须为 select 或 url-test，当前为 %q", g.Strategy))
		}
	}

	// rulesets
	for k, urls := range cfg.Rulesets.Entries() {
		prefix := fmt.Sprintf("rulesets.%s", k)
		if len(urls) == 0 {
			c.add(prefix, "至少需要一个 URL")
			continue
		}
		for i, rawURL := range urls {
			field := fmt.Sprintf("%s[%d]", prefix, i)
			if rawURL == "" {
				c.add(field, "必填")
				continue
			}
			c.validateHTTPURL(field, rawURL)
		}
	}

	// routing: @all and @auto are mutually exclusive within the same entry
	for k, members := range cfg.Routing.Entries() {
		hasAll, autoCount := false, 0
		for _, m := range members {
			if m == "@all" {
				hasAll = true
			}
			if m == "@auto" {
				autoCount++
			}
		}
		if hasAll && autoCount > 0 {
			c.add(fmt.Sprintf("routing.%s", k), "@all 和 @auto 不能同时使用")
		}
		if autoCount > 1 {
			c.add(fmt.Sprintf("routing.%s", k), "@auto 不能重复使用")
		}
	}

	// fallback
	if cfg.Fallback == "" {
		c.add("fallback", "必填")
	}

	// base_url
	if cfg.BaseURL != "" {
		c.validateBaseURL("base_url", cfg.BaseURL)
	}

	// templates
	if cfg.Templates.Clash != "" {
		c.validateTemplatePath("templates.clash", cfg.Templates.Clash)
	}
	if cfg.Templates.Surge != "" {
		c.validateTemplatePath("templates.surge", cfg.Templates.Surge)
	}

	return c.result()
}

func (c *collector) validateRelayThrough(rt *RelayThrough, prefix string) {
	switch rt.Type {
	case "group":
		if rt.Name == "" {
			c.add(prefix+".name", "type=group 时必填")
		}
	case "select":
		if rt.Match == "" {
			c.add(prefix+".match", "type=select 时必填")
		} else {
			c.compileRegex(prefix+".match", rt.Match)
		}
	case "all":
		// no additional fields required
	case "":
		c.add(prefix+".type", "必填")
	default:
		c.add(prefix+".type", fmt.Sprintf("必须为 group、select 或 all，当前为 %q", rt.Type))
	}

	if rt.Strategy == "" {
		c.add(prefix+".strategy", "必填")
	} else if rt.Strategy != "select" && rt.Strategy != "url-test" {
		c.add(prefix+".strategy", fmt.Sprintf("必须为 select 或 url-test，当前为 %q", rt.Strategy))
	}
}

func (c *collector) compileRegex(field, pattern string) {
	if _, err := regexp.Compile(pattern); err != nil {
		c.add(field, fmt.Sprintf("正则表达式无效：%v", err))
	}
}

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
