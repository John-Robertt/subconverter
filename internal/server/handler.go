package server

import (
	"log"
	"net/http"

	"github.com/John-Robertt/subconverter/internal/fetch"
	"github.com/John-Robertt/subconverter/internal/pipeline"
	"github.com/John-Robertt/subconverter/internal/render"
)

// handleGenerate executes the pipeline and renders output in the requested format.
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "clash" && format != "surge" {
		http.Error(w, "invalid or missing format parameter: must be clash or surge", http.StatusBadRequest)
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
		output, err = render.Surge(p, s.cfg.BaseURL, tmpl)
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
	w.Write(output)
}

// handleHealthz returns 200 OK for health checks.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
