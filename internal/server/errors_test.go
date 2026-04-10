package server

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

func TestPresentError_MultipleFetchErrors(t *testing.T) {
	err := errors.Join(
		&errtype.FetchError{
			Code:    errtype.CodeFetchRequestFailed,
			URL:     "https://sub1.example.com/***",
			Message: "连接超时",
		},
		&errtype.FetchError{
			Code:    errtype.CodeFetchStatusInvalid,
			URL:     "https://sub2.example.com/***",
			Message: "上游返回 HTTP 503",
		},
	)

	code, msg := presentError(err)

	if code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", code, http.StatusBadGateway)
	}
	if !strings.Contains(msg, "sub1.example.com") {
		t.Errorf("message should mention first URL, got %q", msg)
	}
	if !strings.Contains(msg, "sub2.example.com") {
		t.Errorf("message should mention second URL, got %q", msg)
	}
	if !strings.Contains(msg, "连接超时") {
		t.Errorf("message should contain first error detail, got %q", msg)
	}
	if !strings.Contains(msg, "HTTP 503") {
		t.Errorf("message should contain second error detail, got %q", msg)
	}
}
