package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// Fetcher 抽象了对订阅 URL 的 HTTP GET。
// 生产代码使用 HTTPFetcher，测试注入假实现。
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, error)
}

// HTTPFetcher 发出真实的 HTTP GET 请求。
type HTTPFetcher struct {
	Client *http.Client
}

// Fetch 发出 HTTP GET 并返回响应体。
// 错误包装为 *errtype.FetchError，其中的 URL 已经过清理。
func (f *HTTPFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchRequestURLInvalid,
			URL:     SanitizeURL(rawURL),
			Message: "请求 URL 无效",
			Cause:   err,
		}
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchRequestFailed,
			URL:     SanitizeURL(rawURL),
			Message: "请求上游失败：" + err.Error(),
			Cause:   err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 读完响应体，让 HTTP keep-alive 连接可以复用。
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchStatusInvalid,
			URL:     SanitizeURL(rawURL),
			Message: fmt.Sprintf("上游返回 HTTP %d", resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &errtype.FetchError{
			Code:    errtype.CodeFetchBodyReadFailed,
			URL:     SanitizeURL(rawURL),
			Message: "读取响应体失败",
			Cause:   err,
		}
	}

	return body, nil
}

// SanitizeURL 去掉 URL 中的查询参数和片段（fragment），
// 防止错误信息泄露敏感信息（例如用户令牌）。
func SanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
