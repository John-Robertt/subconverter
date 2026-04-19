package server

import (
	"context"
	"net/http"

	"github.com/John-Robertt/subconverter/internal/generate"
)

// Generator abstracts the generation use case behind the HTTP transport.
type Generator interface {
	Generate(ctx context.Context, req generate.Request) (*generate.Result, error)
}

// Server holds the dependencies for the HTTP handlers.
type Server struct {
	generator Generator
	opts      Options
}

// Options holds runtime-only server behavior toggles.
type Options struct {
	AccessToken string
}

// New creates a Server with the given generator.
func New(generator Generator, opts Options) *Server {
	return &Server{generator: generator, opts: opts}
}

// Handler returns an http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /generate", s.handleGenerate)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	return mux
}
