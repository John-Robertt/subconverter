package errtype

import (
	"errors"
	"io"
	"testing"
)

func TestConfigError(t *testing.T) {
	e := &ConfigError{Field: "groups.HK.strategy", Message: "must be select or url-test"}
	want := `config error [groups.HK.strategy]: must be select or url-test`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfigErrorNoField(t *testing.T) {
	e := &ConfigError{Message: "invalid YAML"}
	want := `config error: invalid YAML`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFetchError(t *testing.T) {
	e := &FetchError{URL: "https://example.com/sub?token=***", Message: "timeout", Cause: io.ErrUnexpectedEOF}
	want := `fetch error [https://example.com/sub?token=***]: timeout`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFetchErrorUnwrap(t *testing.T) {
	e := &FetchError{URL: "https://example.com/sub", Message: "timeout", Cause: io.ErrUnexpectedEOF}
	if !errors.Is(e, io.ErrUnexpectedEOF) {
		t.Error("FetchError should unwrap to its Cause")
	}
}

func TestBuildError(t *testing.T) {
	e := &BuildError{Phase: "group", Message: "empty chain group"}
	want := `build error [group]: empty chain group`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderError(t *testing.T) {
	e := &RenderError{Format: "clash", Message: "template failed", Cause: io.EOF}
	want := `render error [clash]: template failed`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderErrorUnwrap(t *testing.T) {
	e := &RenderError{Format: "surge", Message: "write failed", Cause: io.EOF}
	if !errors.Is(e, io.EOF) {
		t.Error("RenderError should unwrap to its Cause")
	}
}

func TestFetchErrorNilCause(t *testing.T) {
	e := &FetchError{URL: "https://example.com/sub", Message: "not found"}
	if e.Unwrap() != nil {
		t.Error("FetchError with nil Cause should unwrap to nil")
	}
}

func TestRenderErrorNilCause(t *testing.T) {
	e := &RenderError{Format: "clash", Message: "unknown field"}
	if e.Unwrap() != nil {
		t.Error("RenderError with nil Cause should unwrap to nil")
	}
}
