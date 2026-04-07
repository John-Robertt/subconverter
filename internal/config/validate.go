package config

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// Validate performs static validation on a loaded Config.
// It collects all errors and returns them joined, or nil if valid.
func Validate(cfg *Config) error {
	var c collector

	// sources.subscriptions
	for i, sub := range cfg.Sources.Subscriptions {
		if sub.URL == "" {
			c.add(fmt.Sprintf("sources.subscriptions[%d].url", i), "required")
		}
	}

	// sources.custom_proxies
	seen := make(map[string]bool)
	for i, cp := range cfg.Sources.CustomProxies {
		prefix := fmt.Sprintf("sources.custom_proxies[%d]", i)

		if cp.Name == "" {
			c.add(prefix+".name", "required")
		}
		if cp.Type == "" {
			c.add(prefix+".type", "required")
		} else if cp.Type != "socks5" && cp.Type != "http" {
			c.add(prefix+".type", fmt.Sprintf("must be socks5 or http, got %q", cp.Type))
		}
		if cp.Server == "" {
			c.add(prefix+".server", "required")
		}
		if cp.Port <= 0 {
			c.add(prefix+".port", "must be positive")
		}

		if cp.Name != "" {
			if seen[cp.Name] {
				c.add(prefix+".name", fmt.Sprintf("duplicate custom proxy name %q", cp.Name))
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
			c.add(prefix+".match", "required")
		} else {
			c.compileRegex(prefix+".match", g.Match)
		}
		if g.Strategy == "" {
			c.add(prefix+".strategy", "required")
		} else if g.Strategy != "select" && g.Strategy != "url-test" {
			c.add(prefix+".strategy", fmt.Sprintf("must be select or url-test, got %q", g.Strategy))
		}
	}

	// fallback
	if cfg.Fallback == "" {
		c.add("fallback", "required")
	}

	return c.result()
}

func (c *collector) validateRelayThrough(rt *RelayThrough, prefix string) {
	switch rt.Type {
	case "group":
		if rt.Name == "" {
			c.add(prefix+".name", "required when type is group")
		}
	case "select":
		if rt.Match == "" {
			c.add(prefix+".match", "required when type is select")
		} else {
			c.compileRegex(prefix+".match", rt.Match)
		}
	case "all":
		// no additional fields required
	case "":
		c.add(prefix+".type", "required")
	default:
		c.add(prefix+".type", fmt.Sprintf("must be group, select, or all, got %q", rt.Type))
	}

	if rt.Strategy == "" {
		c.add(prefix+".strategy", "required")
	} else if rt.Strategy != "select" && rt.Strategy != "url-test" {
		c.add(prefix+".strategy", fmt.Sprintf("must be select or url-test, got %q", rt.Strategy))
	}
}

func (c *collector) compileRegex(field, pattern string) {
	if _, err := regexp.Compile(pattern); err != nil {
		c.add(field, fmt.Sprintf("invalid regex: %v", err))
	}
}

// collector accumulates validation errors.
type collector struct {
	errs []error
}

func (c *collector) add(field, message string) {
	c.errs = append(c.errs, &errtype.ConfigError{Field: field, Message: message})
}

func (c *collector) result() error {
	return errors.Join(c.errs...)
}
