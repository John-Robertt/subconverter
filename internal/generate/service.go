package generate

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/model"
	"github.com/John-Robertt/subconverter/internal/pipeline"
	"github.com/John-Robertt/subconverter/internal/render"
	"github.com/John-Robertt/subconverter/internal/target"
)

// Request describes one configuration generation request after transport-level
// validation has already been performed.
type Request struct {
	Format   string
	Filename string
}

// Result is the generated config payload plus response metadata expected by
// the transport layer.
type Result struct {
	Filename    string
	ContentType string
	Body        []byte
}

// Options holds runtime-only generation behavior.
type Options struct {
	AccessToken string
}

// Service executes the full generation use case from format-agnostic build to
// target projection and final rendering.
type Service struct {
	cfg     *config.RuntimeConfig
	fetcher fetch.Fetcher
	opts    Options
}

// New creates a generation service for one loaded/validated config.
func New(cfg *config.RuntimeConfig, fetcher fetch.Fetcher, opts Options) *Service {
	return &Service{cfg: cfg, fetcher: fetcher, opts: opts}
}

// Generate builds the shared Pipeline, projects it to the requested target,
// loads the optional base template, and renders the final output.
func (s *Service) Generate(ctx context.Context, req Request) (*Result, error) {
	p, err := pipeline.Build(ctx, s.cfg, s.fetcher)
	if err != nil {
		return nil, err
	}

	switch req.Format {
	case "clash":
		return s.generateClash(ctx, p, req)
	case "surge":
		return s.generateSurge(ctx, p, req)
	default:
		return nil, fmt.Errorf("unsupported format %q", req.Format)
	}
}

func (s *Service) generateClash(ctx context.Context, p *model.Pipeline, req Request) (*Result, error) {
	projected, err := target.ForClash(p)
	if err != nil {
		return nil, err
	}

	templates := s.cfg.Templates()
	tmpl, err := s.loadTemplate(ctx, templates.Clash)
	if err != nil {
		return nil, err
	}
	body, err := render.Clash(projected, tmpl)
	if err != nil {
		return nil, err
	}

	return &Result{
		Filename:    req.Filename,
		ContentType: "text/yaml; charset=utf-8",
		Body:        body,
	}, nil
}

func (s *Service) generateSurge(ctx context.Context, p *model.Pipeline, req Request) (*Result, error) {
	projected, err := target.ForSurge(p)
	if err != nil {
		return nil, err
	}

	templates := s.cfg.Templates()
	tmpl, err := s.loadTemplate(ctx, templates.Surge)
	if err != nil {
		return nil, err
	}
	body, err := render.Surge(projected, buildManagedURL(s.cfg.BaseURL(), req.Filename, s.opts.AccessToken), tmpl)
	if err != nil {
		return nil, err
	}

	return &Result{
		Filename:    req.Filename,
		ContentType: "text/plain; charset=utf-8",
		Body:        body,
	}, nil
}

func (s *Service) loadTemplate(ctx context.Context, location string) ([]byte, error) {
	if location == "" {
		return nil, nil
	}
	return fetch.LoadResource(ctx, location, s.fetcher)
}

func buildManagedURL(baseURL, filename, accessToken string) string {
	if baseURL == "" {
		return ""
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	base.Path = "/generate"
	base.RawPath = ""

	params := []string{"format=surge"}
	if accessToken != "" {
		params = append(params, "token="+url.QueryEscape(accessToken))
	}
	params = append(params, "filename="+url.QueryEscape(filename))
	base.RawQuery = strings.Join(params, "&")
	return base.String()
}
