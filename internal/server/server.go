package server

import (
	"context"
	"io/fs"
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
	AccessToken           string
	AdminHandler          http.Handler
	AdminSessionValidator func(*http.Request) bool
	EnableCORS            bool
	WebFS                 fs.FS
	// RequestCounter, if set, is invoked once per inbound request and feeds
	// the runtime environment surface on /api/status.
	RequestCounter func()
}

// New creates a Server with the given generator.
func New(generator Generator, opts Options) *Server {
	return &Server{generator: generator, opts: opts}
}

// Handler returns an http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	if s.opts.AdminHandler != nil {
		mux.Handle("/api/", s.opts.AdminHandler)
	}
	mux.HandleFunc("GET /generate", s.handleGenerate)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	var handler http.Handler = mux
	if s.opts.WebFS != nil {
		handler = s.webUIFallback(handler)
	}
	if s.opts.RequestCounter != nil {
		handler = requestCounterMiddleware(handler, s.opts.RequestCounter)
	}
	if s.opts.EnableCORS {
		handler = corsMiddleware(handler)
	}
	return handler
}

func requestCounterMiddleware(next http.Handler, counter func()) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CORS preflight and the health probe so the counter reflects
		// real client / admin traffic only.
		if r.Method != http.MethodOptions && r.URL.Path != "/healthz" {
			counter()
		}
		next.ServeHTTP(w, r)
	})
}
