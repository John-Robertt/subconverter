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
	//nolint:gosec // Plain-text error responses are not rendered as HTML.
	_, _ = w.Write([]byte(message))
}

// presentError converts internal errors into stable HTTP status codes and
// Chinese text intended for end users.
func presentError(err error) (statusCode int, message string) {
	var reqErr *requestError
	if errors.As(err, &reqErr) {
		return reqErr.status, reqErr.message
	}

	if cfgErrs := collectConfigErrors(err); len(cfgErrs) > 0 {
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

	var renderErr *errtype.RenderError
	if errors.As(err, &renderErr) {
		return http.StatusInternalServerError, formatRenderError(renderErr)
	}

	return http.StatusInternalServerError, "内部错误"
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

// flattenErrors splits errors.Join chains into individual errors.
// It only expands multi-Unwrap (errors.Join); single-Unwrap chains are left
// intact so that typed errors like FetchError (which wrap a Cause) are
// preserved as leaves. errors.As already walks single-Unwrap chains.
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
	if err.Field != "" {
		return "配置错误 [" + err.Field + "]：" + err.Message
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

func formatRenderError(err *errtype.RenderError) string {
	if err.Format != "" {
		return "渲染错误 [" + err.Format + "]：" + err.Message
	}
	return "渲染错误：" + err.Message
}
