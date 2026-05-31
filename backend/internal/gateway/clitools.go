package gateway

import (
	"fmt"
	"net/http"
)

// cliTool describes how to point a coding tool at KeiRouter. Snippets are
// generated against the running server's address so they are copy-paste ready.
type cliTool struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Dialect  string `json:"dialect"` // openai | anthropic
	Instruct string `json:"instructions"`
	Snippet  string `json:"snippet"`
}

// handleCLITools returns config snippets for supported coding tools, wired to
// this server's base URL. The caller supplies the model (provider/model or
// chain:name) to embed.
func (s *Server) handleCLITools(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	if model == "" {
		model = "openai/gpt-4o"
	}
	base := s.publicBaseURL(r)

	tools := []cliTool{
		{
			ID: "openai-sdk", Name: "OpenAI SDK / generic", Dialect: "openai",
			Instruct: "Set the base URL and API key for any OpenAI-compatible client.",
			Snippet: fmt.Sprintf("export OPENAI_BASE_URL=%s/v1\nexport OPENAI_API_KEY=<your kr_ key>\n# model: %s",
				base, model),
		},
		{
			ID: "claude-code", Name: "Claude Code", Dialect: "anthropic",
			Instruct: "Point Claude Code at KeiRouter's Anthropic-compatible endpoint.",
			Snippet: fmt.Sprintf("export ANTHROPIC_BASE_URL=%s\nexport ANTHROPIC_API_KEY=<your kr_ key>\n# model: %s",
				base, model),
		},
		{
			ID: "cursor", Name: "Cursor", Dialect: "openai",
			Instruct: "Cursor → Settings → Models → Override OpenAI Base URL.",
			Snippet: fmt.Sprintf("Base URL: %s/v1\nAPI Key:  <your kr_ key>\nModel:    %s",
				base, model),
		},
		{
			ID: "codex", Name: "Codex CLI", Dialect: "openai",
			Instruct: "Export the OpenAI-compatible variables before running codex.",
			Snippet: fmt.Sprintf("export OPENAI_BASE_URL=%s\nexport OPENAI_API_KEY=<your kr_ key>",
				base),
		},
		{
			ID: "cline", Name: "Cline / Roo", Dialect: "openai",
			Instruct: "Choose the OpenAI Compatible provider in settings.",
			Snippet: fmt.Sprintf("Base URL: %s/v1\nAPI Key:  <your kr_ key>\nModel ID: %s",
				base, model),
		},
	}

	writeJSON(w, http.StatusOK, map[string]any{"base_url": base, "model": model, "tools": tools})
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