package errtype

import "fmt"

// ConfigError indicates invalid configuration: bad YAML syntax,
// missing required fields, illegal enum values, or uncompilable regexes.
type ConfigError struct {
	Field   string // config path, e.g. "groups.🇭🇰 Hong Kong.strategy"
	Message string
}

func (e *ConfigError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("config error [%s]: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("config error: %s", e.Message)
}

// FetchError indicates a failure to retrieve a remote subscription.
// URL must be sanitized (query params redacted) before storing.
type FetchError struct {
	URL     string // sanitized URL (query params redacted)
	Message string
	Cause   error
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("fetch error [%s]: %s", e.URL, e.Message)
}

func (e *FetchError) Unwrap() error {
	return e.Cause
}

// BuildError indicates a failure during pipeline construction
// (group building, route assembly, graph validation).
type BuildError struct {
	Phase   string // e.g. "group", "route", "validate"
	Message string
}

func (e *BuildError) Error() string {
	return fmt.Sprintf("build error [%s]: %s", e.Phase, e.Message)
}

// RenderError indicates a failure during output generation.
type RenderError struct {
	Format  string // "clash" or "surge"
	Message string
	Cause   error
}

func (e *RenderError) Error() string {
	return fmt.Sprintf("render error [%s]: %s", e.Format, e.Message)
}

func (e *RenderError) Unwrap() error {
	return e.Cause
}
