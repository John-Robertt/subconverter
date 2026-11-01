package app

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/John-Robertt/subconverter/internal/errtype"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/pipeline"
)

type CheckSubscriptionInput struct {
	Location       string `json:"location,omitempty"`
	Content        string `json:"content,omitempty"`
	SourceLocation string `json:"source_location,omitempty"`
	Format         string `json:"format,omitempty"`
}

type CheckSubscriptionResult struct {
	Status        string   `json:"status"`
	Format        string   `json:"format,omitempty"`
	NodeCount     int      `json:"node_count"`
	OutputFormats []string `json:"output_formats"`
	Candidates    []string `json:"candidates,omitempty"`
	Content       string   `json:"content"`
}

// CheckSubscription reads and parses a copy; it never saves a config or changes runtime state.
func (s *Service) CheckSubscription(ctx context.Context, input CheckSubscriptionInput) (*CheckSubscriptionResult, error) {
	input.Location = strings.TrimSpace(input.Location)
	if (input.Location == "") == (strings.TrimSpace(input.Content) == "") {
		return nil, newBadRequestError("invalid_request", "location 与 content 必须且只能填写一个")
	}
	switch input.Format {
	case "", "auto", "text", "clash", "surge", "loon":
	default:
		return nil, newBadRequestError("invalid_request", "订阅格式无效")
	}
	body := []byte(input.Content)
	origin := input.Location
	if origin == "" {
		origin = strings.TrimSpace(input.SourceLocation)
	}
	inline := origin == "" || !isRemoteConfig(origin)
	if input.Location != "" {
		var err error
		body, err = fetch.LoadResource(ctx, input.Location, s.fetcher)
		if err != nil {
			var upstream *errtype.FetchError
			if errors.As(err, &upstream) {
				copy := *upstream
				if upstream.Code == errtype.CodeFetchRequestFailed {
					copy.Message = "订阅读取失败，请检查地址或稍后重试"
				}
				return nil, &copy
			}
			var local *errtype.ResourceError
			if errors.As(err, &local) {
				copy := *local
				copy.Message = "无法读取订阅文件，请检查路径和读取权限：" + local.Message
				return nil, &copy
			}
			return nil, err
		}
	}
	if !utf8.Valid(body) || strings.ContainsRune(string(body), '\x00') {
		return nil, newBadRequestError(string(errtype.CodeFetchSubscriptionFormatInvalid), "订阅必须为 UTF-8 文本")
	}
	inspection, err := pipeline.InspectSubscription(origin, body, input.Format, inline)
	if err != nil {
		var parseErr *errtype.FetchError
		if errors.As(err, &parseErr) {
			return nil, newBadRequestError(string(parseErr.Code), parseErr.Message)
		}
		return nil, newBadRequestError(string(errtype.CodeFetchSubscriptionFormatInvalid), "无法提取订阅节点，请检查内容或更正格式")
	}
	result := &CheckSubscriptionResult{Status: "recognized", Format: inspection.Format, NodeCount: len(inspection.Proxies), OutputFormats: []string{}, Content: string(body)}
	if len(inspection.Candidates) > 0 {
		result.Status = "ambiguous"
		result.Candidates = inspection.Candidates
	} else {
		result.OutputFormats = s.generator.CheckProxyFormats(ctx, inspection.Proxies)
	}
	return result, nil
}
