package generate

import (
	"context"
	"fmt"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/pipeline"
	"github.com/John-Robertt/subconverter/internal/render"
	"github.com/John-Robertt/subconverter/internal/target"
)

// Request 描述一次已通过传输层校验的配置生成请求。
type Request struct {
	Format   string
	Filename string
}

// Result 是生成的配置内容，以及传输层需要的响应元数据。
type Result struct {
	Filename    string
	ContentType string
	Body        []byte
}

// Options 保存只在运行时生效的生成行为设置。
type Options struct {
	AccessToken string
}

// Service 执行完整的生成用例：从与格式无关的构建，到目标投影，
// 再到最终渲染。
type Service struct {
	fetcher fetch.Fetcher
	opts    Options
}

// New 创建一个无状态的生成服务。
func New(fetcher fetch.Fetcher, opts Options) *Service {
	return &Service{fetcher: fetcher, opts: opts}
}

// Generate 构建共享的 Pipeline，投影到请求的目标格式，
// 加载可选的底版模板，并渲染最终输出。
func (s *Service) Generate(ctx context.Context, cfg *config.RuntimeConfig, req Request) (*Result, error) {
	p, err := pipeline.Build(ctx, cfg, s.fetcher)
	if err != nil {
		return nil, err
	}

	projected, err := projectPipeline(p, req.Format)
	if err != nil {
		return nil, err
	}
	return s.renderPipeline(ctx, cfg, projected, req)
}

func projectPipeline(p *model.Pipeline, format string) (*model.TargetPipeline, error) {
	switch format {
	case "clash":
		return target.ForClash(p)
	case "surge":
		return target.ForSurge(p)
	case "loon":
		return target.ForLoon(p)
	default:
		return nil, fmt.Errorf("不支持的格式 %q", format)
	}
}

func (s *Service) renderPipeline(ctx context.Context, cfg *config.RuntimeConfig, p *model.TargetPipeline, req Request) (*Result, error) {
	templates := cfg.Templates()
	var location string
	switch req.Format {
	case "clash":
		location = templates.Clash
	case "surge":
		location = templates.Surge
	case "loon":
		location = templates.Loon
	}
	tmpl, err := s.loadTemplate(ctx, location, templates.Inline[req.Format])
	if err != nil {
		return nil, err
	}
	var body []byte
	contentType := "text/plain; charset=utf-8"
	switch req.Format {
	case "clash":
		body, err = render.Clash(p, tmpl)
		contentType = "text/yaml; charset=utf-8"
	case "surge":
		body, err = render.Surge(p, buildManagedURL(cfg.BaseURL(), req.Filename, s.opts.AccessToken), tmpl)
	case "loon":
		body, err = render.Loon(p, tmpl)
	default:
		return nil, fmt.Errorf("不支持的格式 %q", req.Format)
	}
	if err != nil {
		return nil, err
	}
	return &Result{Filename: req.Filename, ContentType: contentType, Body: body}, nil
}

// CheckProxyFormats 在不使用模板、不访问网络的情况下，检查每种格式能否完整保留并渲染全部节点。
func (s *Service) CheckProxyFormats(ctx context.Context, proxies []model.Proxy) []string {
	formats := make([]string, 0, 3)
	p := &model.Pipeline{Proxies: proxies, Fallback: "DIRECT"}
	for _, format := range []string{"clash", "surge", "loon"} {
		projected, err := projectPipeline(p, format)
		if err != nil || len(projected.Proxies) != len(proxies) {
			continue
		}
		if _, err := s.renderPipeline(ctx, nil, projected, Request{Format: format, Filename: DefaultFilename(format)}); err == nil {
			formats = append(formats, format)
		}
	}
	return formats
}

func (s *Service) loadTemplate(ctx context.Context, location, content string) ([]byte, error) {
	if content != "" {
		return []byte(content), nil
	}
	if location == "" {
		return nil, nil
	}
	return fetch.LoadResource(ctx, location, s.fetcher)
}

func buildManagedURL(baseURL, filename, accessToken string) string {
	if baseURL == "" {
		return ""
	}
	managedURL, err := BuildGenerateURL(baseURL, "surge", filename, accessToken, accessToken != "")
	if err != nil {
		return ""
	}
	return managedURL
}
