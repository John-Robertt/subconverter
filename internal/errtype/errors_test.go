package errtype

import (
	"errors"
	"io"
	"testing"
)

func TestConfigError(t *testing.T) {
	e := &ConfigError{Code: CodeConfigValidationFailed, Field: "groups.HK.strategy", Message: "必须为 select 或 url-test"}
	want := `config error [groups.HK.strategy]: 必须为 select 或 url-test`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if e.Code != CodeConfigValidationFailed {
		t.Errorf("code = %q, want %q", e.Code, CodeConfigValidationFailed)
	}
}

func TestConfigErrorNoField(t *testing.T) {
	e := &ConfigError{Code: CodeConfigYAMLInvalid, Message: "YAML 解析失败"}
	want := `config error: YAML 解析失败`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFetchError(t *testing.T) {
	e := &FetchError{Code: CodeFetchRequestFailed, URL: "https://example.com/sub?token=***", Message: "请求上游失败：timeout", Cause: io.ErrUnexpectedEOF}
	want := `fetch error [https://example.com/sub?token=***]: 请求上游失败：timeout`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFetchErrorUnwrap(t *testing.T) {
	e := &FetchError{Code: CodeFetchRequestFailed, URL: "https://example.com/sub", Message: "请求上游失败：timeout", Cause: io.ErrUnexpectedEOF}
	if !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Error("FetchError should unwrap to its Cause")
	}
}

func TestResourceError(t *testing.T) {
	e := &ResourceError{Code: CodeResourceLocalReadFailed, Location: "/tmp/base.yaml", Message: "no such file or directory", Cause: io.ErrUnexpectedEOF}
	want := `resource error [/tmp/base.yaml]: no such file or directory`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResourceErrorUnwrap(t *testing.T) {
	e := &ResourceError{Code: CodeResourceLocalReadFailed, Location: "/tmp/base.yaml", Message: "permission denied", Cause: io.ErrUnexpectedEOF}
	if !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Error("ResourceError should unwrap to its Cause")
	}
}

func TestBuildError(t *testing.T) {
	e := &BuildError{Code: CodeBuildValidationFailed, Phase: "group", Message: "链式节点组为空"}
	want := `build error [group]: 链式节点组为空`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildErrorUnwrap(t *testing.T) {
	e := &BuildError{Code: CodeBuildValidationFailed, Phase: "group", Message: "链式节点组为空", Cause: io.ErrUnexpectedEOF}
	if !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Error("BuildError should unwrap to its Cause")
	}
}

func TestRenderError(t *testing.T) {
	e := &RenderError{Code: CodeRenderTemplateParseFailed, Format: "clash", Message: "底版模板解析失败", Cause: io.EOF}
	want := `render error [clash]: 底版模板解析失败`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderErrorUnwrap(t *testing.T) {
	e := &RenderError{Code: CodeRenderYAMLEncodeFailed, Format: "surge", Message: "写入失败", Cause: io.EOF}
	if !errors.Is(e, io.EOF) {
		t.Error("RenderError should unwrap to its Cause")
	}
}

func TestFetchErrorNilCause(t *testing.T) {
	e := &FetchError{Code: CodeFetchStatusInvalid, URL: "https://example.com/sub", Message: "上游返回 HTTP 404"}
	if e.Unwrap() != nil {
		t.Error("FetchError with nil Cause should unwrap to nil")
	}
}

func TestResourceErrorNilCause(t *testing.T) {
	e := &ResourceError{Code: CodeResourceLocalReadFailed, Location: "/tmp/base.yaml", Message: "no such file or directory"}
	if e.Unwrap() != nil {
		t.Error("ResourceError with nil Cause should unwrap to nil")
	}
}

func TestRenderErrorNilCause(t *testing.T) {
	e := &RenderError{Code: CodeRenderTemplateInvalid, Format: "clash", Message: "底版模板格式无效"}
	if e.Unwrap() != nil {
		t.Error("RenderError with nil Cause should unwrap to nil")
	}
}

func TestBuildErrorNilCause(t *testing.T) {
	e := &BuildError{Code: CodeBuildValidationFailed, Phase: "group", Message: "链式节点组为空"}
	if e.Unwrap() != nil {
		t.Error("BuildError with nil Cause should unwrap to nil")
	}
}
