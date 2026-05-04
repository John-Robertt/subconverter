package server

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	webUICacheControlHTML  = "no-cache"
	webUICacheControlAsset = "public, max-age=31536000, immutable"
)

func (s *Server) webUIFallback(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldServeWebUI(r) {
			s.handleWebUI(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func shouldServeWebUI(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}

	switch {
	case r.URL.Path == "/api", strings.HasPrefix(r.URL.Path, "/api/"):
		return false
	case r.URL.Path == "/generate", strings.HasPrefix(r.URL.Path, "/generate/"):
		return false
	case r.URL.Path == "/healthz", strings.HasPrefix(r.URL.Path, "/healthz/"):
		return false
	default:
		return true
	}
}

func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	if s.opts.WebFS == nil {
		http.NotFound(w, r)
		return
	}

	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "." || name == "" {
		s.serveWebUIFile(w, r, "index.html", webUICacheControlHTML)
		return
	}

	if name == "assets" || strings.HasPrefix(name, "assets/") {
		if !webUIFileExists(s.opts.WebFS, name) {
			http.NotFound(w, r)
			return
		}
		s.serveWebUIFile(w, r, name, webUICacheControlAsset)
		return
	}

	if name == "index.html" || !webUIFileExists(s.opts.WebFS, name) {
		s.serveWebUIFile(w, r, "index.html", webUICacheControlHTML)
		return
	}

	s.serveWebUIFile(w, r, name, webUICacheControlHTML)
}

func (s *Server) serveWebUIFile(w http.ResponseWriter, r *http.Request, name, cacheControl string) {
	body, err := fs.ReadFile(s.opts.WebFS, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", cacheControl)
	http.ServeContent(w, r, path.Base(name), time.Time{}, bytes.NewReader(body))
}

func webUIFileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
