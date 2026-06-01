package gateway

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleCLITools returns the status of all supported CLI tools.
func (s *Server) handleCLITools(w http.ResponseWriter, r *http.Request) {
	statuses := s.cliTools.DetectAll(s.cliToolHome)
	writeJSON(w, http.StatusOK, map[string]any{"tools": statuses})
}

// handleCLIToolConfigure writes KeiRouter config into a specific tool.
func (s *Server) handleCLIToolConfigure(w http.ResponseWriter, r *http.Request) {
	toolID := chi.URLParam(r, "toolId")
	tool := s.cliTools.Get(toolID)
	if tool == nil {
		writeError(w, http.StatusNotFound, "unknown tool: "+toolID)
		return
	}

	var body struct {
		BaseURL string   `json:"base_url"`
		APIKey  string   `json:"api_key"`
		Models  []string `json:"models"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "base_url is required")
		return
	}
	if body.APIKey == "" {
		writeError(w, http.StatusBadRequest, "api_key is required")
		return
	}

	if err := tool.Configure(s.cliToolHome, body.BaseURL, body.APIKey, body.Models); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleCLIToolRemove strips KeiRouter config from a specific tool.
func (s *Server) handleCLIToolRemove(w http.ResponseWriter, r *http.Request) {
	toolID := chi.URLParam(r, "toolId")
	tool := s.cliTools.Get(toolID)
	if tool == nil {
		writeError(w, http.StatusNotFound, "unknown tool: "+toolID)
		return
	}

	if err := tool.Remove(s.cliToolHome); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// publicBaseURL derives the externally usable base URL from the request. It
// honors a forwarded host/proto when present (reverse proxy), else falls back
// to the configured listen address.
func (s *Server) publicBaseURL(r *http.Request) string {
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	if host == "" {
		host = s.cfg.Addr()
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

// mountCLITools registers the CLI tool auto-config endpoints.
func (s *Server) mountCLITools(r chi.Router) {
	r.Get("/cli-tools", s.handleCLITools)
	r.Post("/cli-tools/{toolId}/configure", s.handleCLIToolConfigure)
	r.Post("/cli-tools/{toolId}/remove", s.handleCLIToolRemove)
}
