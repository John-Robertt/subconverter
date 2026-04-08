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
				t.Errorf("SanitizeURL(%q) = %q, want %q", tt.raw, got, tt.want)
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
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != body {
		t.Errorf("body = %q, want %q", got, body)
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
		t.Fatal("expected error for 403 response")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("error type = %T, want *errtype.FetchError", err)
	}
	if fetchErr.Message != "HTTP 403" {
		t.Errorf("message = %q, want %q", fetchErr.Message, "HTTP 403")
	}
	// URL should be sanitized (no query params)
	if fetchErr.URL != srv.URL+"/sub" {
		t.Errorf("URL = %q, want query params stripped", fetchErr.URL)
	}
}

func TestHTTPFetcher_NetworkError(t *testing.T) {
	f := &HTTPFetcher{Client: &http.Client{Timeout: 100 * time.Millisecond}}
	// Use a URL that will fail to connect.
	_, err := f.Fetch(context.Background(), "http://192.0.2.1:1/sub?token=secret")
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}

	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("error type = %T, want *errtype.FetchError", err)
	}
	if fetchErr.Cause == nil {
		t.Error("expected Cause to be set for network errors")
	}
	// URL should be sanitized
	if fetchErr.URL != "http://192.0.2.1:1/sub" {
		t.Errorf("URL = %q, want query params stripped", fetchErr.URL)
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

	// Inject a controllable clock.
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cf.now = func() time.Time { return now }

	ctx := context.Background()
	url := "https://sub.example.com/api?token=x"

	// First call: cache miss, fetches from inner.
	body1, err := cf.Fetch(ctx, url)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if string(body1) != "data" {
		t.Errorf("body = %q, want %q", body1, "data")
	}
	if mock.calls != 1 {
		t.Errorf("calls = %d, want 1", mock.calls)
	}

	// Second call within TTL: cache hit.
	now = now.Add(2 * time.Minute)
	body2, err := cf.Fetch(ctx, url)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if string(body2) != "data" {
		t.Errorf("body = %q, want %q", body2, "data")
	}
	if mock.calls != 1 {
		t.Errorf("calls = %d after cache hit, want 1", mock.calls)
	}

	// Third call after TTL: cache miss, re-fetches.
	now = now.Add(4 * time.Minute) // total 6 min > 5 min TTL
	body3, err := cf.Fetch(ctx, url)
	if err != nil {
		t.Fatalf("third fetch: %v", err)
	}
	if string(body3) != "data" {
		t.Errorf("body = %q, want %q", body3, "data")
	}
	if mock.calls != 2 {
		t.Errorf("calls = %d after TTL expiry, want 2", mock.calls)
	}
}

func TestCachedFetcher_ErrorNotCached(t *testing.T) {
	mock := &mockFetcher{err: fmt.Errorf("network error")}
	cf := NewCachedFetcher(mock, 5*time.Minute)
	ctx := context.Background()
	url := "https://sub.example.com/api"

	// First call: error.
	_, err := cf.Fetch(ctx, url)
	if err == nil {
		t.Fatal("expected error")
	}
	if mock.calls != 1 {
		t.Errorf("calls = %d, want 1", mock.calls)
	}

	// Second call: should retry (error was not cached).
	_, err = cf.Fetch(ctx, url)
	if err == nil {
		t.Fatal("expected error")
	}
	if mock.calls != 2 {
		t.Errorf("calls = %d, want 2 (error should not be cached)", mock.calls)
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

	// Mutate the returned slice.
	body1[0] = 'X'

	// Fetch again — should still return original data.
	body2, err := cf.Fetch(ctx, "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if string(body2) != "original" {
		t.Errorf("cached data was mutated: got %q, want %q", body2, "original")
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
		t.Errorf("inner fetcher body was mutated: got %q, want %q", mock.body, "original")
	}
}
