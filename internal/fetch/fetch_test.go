package fetch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// --- SanitizeURL ---

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "strips query params",
			raw:  "https://sub.example.com/api?token=secret&flag=clash",
			want: "https://sub.example.com/api",
		},
		{
			name: "strips fragment",
			raw:  "https://sub.example.com/path#section",
			want: "https://sub.example.com/path",
		},
		{
			name: "strips both query and fragment",
			raw:  "https://sub.example.com/api?token=x#frag",
			want: "https://sub.example.com/api",
		},
		{
			name: "no query or fragment unchanged",
			raw:  "https://sub.example.com/api",
			want: "https://sub.example.com/api",
		},
		{
			name: "preserves path",
			raw:  "https://sub.example.com/v1/subscribe?token=abc",
			want: "https://sub.example.com/v1/subscribe",
		},
		{
			name: "unparseable returns original",
			raw:  "://not-a-url",
			want: "://not-a-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeURL(tt.raw)
			if got != tt.want {
				t.Errorf("SanitizeURL(%q) = %q，期望 %q", tt.raw, got, tt.want)
			}
		})
	}
}

// --- HTTPFetcher ---

func TestHTTPFetcher_Success(t *testing.T) {
	body := "hello world"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, body)
	}))
	defer srv.Close()

	f := &HTTPFetcher{Client: srv.Client()}
	got, err := f.Fetch(context.Background(), srv.URL+"/sub?token=secret")
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if string(got) != body {
		t.Errorf("body = %q，期望 %q", got, body)
	}
}

func TestHTTPFetcher_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	f := &HTTPFetcher{Client: srv.Client()}
	_, err := f.Fetch(context.Background(), srv.URL+"/sub?token=secret")
	if err == nil {
		t.Fatal("403 响应应返回错误")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.FetchError", err)
	}
	if fetchErr.Code != errtype.CodeFetchStatusInvalid {
		t.Errorf("code = %q，期望 %q", fetchErr.Code, errtype.CodeFetchStatusInvalid)
	}
	if fetchErr.Message != "上游返回 HTTP 403" {
		t.Errorf("message = %q，期望 %q", fetchErr.Message, "上游返回 HTTP 403")
	}
	// URL 应已清理（不含查询参数）
	if fetchErr.URL != srv.URL+"/sub" {
		t.Errorf("URL = %q，期望已去掉查询参数", fetchErr.URL)
	}
}

func TestHTTPFetcher_NetworkError(t *testing.T) {
	f := &HTTPFetcher{Client: &http.Client{Timeout: 100 * time.Millisecond}}
	// 使用一个必然连接失败的 URL。
	_, err := f.Fetch(context.Background(), "http://192.0.2.1:1/sub?token=secret")
	if err == nil {
		t.Fatal("主机不可达时应返回错误")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("错误类型 = %T，期望 *errtype.FetchError", err)
	}
	if fetchErr.Code != errtype.CodeFetchRequestFailed {
		t.Errorf("code = %q，期望 %q", fetchErr.Code, errtype.CodeFetchRequestFailed)
	}
	if fetchErr.Cause == nil {
		t.Error("网络错误应设置 Cause")
	}
	// URL 应已清理
	if fetchErr.URL != "http://192.0.2.1:1/sub" {
		t.Errorf("URL = %q，期望已去掉查询参数", fetchErr.URL)
	}
}

// --- CachedFetcher ---

type mockFetcher struct {
	calls int
	body  []byte
	err   error
}

func (m *mockFetcher) Fetch(_ context.Context, _ string) ([]byte, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.body, nil
}

// T-SRC-004
func TestCachedFetcher_TTLHitAndMiss(t *testing.T) {
	mock := &mockFetcher{body: []byte("data")}
	cf := NewCachedFetcher(mock, 5*time.Minute)

	// 注入一个可控的时钟。
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cf.now = func() time.Time { return now }

	ctx := context.Background()
	url := "https://sub.example.com/api?token=x"

	// 第一次调用：缓存未命中，从内层拉取。
	body1, err := cf.Fetch(ctx, url)
	if err != nil {
		t.Fatalf("第一次拉取：%v", err)
	}
	if string(body1) != "data" {
		t.Errorf("body = %q，期望 %q", body1, "data")
	}
	if mock.calls != 1 {
		t.Errorf("calls = %d，期望 1", mock.calls)
	}

	// 第二次调用在 TTL 内：缓存命中。
	now = now.Add(2 * time.Minute)
	body2, err := cf.Fetch(ctx, url)
	if err != nil {
		t.Fatalf("第二次拉取：%v", err)
	}
	if string(body2) != "data" {
		t.Errorf("body = %q，期望 %q", body2, "data")
	}
	if mock.calls != 1 {
		t.Errorf("缓存命中后 calls = %d，期望 1", mock.calls)
	}

	// 第三次调用在 TTL 之后：缓存未命中，重新拉取。
	now = now.Add(4 * time.Minute) // 累计 6 分钟，超过 5 分钟的 TTL
	body3, err := cf.Fetch(ctx, url)
	if err != nil {
		t.Fatalf("第三次拉取：%v", err)
	}
	if string(body3) != "data" {
		t.Errorf("body = %q，期望 %q", body3, "data")
	}
	if mock.calls != 2 {
		t.Errorf("TTL 过期后 calls = %d，期望 2", mock.calls)
	}
}

func TestCachedFetcher_ErrorNotCached(t *testing.T) {
	mock := &mockFetcher{err: fmt.Errorf("网络错误")}
	cf := NewCachedFetcher(mock, 5*time.Minute)
	ctx := context.Background()
	url := "https://sub.example.com/api"

	// 第一次调用：返回错误。
	_, err := cf.Fetch(ctx, url)
	if err == nil {
		t.Fatal("应返回错误")
	}
	if mock.calls != 1 {
		t.Errorf("calls = %d，期望 1", mock.calls)
	}

	// 第二次调用：应重试（错误没有被缓存）。
	_, err = cf.Fetch(ctx, url)
	if err == nil {
		t.Fatal("应返回错误")
	}
	if mock.calls != 2 {
		t.Errorf("calls = %d，期望 2（错误不应被缓存）", mock.calls)
	}
}

func TestCachedFetcher_ReturnsCopy(t *testing.T) {
	mock := &mockFetcher{body: []byte("original")}
	cf := NewCachedFetcher(mock, 5*time.Minute)
	ctx := context.Background()

	body1, err := cf.Fetch(ctx, "https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	// 修改返回的切片。
	body1[0] = 'X'

	// 再次拉取，应仍返回原始数据。
	body2, err := cf.Fetch(ctx, "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if string(body2) != "original" {
		t.Errorf("缓存数据被修改：得到 %q，期望 %q", body2, "original")
	}
}

func TestCachedFetcher_MissReturnsCopy(t *testing.T) {
	mock := &mockFetcher{body: []byte("original")}
	cf := NewCachedFetcher(mock, 5*time.Minute)

	body, err := cf.Fetch(context.Background(), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	body[0] = 'X'

	if string(mock.body) != "original" {
		t.Errorf("内层 fetcher 的 body 被修改：得到 %q，期望 %q", mock.body, "original")
	}
}
