package server

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/pipeline"
	"github.com/John-Robertt/subconverter/internal/render"
)

const maxFilenameLength = 255

// handleGenerate executes the pipeline and renders output in the requested format.
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	format := query.Get("format")
	if format != "clash" && format != "surge" {
		http.Error(w, "invalid or missing format parameter: must be clash or surge", http.StatusBadRequest)
		return
	}

	if !s.isAuthorized(query.Get("token")) {
		http.Error(w, "missing or invalid token", http.StatusUnauthorized)
		return
	}

	filename, err := resolveFilename(query, format)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p, err := pipeline.Execute(r.Context(), s.cfg, s.fetcher)
	if err != nil {
		code, msg := mapError(err)
		log.Printf("pipeline error: %v", err)
		http.Error(w, msg, code)
		return
	}

	// Load template if configured.
	var templatePath string
	switch format {
	case "clash":
		templatePath = s.cfg.Templates.Clash
	case "surge":
		templatePath = s.cfg.Templates.Surge
	}

	var tmpl []byte
	if templatePath != "" {
		tmpl, err = fetch.LoadResource(r.Context(), templatePath, s.fetcher)
		if err != nil {
			code, msg := mapError(err)
			log.Printf("template load error: %v", err)
			http.Error(w, msg, code)
			return
		}
	}

	// Render.
	var output []byte
	switch format {
	case "clash":
		output, err = render.Clash(p, tmpl)
	case "surge":
		output, err = render.Surge(p, buildManagedURL(s.cfg.BaseURL, filename, s.opts.AccessToken), tmpl)
	}
	if err != nil {
		code, msg := mapError(err)
		log.Printf("render error: %v", err)
		http.Error(w, msg, code)
		return
	}

	// Write response.
	switch format {
	case "clash":
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	case "surge":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.Header().Set("Content-Disposition", contentDispositionValue(filename))
	_, _ = w.Write(output)
}

// handleHealthz returns 200 OK for health checks.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) isAuthorized(providedToken string) bool {
	if s.opts.AccessToken == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(providedToken), []byte(s.opts.AccessToken)) == 1
}

func resolveFilename(query url.Values, format string) (string, error) {
	raw := defaultFilename(format)
	if _, ok := query["filename"]; ok {
		raw = query.Get("filename")
		if raw == "" {
			return "", fmt.Errorf("invalid filename parameter: cannot be empty")
		}
	}

	if len(raw) > maxFilenameLength {
		return "", fmt.Errorf("invalid filename parameter: too long")
	}
	if err := validateFilename(raw); err != nil {
		return "", err
	}

	ext := expectedExtension(format)
	name := raw
	currentExt := path.Ext(name)
	if currentExt == "" {
		name += ext
		currentExt = ext
	}
	if !strings.EqualFold(currentExt, ext) {
		return "", fmt.Errorf("invalid filename parameter: %s files must use %s", format, ext)
	}
	if base := strings.TrimSuffix(name, currentExt); strings.Trim(base, ".") == "" {
		return "", fmt.Errorf("invalid filename parameter: basename is required")
	}
	return name, nil
}

func validateFilename(name string) error {
	for _, r := range name {
		switch {
		case r > 127:
			return fmt.Errorf("invalid filename parameter: only ASCII letters, digits, dot, dash, and underscore are allowed")
		case r < 32 || r == 127:
			return fmt.Errorf("invalid filename parameter: control characters are not allowed")
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_':
		default:
			return fmt.Errorf("invalid filename parameter: only ASCII letters, digits, dot, dash, and underscore are allowed")
		}
	}
	return nil
}

func defaultFilename(format string) string {
	switch format {
	case "clash":
		return "clash.yaml"
	case "surge":
		return "surge.conf"
	default:
		return "download"
	}
}

func expectedExtension(format string) string {
	switch format {
	case "clash":
		return ".yaml"
	case "surge":
		return ".conf"
	default:
		return ""
	}
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

func contentDispositionValue(filename string) string {
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		filename,
		url.PathEscape(filename),
	)
}
