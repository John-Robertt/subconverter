package errtype

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// T-ERR-LOON-001：Loon 投影和渲染的错误码保留各自的阶段与格式。
func TestErrors_LoonCodes(t *testing.T) {
	for _, code := range []Code{CodeTargetLoonFallbackEmpty, CodeTargetLoonProjectionInvalid} {
		e := &TargetError{Code: code, Format: "loon", Message: "输出不可用"}
		if e.Code != code || e.Error() != "目标投影错误 [loon]：输出不可用" {
			t.Fatalf("code=%q error=%q", e.Code, e.Error())
		}
	}
	e := &RenderError{Code: CodeRenderLoonProxyInvalid, Format: "loon", Message: "代理无效"}
	if e.Code != CodeRenderLoonProxyInvalid || e.Error() != "渲染错误 [loon]：代理无效" {
		t.Fatalf("code=%q error=%q", e.Code, e.Error())
	}
}

// T-ERR-001：ConfigError 按 section/key/field 格式化
func TestConfigError(t *testing.T) {
	e := &ConfigError{Code: CodeConfigValidationFailed, Field: "groups.HK.strategy", Message: "必须为 select 或 url-test"}
	want := `配置错误 [groups.HK.strategy]：必须为 select 或 url-test`
	if got := e.Error(); got != want {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
	if e.Code != CodeConfigValidationFailed {
		t.Errorf("code = %q，期望 %q", e.Code, CodeConfigValidationFailed)
	}
}

// T-ERR-002：ConfigError 没有 field 时的格式
func TestConfigErrorNoField(t *testing.T) {
	e := &ConfigError{Code: CodeConfigYAMLInvalid, Message: "YAML 解析失败"}
	want := `配置错误：YAML 解析失败`
	if got := e.Error(); got != want {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
}

// T-ERR-003：FetchError 包含 URL 和消息
func TestFetchError(t *testing.T) {
	e := &FetchError{Code: CodeFetchRequestFailed, URL: "https://example.com/sub?token=***", Message: "请求上游失败：timeout", Cause: io.ErrUnexpectedEOF}
	want := `拉取失败 [https://example.com/sub?token=***]：请求上游失败：timeout`
	if got := e.Error(); got != want {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
}

// T-ERR-004：FetchError 可以解包出 Cause
func TestFetchErrorUnwrap(t *testing.T) {
	e := &FetchError{Code: CodeFetchRequestFailed, URL: "https://example.com/sub", Message: "请求上游失败：timeout", Cause: io.ErrUnexpectedEOF}
	if !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Error("FetchError 应能解包出它的 Cause")
	}
}

// T-ERR-005：ResourceError 包含路径和消息
func TestResourceError(t *testing.T) {
	e := &ResourceError{Code: CodeResourceLocalReadFailed, Location: "/tmp/base.yaml", Message: "no such file or directory", Cause: io.ErrUnexpectedEOF}
	want := `资源读取失败 [/tmp/base.yaml]：no such file or directory`
	if got := e.Error(); got != want {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
}

// T-ERR-006：ResourceError 可以解包出 Cause
func TestResourceErrorUnwrap(t *testing.T) {
	e := &ResourceError{Code: CodeResourceLocalReadFailed, Location: "/tmp/base.yaml", Message: "permission denied", Cause: io.ErrUnexpectedEOF}
	if !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Error("ResourceError 应能解包出它的 Cause")
	}
}

// T-ERR-007：BuildError 包含阶段和消息
func TestBuildError(t *testing.T) {
	e := &BuildError{Code: CodeBuildValidationFailed, Phase: "group", Message: "链式节点组为空"}
	want := `构建错误 [group]：链式节点组为空`
	if got := e.Error(); got != want {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
}

// T-ERR-008：BuildError 可以解包出 Cause
func TestBuildErrorUnwrap(t *testing.T) {
	e := &BuildError{Code: CodeBuildValidationFailed, Phase: "group", Message: "链式节点组为空", Cause: io.ErrUnexpectedEOF}
	if !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Error("BuildError 应能解包出它的 Cause")
	}
}

// T-ERR-009：BuildError 携带 VLESS URI 无效错误码
func TestBuildErrorWithVLessURIInvalid(t *testing.T) {
	e := &BuildError{Code: CodeBuildVLessURIInvalid, Phase: "source", Message: `VLESS URI "vless://..." 无效：缺少 @ 分隔符`}
	want := `构建错误 [source]：VLESS URI "vless://..." 无效：缺少 @ 分隔符`
	if got := e.Error(); got != want {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
	if e.Code != CodeBuildVLessURIInvalid {
		t.Errorf("code = %q，期望 %q", e.Code, CodeBuildVLessURIInvalid)
	}
}

// T-ERR-010：BuildError 携带 VLESS 来源行无效错误码
func TestBuildErrorWithVLessSourceLineInvalid(t *testing.T) {
	e := &BuildError{Code: CodeBuildVLessSourceLineInvalid, Phase: "source", Message: `VLESS 来源 "https://ex.com/v.txt" 第 3 行解析失败：invalid UUID`}
	want := `构建错误 [source]：VLESS 来源 "https://ex.com/v.txt" 第 3 行解析失败：invalid UUID`
	if got := e.Error(); got != want {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
	if e.Code != CodeBuildVLessSourceLineInvalid {
		t.Errorf("code = %q，期望 %q", e.Code, CodeBuildVLessSourceLineInvalid)
	}
}

// T-ERR-010b：BuildError 携带 AnyTLS 无效错误码
func TestBuildErrorWithAnyTLSInvalidCodes(t *testing.T) {
	tests := []struct {
		name string
		code Code
		msg  string
	}{
		{name: "uri", code: CodeBuildAnyTLSURIInvalid, msg: `AnyTLS URI "anytls://..." 无效：缺少 password`},
		{name: "line", code: CodeBuildAnyTLSLineInvalid, msg: `AnyTLS 订阅行 "HK = anytls, ..." 无效：缺少 password`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &BuildError{Code: tt.code, Phase: "source", Message: tt.msg}
			if got := e.Error(); got != "构建错误 [source]："+tt.msg {
				t.Errorf("得到 %q，期望 %q", got, "构建错误 [source]："+tt.msg)
			}
			if e.Code != tt.code {
				t.Errorf("code = %q，期望 %q", e.Code, tt.code)
			}
		})
	}
}

// T-ERR-011：RenderError 包含格式和消息
func TestRenderError(t *testing.T) {
	e := &RenderError{Code: CodeRenderTemplateParseFailed, Format: "clash", Message: "底版模板解析失败", Cause: io.EOF}
	want := `渲染错误 [clash]：底版模板解析失败`
	if got := e.Error(); got != want {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
}

// T-ERR-012：RenderError 可以解包出 Cause
func TestRenderErrorUnwrap(t *testing.T) {
	e := &RenderError{Code: CodeRenderYAMLEncodeFailed, Format: "surge", Message: "写入失败", Cause: io.EOF}
	if !errors.Is(e, io.EOF) {
		t.Error("RenderError 应能解包出它的 Cause")
	}
}

// T-ERR-013：TargetError 携带 Surge fallback 为空错误码
func TestTargetErrorWithSurgeFallbackEmpty(t *testing.T) {
	e := &TargetError{Code: CodeTargetSurgeFallbackEmpty, Format: "surge", Message: `fallback 服务组 "FINAL" 在 Surge 输出中成员为空`}
	want := `目标投影错误 [surge]：fallback 服务组 "FINAL" 在 Surge 输出中成员为空`
	if got := e.Error(); got != want {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
	if e.Code != CodeTargetSurgeFallbackEmpty {
		t.Errorf("code = %q，期望 %q", e.Code, CodeTargetSurgeFallbackEmpty)
	}
}

// T-ERR-014：TargetError 可以解包出 Cause
func TestTargetErrorUnwrap(t *testing.T) {
	e := &TargetError{Code: CodeTargetClashProjectionInvalid, Format: "clash", Message: "投影失败", Cause: io.EOF}
	if !errors.Is(e, io.EOF) {
		t.Error("TargetError 应能解包出它的 Cause")
	}
}

// T-ERR-015：FetchError 的 Cause 为 nil 时不会 panic
func TestFetchErrorNilCause(t *testing.T) {
	e := &FetchError{Code: CodeFetchStatusInvalid, URL: "https://example.com/sub", Message: "上游返回 HTTP 404"}
	if e.Unwrap() != nil {
		t.Error("Cause 为 nil 的 FetchError 解包结果应为 nil")
	}
}

// T-ERR-016：ResourceError 的 Cause 为 nil 时不会 panic
func TestResourceErrorNilCause(t *testing.T) {
	e := &ResourceError{Code: CodeResourceLocalReadFailed, Location: "/tmp/base.yaml", Message: "no such file or directory"}
	if e.Unwrap() != nil {
		t.Error("Cause 为 nil 的 ResourceError 解包结果应为 nil")
	}
}

// T-ERR-017：RenderError 的 Cause 为 nil 时不会 panic
func TestRenderErrorNilCause(t *testing.T) {
	e := &RenderError{Code: CodeRenderTemplateInvalid, Format: "clash", Message: "底版模板格式无效"}
	if e.Unwrap() != nil {
		t.Error("Cause 为 nil 的 RenderError 解包结果应为 nil")
	}
}

// T-ERR-018：TargetError 的 Cause 为 nil 时不会 panic
func TestTargetErrorNilCause(t *testing.T) {
	e := &TargetError{Code: CodeTargetClashFallbackEmpty, Format: "clash", Message: "fallback 清空"}
	if e.Unwrap() != nil {
		t.Error("Cause 为 nil 的 TargetError 解包结果应为 nil")
	}
}

// T-ERR-019：BuildError 的 Cause 为 nil 时不会 panic
func TestBuildErrorNilCause(t *testing.T) {
	e := &BuildError{Code: CodeBuildValidationFailed, Phase: "group", Message: "链式节点组为空"}
	if e.Unwrap() != nil {
		t.Error("Cause 为 nil 的 BuildError 解包结果应为 nil")
	}
}

// T-ERR-020：结构化订阅格式错误使用稳定的拉取错误码。
func TestFetchErrorWithSubscriptionFormatInvalid(t *testing.T) {
	e := &FetchError{
		Code:    CodeFetchSubscriptionFormatInvalid,
		URL:     "https://example.com/profile.yaml",
		Message: "Clash YAML 订阅格式无效：缺少顶层 proxies 字段",
	}
	want := `拉取失败 [https://example.com/profile.yaml]：Clash YAML 订阅格式无效：缺少顶层 proxies 字段`
	if got := e.Error(); got != want {
		t.Fatalf("得到 %q，期望 %q", got, want)
	}
	if e.Code != CodeFetchSubscriptionFormatInvalid {
		t.Fatalf("code = %q，期望 %q", e.Code, CodeFetchSubscriptionFormatInvalid)
	}
}

// T-ERR-021：格式错误的 Clash 原生节点在渲染时有专用错误码。
func TestRenderErrorWithClashProxyInvalid(t *testing.T) {
	e := &RenderError{
		Code:    CodeRenderClashProxyInvalid,
		Format:  "clash",
		Message: `代理 "HK" 渲染失败：原生节点无效`,
	}
	if e.Code != CodeRenderClashProxyInvalid {
		t.Fatalf("code = %q，期望 %q", e.Code, CodeRenderClashProxyInvalid)
	}
	if !strings.Contains(e.Error(), "原生节点无效") {
		t.Fatalf("错误文本不符合预期：%s", e.Error())
	}
}

// T-ERR-STORE-001：正文存储失败沿用资源错误的约定。
func TestResourceError_ContentWriteFailed(t *testing.T) {
	e := &ResourceError{Code: CodeResourceContentWriteFailed, Location: "/config", Message: "正文目录无法写入"}
	if e.Code != "resource_content_write_failed" || e.Error() != "资源读取失败 [/config]：正文目录无法写入" {
		t.Fatalf("存储错误不符合预期：%v", e)
	}
}

// T-ERR-BOOT-001：配置未初始化时有稳定的错误码和配置错误格式。
func TestConfigError_NotInitialized(t *testing.T) {
	e := &ConfigError{Code: CodeConfigNotInitialized, Message: "请先保存并热重载配置"}
	if e.Code != "config_not_initialized" || e.Error() != "配置错误：请先保存并热重载配置" {
		t.Fatalf("code=%s error=%q", e.Code, e.Error())
	}
}
