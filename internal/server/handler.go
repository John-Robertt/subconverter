package server

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/John-Robertt/subconverter/internal/generate"
)

// handleGenerate executes the pipeline and renders output in the requested format.
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	format := query.Get("format")
	if !generate.ValidFormat(format) {
		writeError(w, http.StatusBadRequest, "format 参数无效：必须为 clash 或 surge")
		return
	}

	if !s.isAuthorized(r, query.Get("token")) {
		writeError(w, http.StatusUnauthorized, "访问令牌缺失或无效")
		return
	}

	filename, err := resolveFilename(query, format)
	if err != nil {
		code, msg := presentError(err)
		writeError(w, code, msg)
		return
	}

	result, err := s.generator.Generate(r.Context(), generate.Request{
		Format:   format,
		Filename: filename,
	})
	if err != nil {
		code, msg := presentError(err)
		log.Printf("generate error: %v", err)
		writeError(w, code, msg)
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", contentDispositionValue(result.Filename))
	_, _ = w.Write(result.Body)
}

// handleHealthz returns 200 OK for health checks.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) isAuthorized(r *http.Request, providedToken string) bool {
	if s.opts.AdminSessionValidator != nil && s.opts.AdminSessionValidator(r) {
		return true
	}
	if s.opts.AccessToken == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(providedToken), []byte(s.opts.AccessToken)) == 1
}

func resolveFilename(query url.Values, format string) (string, error) {
	_, present := query["filename"]
	name, err := generate.ResolveFilename(query.Get("filename"), present, format)
	if err != nil {
		return "", badRequest(err.Error())
	}
	return name, nil
}

func contentDispositionValue(filename string) string {
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		filename,
		url.PathEscape(filename),
	)
}
