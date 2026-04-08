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

func TestLoadResource_LocalFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	want := []byte("hello world")
	if err := os.WriteFile(path, want, 0644); err != nil {
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

func TestLoadResource_LocalFileNotFound(t *testing.T) {
	_, err := LoadResource(context.Background(), "/nonexistent/path/file.txt", nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Errorf("expected *errtype.FetchError, got %T", err)
	}
}

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

func TestLoadResource_RemoteURLFetchError(t *testing.T) {
	f := &fakeFetcher{err: &errtype.FetchError{URL: "example.com", Message: "timeout"}}

	_, err := LoadResource(context.Background(), "https://example.com/config.yaml", f)
	if err == nil {
		t.Fatal("expected error from fetcher")
	}
}

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
