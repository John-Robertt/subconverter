package server

import (
	"errors"
	"fmt"
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

func TestPresentError_ResourceError(t *testing.T) {
	err := &errtype.ResourceError{
		Code:     errtype.CodeResourceLocalReadFailed,
		Location: "/tmp/base.yaml",
		Message:  "no such file or directory",
	}

	code, msg := presentError(err)

	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", code, http.StatusInternalServerError)
	}
	if !strings.Contains(msg, "资源读取失败") {
		t.Fatalf("message should contain 资源读取失败, got %q", msg)
	}
	if !strings.Contains(msg, "/tmp/base.yaml") {
		t.Errorf("message should mention location, got %q", msg)
	}
	if !strings.Contains(msg, "no such file or directory") {
		t.Errorf("message should contain os error detail, got %q", msg)
	}
}

func TestPresentError_TargetError(t *testing.T) {
	err := &errtype.TargetError{
		Code:    errtype.CodeTargetClashFallbackEmpty,
		Format:  "clash",
		Message: `fallback 服务组 "FINAL" 在 Clash 输出中成员为空`,
	}

	code, msg := presentError(err)

	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", code, http.StatusBadRequest)
	}
	if !strings.Contains(msg, "目标投影错误 [clash]") {
		t.Fatalf("message should contain target projection prefix, got %q", msg)
	}
	if !strings.Contains(msg, "FINAL") {
		t.Errorf("message should contain target error detail, got %q", msg)
	}
}

// TargetError 可能被 generate.Service 包进 errors.Join 与其他非 errtype 错误一起返回。
// 锁定：flattenErrors 会拆分 Join，collect* 对非 errtype 叶子不匹配，最终 errors.As 能找到
// TargetError，并按错误码映射为用户可修复的 400。这条测试防止未来移动 targetErr
// 分支到 collect* 之前时误让 errors.Join 场景退化到 "内部错误"。
func TestPresentError_TargetErrorWrappedInErrorsJoin(t *testing.T) {
	targetErr := &errtype.TargetError{
		Code:    errtype.CodeTargetSurgeFallbackEmpty,
		Format:  "surge",
		Message: `fallback 服务组 "FINAL" 在 Surge 输出中成员为空`,
	}
	err := errors.Join(targetErr, errors.New("ancillary context"))

	code, msg := presentError(err)

	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", code, http.StatusBadRequest)
	}
	if !strings.Contains(msg, "目标投影错误 [surge]") {
		t.Fatalf("message should contain target projection prefix, got %q", msg)
	}
	if !strings.Contains(msg, "FINAL") {
		t.Errorf("message should contain target error detail, got %q", msg)
	}
}

// 锁定：errors.As 穿透 fmt.Errorf 单层 %w 包装仍能提取 TargetError。
// 管道任何一层用 fmt.Errorf("...: %w", targetErr) 再返回，HTTP 层映射不退化。
func TestPresentError_TargetErrorThroughFmtErrorfWrap(t *testing.T) {
	inner := &errtype.TargetError{
		Code:    errtype.CodeTargetClashProjectionInvalid,
		Format:  "clash",
		Message: "投影阶段内部不变量异常",
	}
	err := fmt.Errorf("generate pipeline failed: %w", inner)

	code, msg := presentError(err)

	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", code, http.StatusInternalServerError)
	}
	if !strings.Contains(msg, "目标投影错误 [clash]") {
		t.Fatalf("message should contain target projection prefix, got %q", msg)
	}
	if !strings.Contains(msg, "内部不变量异常") {
		t.Errorf("message should contain inner detail, got %q", msg)
	}
}
