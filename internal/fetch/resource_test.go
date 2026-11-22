package fetch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// fakeFetcher 是供测试使用的 Fetcher 实现。
type fakeFetcher struct {
	data []byte
	err  error
}

func (f *fakeFetcher) Fetch(_ context.Context, _ string) ([]byte, error) {
	return f.data, f.err
}

// T-RES-001：LoadResource 读取本地文件
func TestLoadResource_LocalFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	want := []byte("hello world")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadResource(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if string(got) != string(want) {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
}

// T-RES-002：本地文件不存在时 LoadResource 返回 ResourceError
func TestLoadResource_LocalFileNotFound(t *testing.T) {
	const location = "/nonexistent/path/file.txt"
	_, err := LoadResource(context.Background(), location, nil)
	if err == nil {
		t.Fatal("文件不存在时应返回错误")
	}
	var resourceErr *errtype.ResourceError
	if !errors.As(err, &resourceErr) {
		t.Fatalf("期望 *errtype.ResourceError，得到 %T", err)
	}
	if resourceErr.Code != errtype.CodeResourceLocalReadFailed {
		t.Errorf("code = %q，期望 %q", resourceErr.Code, errtype.CodeResourceLocalReadFailed)
	}
	if resourceErr.Location != location {
		t.Errorf("location = %q，期望 %q", resourceErr.Location, location)
	}
}

// T-RES-003：LoadResource 拉取远程 URL
func TestLoadResource_RemoteURL(t *testing.T) {
	want := []byte("remote content")
	f := &fakeFetcher{data: want}

	got, err := LoadResource(context.Background(), "https://example.com/config.yaml", f)
	if err != nil {
		t.Fatalf("意外错误：%v", err)
	}
	if string(got) != string(want) {
		t.Errorf("得到 %q，期望 %q", got, want)
	}
}

// T-RES-004：远程 URL 拉取失败时 LoadResource 返回 FetchError
func TestLoadResource_RemoteURLFetchError(t *testing.T) {
	f := &fakeFetcher{err: &errtype.FetchError{Code: errtype.CodeFetchRequestFailed, URL: "example.com", Message: "请求上游失败：timeout"}}

	_, err := LoadResource(context.Background(), "https://example.com/config.yaml", f)
	if err == nil {
		t.Fatal("fetcher 出错时应返回错误")
	}
}

// T-RES-005：fetcher 为 nil 时 LoadResource 返回 FetcherRequired 错误
func TestLoadResource_RemoteURLNilFetcher(t *testing.T) {
	_, err := LoadResource(context.Background(), "https://example.com/config.yaml", nil)
	if err == nil {
		t.Fatal("fetcher 为 nil 时访问远程 URL 应返回错误")
	}
	var fetchErr *errtype.FetchError
	if !errors.As(err, &fetchErr) {
		t.Errorf("期望 *errtype.FetchError，得到 %T", err)
	}
}
