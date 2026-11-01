package app

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/pipeline"
)

type ResourceContentResult struct {
	Content string `json:"content"`
	Format  string `json:"format,omitempty"`
}

// ReadResource returns an editable copy without changing the source or config.
func (s *Service) ReadResource(ctx context.Context, location string) (*ResourceContentResult, error) {
	if strings.TrimSpace(location) == "" {
		return nil, newBadRequestError("invalid_request", "资源 URL 或路径不能为空")
	}
	body, err := fetch.LoadResource(ctx, location, s.fetcher)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(body) || strings.ContainsRune(string(body), '\x00') {
		return nil, newBadRequestError("invalid_resource", "资源必须为 UTF-8 文本")
	}
	return &ResourceContentResult{Content: string(body), Format: pipeline.DetectSubscriptionFormat(location, body)}, nil
}
