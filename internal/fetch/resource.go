package fetch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// LoadResource loads content from a local file path or a remote HTTP(S) URL.
// If location starts with http:// or https://, it uses the provided Fetcher;
// otherwise it reads from the local filesystem.
// When f is nil, only local paths are supported.
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
		return nil, fmt.Errorf("reading local file %q: %w", location, err)
	}
	return data, nil
}

func isRemoteURL(location string) bool {
	return strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://")
}
