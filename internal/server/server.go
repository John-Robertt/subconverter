package server

import (
	"net/http"

	"github.com/John-Robertt/subconverter/internal/config"
	"github.com/John-Robertt/subconverter/internal/fetch"
)

// Server holds the dependencies for the HTTP handlers.
type Server struct {
	cfg     *config.Config
	fetcher fetch.Fetcher
	opts    Options
}

// Options holds runtime-only server behavior toggles.
type Options struct {
	AccessToken string
}

// New creates a Server with the given configuration and fetcher.
func New(cfg *config.Config, fetcher fetch.Fetcher, opts Options) *Server {
	return &Server{cfg: cfg, fetcher: fetcher, opts: opts}
}

// Handler returns an http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /generate", s.handleGenerate)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	return mux
}
