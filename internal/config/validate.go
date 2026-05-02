package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

func (c *collector) validateHTTPURL(loc diagnosticPath, rawURL string) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		c.addCode(loc, errtype.CodeConfigInvalidURL, fmt.Sprintf("URL 无效：%q", rawURL))
		return
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		c.addCode(loc, errtype.CodeConfigInvalidURL, fmt.Sprintf("必须以 http:// 或 https:// 开头，当前为 %q", rawURL))
	}
}

func (c *collector) validateBaseURL(loc diagnosticPath, rawURL string) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		c.addCode(loc, errtype.CodeConfigInvalidURL, fmt.Sprintf("URL 无效：%q", rawURL))
		return
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		c.addCode(loc, errtype.CodeConfigInvalidURL, fmt.Sprintf("必须以 http:// 或 https:// 开头，当前为 %q", rawURL))
		return
	}

	if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		c.addCode(loc, errtype.CodeConfigInvalidURL, "只能包含 scheme 和 host，不能包含 path、query 或 fragment")
	}
}

func (c *collector) validateTemplatePath(loc diagnosticPath, location string) {
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		c.validateHTTPURL(loc, location)
	}
	// Local paths are not validated here; OS will report errors at load time.
}

type diagnosticPath struct {
	section   string
	key       string
	index     *int
	valuePath string
}

func sectionPath(section string) diagnosticPath {
	return diagnosticPath{section: section}
}

func valuePath(section, valuePath string) diagnosticPath {
	return diagnosticPath{section: section, valuePath: valuePath}
}

func keyedPath(section, key, valuePath string) diagnosticPath {
	return diagnosticPath{section: section, key: key, valuePath: valuePath}
}

func keyedIndexedPath(section string, index int, key, valuePath string) diagnosticPath {
	idx := index
	return diagnosticPath{section: section, key: key, index: &idx, valuePath: valuePath}
}

func indexedPath(section string, index int, valuePath string) diagnosticPath {
	idx := index
	return diagnosticPath{section: section, index: &idx, valuePath: valuePath}
}

// collector accumulates validation errors.
type collector struct {
	errs []error
}

func (c *collector) add(loc diagnosticPath, message string) {
	c.addCode(loc, errtype.CodeConfigValidationFailed, message)
}

func (c *collector) addCode(loc diagnosticPath, code errtype.Code, message string) {
	c.errs = append(c.errs, &errtype.ConfigError{
		Code:      code,
		Section:   loc.section,
		Key:       loc.key,
		Index:     loc.index,
		ValuePath: loc.valuePath,
		Field:     displayPath(loc),
		Message:   message,
	})
}

func displayPath(loc diagnosticPath) string {
	if loc.section == "" {
		return ""
	}
	path := loc.section
	if loc.key != "" {
		path += "." + loc.key
	} else if loc.index != nil {
		path += fmt.Sprintf("[%d]", *loc.index)
	}
	if loc.valuePath != "" {
		if loc.key != "" && len(loc.valuePath) > 0 && loc.valuePath[0] == '[' {
			path += loc.valuePath
		} else if len(loc.valuePath) > 0 && loc.valuePath[0] == '[' {
			path += loc.valuePath
		} else {
			path += "." + loc.valuePath
		}
	}
	return path
}

func (c *collector) result() error {
	return errors.Join(c.errs...)
}
