package errtype

import (
	"errors"
	"fmt"
	"strings"
)

// Code 是错误情形的稳定标识，供程序识别。
type Code string

const (
	CodeConfigLoadFailed        Code = "config_load_failed"
	CodeConfigNotInitialized    Code = "config_not_initialized"
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
	CodeFetchSubscriptionFormatInvalid Code = "fetch_subscription_format_invalid"
	CodeFetchSubscriptionEmpty         Code = "fetch_subscription_empty"

	CodeResourceLocalReadFailed    Code = "resource_local_read_failed"
	CodeResourceContentWriteFailed Code = "resource_content_write_failed"

	CodeBuildFilterRegexInvalid     Code = "build_filter_regex_invalid"
	CodeBuildGroupRegexInvalid      Code = "build_group_regex_invalid"
	CodeBuildRelayGroupMissing      Code = "build_relay_group_missing"
	CodeBuildRelayRegexInvalid      Code = "build_relay_regex_invalid"
	CodeBuildRelayTypeInvalid       Code = "build_relay_type_invalid"
	CodeBuildSSURIInvalid           Code = "build_ss_uri_invalid"
	CodeBuildAnyTLSURIInvalid       Code = "build_anytls_uri_invalid"
	CodeBuildAnyTLSLineInvalid      Code = "build_anytls_line_invalid"
	CodeBuildSnellLineInvalid       Code = "build_snell_line_invalid"
	CodeBuildVLessURIInvalid        Code = "build_vless_uri_invalid"
	CodeBuildVLessSourceLineInvalid Code = "build_vless_source_line_invalid"
	CodeBuildValidationFailed       Code = "build_validation_failed"

	CodeRenderTemplateParseFailed Code = "render_template_parse_failed"
	CodeRenderTemplateInvalid     Code = "render_template_invalid"
	CodeRenderYAMLEncodeFailed    Code = "render_yaml_encode_failed"
	CodeRenderYAMLFinalizeFailed  Code = "render_yaml_finalize_failed"
	CodeRenderClashProxyInvalid   Code = "render_clash_proxy_invalid"
	CodeRenderSurgeProxyInvalid   Code = "render_surge_proxy_invalid"
	CodeRenderLoonProxyInvalid    Code = "render_loon_proxy_invalid"

	CodeTargetClashFallbackEmpty     Code = "target_clash_fallback_empty"
	CodeTargetSurgeFallbackEmpty     Code = "target_surge_fallback_empty"
	CodeTargetClashProjectionInvalid Code = "target_clash_projection_invalid"
	CodeTargetSurgeProjectionInvalid Code = "target_surge_projection_invalid"
	CodeTargetLoonFallbackEmpty      Code = "target_loon_fallback_empty"
	CodeTargetLoonProjectionInvalid  Code = "target_loon_projection_invalid"
)

var (
	ErrConfigSourceReadonly  = errors.New("配置源只读")
	ErrConfigFileNotWritable = errors.New("配置文件不可写")
	ErrReloadInProgress      = errors.New("热重载正在进行中")
)

// ConfigError 表示配置不可用或无效：尚未初始化、YAML 格式错误，
// 或声明的字段违反配置约束。
type ConfigError struct {
	Code      Code
	Section   string
	Key       string
	Index     *int
	ValuePath string
	Field     string // 已弃用：v1 文本错误使用的展示路径。
	Message   string
}

func (e *ConfigError) Error() string {
	if path := e.DisplayPath(); path != "" {
		return fmt.Sprintf("配置错误 [%s]：%s", path, e.Message)
	}
	return fmt.Sprintf("配置错误：%s", e.Message)
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

// RevisionConflictError 表示带条件的配置写入使用了过期的 config_revision。
// CurrentConfigRevision 可以安全地放进 API 响应。
type RevisionConflictError struct {
	CurrentConfigRevision string
}

func (e *RevisionConflictError) Error() string {
	return "配置版本冲突"
}

// FetchError 表示拉取远程 HTTP(S) 资源失败。
// 保存 URL 前必须先脱敏（隐去查询参数）。
type FetchError struct {
	Code    Code
	URL     string // 脱敏后的 URL（已隐去查询参数）
	Message string
	Cause   error
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("拉取失败 [%s]：%s", e.URL, e.Message)
}

func (e *FetchError) Unwrap() error {
	return e.Cause
}

// ResourceError 表示读取或写入本地资源失败。
type ResourceError struct {
	Code     Code
	Location string
	Message  string
	Cause    error
}

func (e *ResourceError) Error() string {
	if e.Location != "" {
		return fmt.Sprintf("资源读取失败 [%s]：%s", e.Location, e.Message)
	}
	return fmt.Sprintf("资源读取失败：%s", e.Message)
}

func (e *ResourceError) Unwrap() error {
	return e.Cause
}

// BuildError 表示流水线构建阶段失败
// （解析来源、构建分组、组装路由、校验关系图）。
type BuildError struct {
	Code    Code
	Phase   string // 例如 "group"、"route"、"validate"
	Message string
	Cause   error
}

func (e *BuildError) Error() string {
	return fmt.Sprintf("构建错误 [%s]：%s", e.Phase, e.Message)
}

func (e *BuildError) Unwrap() error {
	return e.Cause
}

// TargetError 表示渲染开始前、按目标格式投影时失败。
type TargetError struct {
	Code    Code
	Format  string // "clash"、"surge" 或 "loon"
	Message string
	Cause   error
}

func (e *TargetError) Error() string {
	return fmt.Sprintf("目标投影错误 [%s]：%s", e.Format, e.Message)
}

func (e *TargetError) Unwrap() error {
	return e.Cause
}

// RenderError 表示目标投影成功之后，在序列化输出或合并模板时失败。
type RenderError struct {
	Code    Code
	Format  string // "clash"、"surge" 或 "loon"
	Message string
	Cause   error
}

func (e *RenderError) Error() string {
	return fmt.Sprintf("渲染错误 [%s]：%s", e.Format, e.Message)
}

func (e *RenderError) Unwrap() error {
	return e.Cause
}
