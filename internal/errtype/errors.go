package errtype

import (
	"errors"
	"fmt"
	"strings"
)

// Code is a stable machine-readable identifier for an error condition.
type Code string

const (
	CodeConfigLoadFailed        Code = "config_load_failed"
	CodeConfigYAMLInvalid       Code = "config_yaml_invalid"
	CodeConfigValidationFailed  Code = "config_validation_failed"
	CodeConfigRequired          Code = "required"
	CodeConfigInvalidURL        Code = "invalid_url"
	CodeConfigInvalidRegex      Code = "invalid_regex"
	CodeConfigInvalidEnum       Code = "invalid_enum"
	CodeConfigInvalidReference  Code = "invalid_reference"
	CodeConfigDuplicateName     Code = "duplicate_name"
	CodeConfigInvalidRule       Code = "invalid_rule"
	CodeConfigInvalidFetchOrder Code = "invalid_fetch_order"

	CodeFetchRequestURLInvalid         Code = "fetch_request_url_invalid"
	CodeFetchRequestFailed             Code = "fetch_request_failed"
	CodeFetchStatusInvalid             Code = "fetch_status_invalid"
	CodeFetchBodyReadFailed            Code = "fetch_body_read_failed"
	CodeFetchFetcherRequired           Code = "fetch_fetcher_required"
	CodeFetchSubscriptionBase64Invalid Code = "fetch_subscription_base64_invalid"
	CodeFetchSubscriptionEmpty         Code = "fetch_subscription_empty"

	CodeResourceLocalReadFailed Code = "resource_local_read_failed"

	CodeBuildFilterRegexInvalid     Code = "build_filter_regex_invalid"
	CodeBuildGroupRegexInvalid      Code = "build_group_regex_invalid"
	CodeBuildRelayGroupMissing      Code = "build_relay_group_missing"
	CodeBuildRelayRegexInvalid      Code = "build_relay_regex_invalid"
	CodeBuildRelayTypeInvalid       Code = "build_relay_type_invalid"
	CodeBuildCustomNameConflict     Code = "build_custom_name_conflict"
	CodeBuildRuleFormatInvalid      Code = "build_rule_format_invalid"
	CodeBuildSSURIInvalid           Code = "build_ss_uri_invalid"
	CodeBuildSnellLineInvalid       Code = "build_snell_line_invalid"
	CodeBuildVLessURIInvalid        Code = "build_vless_uri_invalid"
	CodeBuildVLessSourceLineInvalid Code = "build_vless_source_line_invalid"
	CodeBuildValidationFailed       Code = "build_validation_failed"

	CodeRenderTemplateParseFailed Code = "render_template_parse_failed"
	CodeRenderTemplateInvalid     Code = "render_template_invalid"
	CodeRenderYAMLEncodeFailed    Code = "render_yaml_encode_failed"
	CodeRenderYAMLFinalizeFailed  Code = "render_yaml_finalize_failed"
	CodeRenderSurgeProxyInvalid   Code = "render_surge_proxy_invalid"

	CodeTargetClashFallbackEmpty     Code = "target_clash_fallback_empty"
	CodeTargetSurgeFallbackEmpty     Code = "target_surge_fallback_empty"
	CodeTargetClashProjectionInvalid Code = "target_clash_projection_invalid"
	CodeTargetSurgeProjectionInvalid Code = "target_surge_projection_invalid"
)

var (
	ErrConfigSourceReadonly  = errors.New("config source is read-only")
	ErrConfigFileNotWritable = errors.New("config file is not writable")
	ErrReloadInProgress      = errors.New("reload already in progress")
)

// ConfigError indicates invalid configuration: bad YAML syntax,
// missing required fields, illegal enum values, or uncompilable regexes.
type ConfigError struct {
	Code      Code
	Section   string
	Key       string
	Index     *int
	ValuePath string
	Field     string // deprecated display path for v1 text errors.
	Message   string
}

func (e *ConfigError) Error() string {
	if path := e.DisplayPath(); path != "" {
		return fmt.Sprintf("config error [%s]: %s", path, e.Message)
	}
	return fmt.Sprintf("config error: %s", e.Message)
}

func (e *ConfigError) DisplayPath() string {
	if e.Field != "" {
		return e.Field
	}
	if e.Section == "" {
		return ""
	}
	path := e.Section
	if e.Key != "" {
		path += "." + e.Key
	} else if e.Index != nil {
		path += fmt.Sprintf("[%d]", *e.Index)
	}
	if e.ValuePath != "" {
		if strings.HasPrefix(e.ValuePath, "[") {
			path += e.ValuePath
		} else if e.Key != "" || e.Index == nil {
			path += "." + e.ValuePath
		} else {
			path += "." + e.ValuePath
		}
	}
	return path
}

// RevisionConflictError indicates that a conditional config write used a stale
// config_revision. CurrentConfigRevision is safe to return in API responses.
type RevisionConflictError struct {
	CurrentConfigRevision string
}

func (e *RevisionConflictError) Error() string {
	return "config revision conflict"
}

// FetchError indicates a failure to retrieve a remote HTTP(S) resource.
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

// ResourceError indicates a failure to read a local resource.
type ResourceError struct {
	Code     Code
	Location string
	Message  string
	Cause    error
}

func (e *ResourceError) Error() string {
	if e.Location != "" {
		return fmt.Sprintf("resource error [%s]: %s", e.Location, e.Message)
	}
	return fmt.Sprintf("resource error: %s", e.Message)
}

func (e *ResourceError) Unwrap() error {
	return e.Cause
}

// BuildError indicates a failure during pipeline construction
// (source parsing, group building, route assembly, graph validation).
type BuildError struct {
	Code    Code
	Phase   string // e.g. "group", "route", "validate"
	Message string
	Cause   error
}

func (e *BuildError) Error() string {
	return fmt.Sprintf("build error [%s]: %s", e.Phase, e.Message)
}

func (e *BuildError) Unwrap() error {
	return e.Cause
}

// TargetError indicates a failure during target-format projection, before
// rendering starts.
type TargetError struct {
	Code    Code
	Format  string // "clash" or "surge"
	Message string
	Cause   error
}

func (e *TargetError) Error() string {
	return fmt.Sprintf("target error [%s]: %s", e.Format, e.Message)
}

func (e *TargetError) Unwrap() error {
	return e.Cause
}

// RenderError indicates a failure during output serialization or template
// merging after target projection has already succeeded.
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
