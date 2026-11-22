package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

type requestError struct {
	status  int
	message string
}

func (e *requestError) Error() string {
	return e.message
}

func badRequest(message string) error {
	return &requestError{status: http.StatusBadRequest, message: message}
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	//nolint:gosec // 纯文本错误响应不会被当作 HTML 渲染。
	_, _ = w.Write([]byte(message))
}

// presentError 把内部错误转换为稳定的 HTTP 状态码和面向最终用户的中文文案。
func presentError(err error) (statusCode int, message string) {
	var reqErr *requestError
	if errors.As(err, &reqErr) {
		return reqErr.status, reqErr.message
	}

	if cfgErrs := collectConfigErrors(err); len(cfgErrs) > 0 {
		for _, cfgErr := range cfgErrs {
			if cfgErr.Code == errtype.CodeConfigNotInitialized {
				return http.StatusConflict, joinMessages(cfgErrs, formatConfigError)
			}
		}
		return http.StatusBadRequest, joinMessages(cfgErrs, formatConfigError)
	}

	if fetchErrs := collectFetchErrors(err); len(fetchErrs) > 0 {
		return http.StatusBadGateway, joinMessages(fetchErrs, formatFetchError)
	}

	if resourceErrs := collectResourceErrors(err); len(resourceErrs) > 0 {
		return http.StatusInternalServerError, joinMessages(resourceErrs, formatResourceError)
	}

	if buildErrs := collectBuildErrors(err); len(buildErrs) > 0 {
		return http.StatusBadRequest, joinMessages(buildErrs, formatBuildError)
	}

	var targetErr *errtype.TargetError
	if errors.As(err, &targetErr) {
		if isUserFixableTargetError(targetErr) {
			return http.StatusBadRequest, formatTargetError(targetErr)
		}
		return http.StatusInternalServerError, formatTargetError(targetErr)
	}

	var renderErr *errtype.RenderError
	if errors.As(err, &renderErr) {
		return http.StatusInternalServerError, formatRenderError(renderErr)
	}

	return http.StatusInternalServerError, "内部错误"
}

func isUserFixableTargetError(err *errtype.TargetError) bool {
	switch err.Code {
	case errtype.CodeTargetClashFallbackEmpty,
		errtype.CodeTargetSurgeFallbackEmpty,
		errtype.CodeTargetLoonFallbackEmpty:
		return true
	default:
		return false
	}
}

func collectConfigErrors(err error) []*errtype.ConfigError {
	leaves := flattenErrors(err)
	result := make([]*errtype.ConfigError, 0, len(leaves))
	for _, leaf := range leaves {
		var cfgErr *errtype.ConfigError
		if errors.As(leaf, &cfgErr) {
			result = append(result, cfgErr)
		}
	}
	return result
}

func collectFetchErrors(err error) []*errtype.FetchError {
	leaves := flattenErrors(err)
	result := make([]*errtype.FetchError, 0, len(leaves))
	for _, leaf := range leaves {
		var fetchErr *errtype.FetchError
		if errors.As(leaf, &fetchErr) {
			result = append(result, fetchErr)
		}
	}
	return result
}

func collectBuildErrors(err error) []*errtype.BuildError {
	leaves := flattenErrors(err)
	result := make([]*errtype.BuildError, 0, len(leaves))
	for _, leaf := range leaves {
		var buildErr *errtype.BuildError
		if errors.As(leaf, &buildErr) {
			result = append(result, buildErr)
		}
	}
	return result
}

func collectResourceErrors(err error) []*errtype.ResourceError {
	leaves := flattenErrors(err)
	result := make([]*errtype.ResourceError, 0, len(leaves))
	for _, leaf := range leaves {
		var resourceErr *errtype.ResourceError
		if errors.As(leaf, &resourceErr) {
			result = append(result, resourceErr)
		}
	}
	return result
}

// flattenErrors 把 errors.Join 链拆成单个错误。
// 它只展开多值 Unwrap（errors.Join）；单值 Unwrap 链保持原样，这样 FetchError
// 这类包装了 Cause 的类型化错误会作为叶子保留下来。errors.As 本身就会沿单值
// Unwrap 链查找。
func flattenErrors(err error) []error {
	if err == nil {
		return nil
	}

	type multiUnwrapper interface{ Unwrap() []error }
	if joined, ok := err.(multiUnwrapper); ok {
		var result []error
		for _, inner := range joined.Unwrap() {
			result = append(result, flattenErrors(inner)...)
		}
		return result
	}

	return []error{err}
}

func joinMessages[T any](items []T, format func(T) string) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, format(item))
	}
	return strings.Join(lines, "\n")
}

func formatConfigError(err *errtype.ConfigError) string {
	if path := err.DisplayPath(); path != "" {
		return "配置错误 [" + path + "]：" + err.Message
	}
	return "配置错误：" + err.Message
}

func formatFetchError(err *errtype.FetchError) string {
	if err.URL != "" {
		return "拉取失败 [" + err.URL + "]：" + err.Message
	}
	return "拉取失败：" + err.Message
}

func formatResourceError(err *errtype.ResourceError) string {
	if err.Location != "" {
		return "资源读取失败 [" + err.Location + "]：" + err.Message
	}
	return "资源读取失败：" + err.Message
}

func formatBuildError(err *errtype.BuildError) string {
	if err.Phase != "" {
		return "构建错误 [" + err.Phase + "]：" + err.Message
	}
	return "构建错误：" + err.Message
}

func formatTargetError(err *errtype.TargetError) string {
	if err.Format != "" {
		return "目标投影错误 [" + err.Format + "]：" + err.Message
	}
	return "目标投影错误：" + err.Message
}

func formatRenderError(err *errtype.RenderError) string {
	if err.Format != "" {
		return "渲染错误 [" + err.Format + "]：" + err.Message
	}
	return "渲染错误：" + err.Message
}
