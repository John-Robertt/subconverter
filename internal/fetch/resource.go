package fetch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// LoadResource 从本地文件路径或远程 HTTP(S) URL 加载内容。
// location 以 http:// 或 https:// 开头时使用传入的 Fetcher，
// 否则从本地文件系统读取。
// f 为 nil 时只支持本地路径。
func LoadResource(ctx context.Context, location string, f Fetcher) ([]byte, error) {
	if isRemoteURL(location) {
		if f == nil {
			return nil, &errtype.FetchError{
				Code:    errtype.CodeFetchFetcherRequired,
				URL:     SanitizeURL(location),
				Message: "远程 URL 需要 Fetcher，但当前未提供",
			}
		}
		return f.Fetch(ctx, location)
	}

	data, err := os.ReadFile(filepath.Clean(location))
	if err != nil {
		message := err.Error()
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			message = pathErr.Err.Error()
		}

		return nil, &errtype.ResourceError{
			Code:     errtype.CodeResourceLocalReadFailed,
			Location: location,
			Message:  message,
			Cause:    err,
		}
	}
	return data, nil
}

func isRemoteURL(location string) bool {
	return strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://")
}
