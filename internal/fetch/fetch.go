package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// Fetcher abstracts HTTP GET for subscription URLs.
// Production code uses HTTPFetcher; tests inject fakes.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, error)
}

// HTTPFetcher performs real HTTP GET requests.
type HTTPFetcher struct {
	Client *http.Client
}

// Fetch issues an HTTP GET and returns the response body.
// Errors are wrapped as *errtype.FetchError with sanitized URLs.
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
		// Drain body to allow HTTP keep-alive connection reuse.
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

// SanitizeURL removes query parameters and fragment from a URL
// to prevent leaking sensitive information (e.g. user tokens) in error messages.
func SanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
