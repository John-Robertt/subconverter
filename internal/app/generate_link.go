package app

import (
	"context"

	"github.com/John-Robertt/subconverter/internal/generate"
)

type GenerateLinkInput struct {
	Format       string
	Filename     string
	IncludeToken bool
}

type GenerateLinkResult struct {
	URL           string `json:"url"`
	TokenIncluded bool   `json:"token_included"`
}

func (s *Service) GenerateLink(_ context.Context, input *GenerateLinkInput) (*GenerateLinkResult, error) {
	if input == nil {
		return nil, newBadRequestError("invalid_request", "请求参数无效")
	}
	if !generate.ValidFormat(input.Format) {
		return nil, newBadRequestError("invalid_request", "format 参数无效：必须为 clash 或 surge")
	}
	filename, err := generate.ResolveFilename(input.Filename, input.Filename != "", input.Format)
	if err != nil {
		return nil, newBadRequestError("invalid_request", err.Error())
	}
	cfg := s.runtimeSnapshot()
	baseURL := cfg.BaseURL()
	if baseURL == "" {
		return nil, newBadRequestError("base_url_required", "当前配置未声明 base_url")
	}

	includeToken := input.IncludeToken && s.accessToken != ""
	link, err := generate.BuildGenerateURL(baseURL, input.Format, filename, s.accessToken, includeToken)
	if err != nil {
		return nil, newBadRequestError("base_url_invalid", "base_url 无效")
	}
	return &GenerateLinkResult{
		URL:           link,
		TokenIncluded: includeToken,
	}, nil
}
