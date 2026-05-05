package fetch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// fakeFetcher implements Fetcher for testing.
type fakeFetcher struct {
	data []byte
	err  error
}

func (f *fakeFetcher) Fetch(_ context.Context, _ string) ([]byte, error) {
	return f.data, f.err
}

// T-RES-001: LoadResource reads local file
func TestLoadResource_LocalFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	want := []byte("hello world")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadResource(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// T-RES-002: LoadResource returns ResourceError for missing local file
func TestLoadResource_LocalFileNotFound(t *testing.T) {
	const location = "/nonexistent/path/file.txt"
	_, err := LoadResource(context.Background(), location, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	var resourceErr *errtype.ResourceError
	if !errors.As(err, &resourceErr) {
		t.Fatalf("expected *errtype.ResourceError, got %T", err)
	}
	if resourceErr.Code != errtype.CodeResourceLocalReadFailed {
		t.Errorf("code = %q, want %q", resourceErr.Code, errtype.CodeResourceLocalReadFailed)
	}
	if resourceErr.Location != location {
		t.Errorf("location = %q, want %q", resourceErr.Location, location)
	}
}

// T-RES-003: LoadResource fetches remote URL
func TestLoadResource_RemoteURL(t *testing.T) {
	want := []byte("remote content")
	f := &fakeFetcher{data: want}

	got, err := LoadResource(context.Background(), "https://example.com/config.yaml", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// T-RES-004: LoadResource returns FetchError for failed remote URL
func TestLoadResource_RemoteURLFetchError(t *testing.T) {
	f := &fakeFetcher{err: &errtype.FetchError{Code: errtype.CodeFetchRequestFailed, URL: "example.com", Message: "请求上游失败：timeout"}}

	_, err := LoadResource(context.Background(), "https://example.com/config.yaml", f)
	if err == nil {
		t.Fatal("expected error from fetcher")
	}
}

// T-RES-005: LoadResource returns FetcherRequired error when fetcher is nil
func TestLoadResource_RemoteURLNilFetcher(t *testing.T) {
	_, err := LoadResource(context.Background(), "https://example.com/config.yaml", nil)
	if err == nil {
		t.Fatal("expected error for remote URL with nil fetcher")
	}
	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Errorf("expected *errtype.FetchError, got %T", err)
	}
}
