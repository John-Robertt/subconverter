package errtype

import "fmt"

// Code is a stable machine-readable identifier for an error condition.
type Code string

const (
	CodeConfigLoadFailed       Code = "config_load_failed"
	CodeConfigYAMLInvalid      Code = "config_yaml_invalid"
	CodeConfigValidationFailed Code = "config_validation_failed"

	CodeFetchRequestURLInvalid         Code = "fetch_request_url_invalid"
	CodeFetchRequestFailed             Code = "fetch_request_failed"
	CodeFetchStatusInvalid             Code = "fetch_status_invalid"
	CodeFetchBodyReadFailed            Code = "fetch_body_read_failed"
	CodeFetchFetcherRequired           Code = "fetch_fetcher_required"
	CodeFetchSubscriptionBase64Invalid Code = "fetch_subscription_base64_invalid"
	CodeFetchSubscriptionEmpty         Code = "fetch_subscription_empty"

	CodeBuildFilterRegexInvalid Code = "build_filter_regex_invalid"
	CodeBuildGroupRegexInvalid  Code = "build_group_regex_invalid"
	CodeBuildRelayGroupMissing  Code = "build_relay_group_missing"
	CodeBuildRelayRegexInvalid  Code = "build_relay_regex_invalid"
	CodeBuildRelayTypeInvalid   Code = "build_relay_type_invalid"
	CodeBuildCustomNameConflict Code = "build_custom_name_conflict"
	CodeBuildRuleFormatInvalid  Code = "build_rule_format_invalid"
	CodeBuildSSURIInvalid       Code = "build_ss_uri_invalid"
	CodeBuildValidationFailed   Code = "build_validation_failed"

	CodeRenderTemplateParseFailed Code = "render_template_parse_failed"
	CodeRenderTemplateInvalid     Code = "render_template_invalid"
	CodeRenderYAMLEncodeFailed    Code = "render_yaml_encode_failed"
	CodeRenderYAMLFinalizeFailed  Code = "render_yaml_finalize_failed"
	CodeRenderSurgeProxyInvalid   Code = "render_surge_proxy_invalid"
)

// ConfigError indicates invalid configuration: bad YAML syntax,
// missing required fields, illegal enum values, or uncompilable regexes.
type ConfigError struct {
	Code    Code
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
	Code    Code
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
	Code    Code
	Phase   string // e.g. "group", "route", "validate"
	Message string
}

func (e *BuildError) Error() string {
	return fmt.Sprintf("build error [%s]: %s", e.Phase, e.Message)
}

// RenderError indicates a failure during output generation.
type RenderError struct {
	Code    Code
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
