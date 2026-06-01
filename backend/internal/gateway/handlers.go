package gateway

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/pipeline"
	"github.com/mydisha/keirouter/backend/internal/transform"
)

// maxBodyBytes caps inbound request bodies to protect against oversized uploads.
const maxBodyBytes = 32 << 20 // 32 MiB

// handleOpenAIChat serves /v1/chat/completions in the OpenAI dialect.
func (s *Server) handleOpenAIChat(w http.ResponseWriter, r *http.Request) {
	s.handleChat(w, r, core.DialectOpenAI)
}

// handleAnthropicMessages serves /v1/messages in the Anthropic dialect.
func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	s.handleChat(w, r, core.DialectAnthropic)
}

// handleOpenAIResponses serves /v1/responses in the OpenAI Responses dialect
// (Codex and Responses-native clients).
func (s *Server) handleOpenAIResponses(w http.ResponseWriter, r *http.Request) {
	s.handleChat(w, r, core.DialectOpenAIResponses)
}

// handleChat is the shared chat handler parameterized by the client dialect.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request, dialect core.Dialect) {
	key, _ := authedKey(r.Context())
	tenantID := tenantOf(key)

	codec, err := s.codecs.Codec(dialect)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unsupported dialect")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	req, err := codec.ParseRequest(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// Attach routing metadata.
	req.Metadata = core.RequestMetadata{
		ClientKind:    detectClient(r),
		SourceDialect: dialect,
		APIKeyID:      key.ID,
		TenantID:      tenantID,
		ProjectID:     key.ProjectID,
	}

	targets, err := resolveTargets(r.Context(), s.chains, tenantID, req.Model)
	if err != nil {
		var bad badModelError
		if errors.As(err, &bad) {
			writeError(w, http.StatusBadRequest, bad.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve model")
		return
	}

	opts := pipeline.Options{
		Targets: targets,
		Slimmer: s.slimmerConfig(),
		Terse:   s.terseConfig(),
		Caveman: s.cavemanConfig(),
	}

	if req.Stream {
		s.streamChat(w, r, codec, req, opts)
		return
	}
	s.unaryChat(w, r, codec, req, opts)
}

// unaryChat runs a non-streaming request and renders the response.
func (s *Server) unaryChat(w http.ResponseWriter, r *http.Request, codec transform.Codec, req *core.ChatRequest, opts pipeline.Options) {
	result, err := s.pipeline.Chat(r.Context(), req, opts)
	if err != nil {
		s.writeProviderError(w, err)
		return
	}

	out, err := codec.RenderResponse(result.Response)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to render response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-KeiRouter-Provider", result.Provider)
	w.Header().Set("X-KeiRouter-Model", result.Model)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

// streamChat runs a streaming request and relays SSE events in the client's
// dialect, honoring client disconnects and the configured stall timeout.
func (s *Server) streamChat(w http.ResponseWriter, r *http.Request, codec transform.Codec, req *core.ChatRequest, opts pipeline.Options) {
	streamCodec, ok := codec.(transform.StreamCodec)
	if !ok {
		writeError(w, http.StatusInternalServerError, "dialect does not support streaming")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported by server")
		return
	}

	result, err := s.pipeline.Stream(r.Context(), req, opts)
	if err != nil {
		s.writeProviderError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-KeiRouter-Provider", result.Provider)
	w.Header().Set("X-KeiRouter-Model", result.Model)
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	state := &transform.StreamState{Model: result.Model}

	for chunk := range result.Chunks {
		if chunk.Type == core.ChunkError {
			// Best-effort: surface a terminal error event then stop.
			s.log.Warn("stream error", "err", chunk.Err)
			break
		}
		events, rerr := streamCodec.RenderStreamChunk(chunk, state)
		if rerr != nil {
			s.log.Warn("failed to render stream chunk", "err", rerr)
			continue
		}
		for _, ev := range events {
			if _, werr := w.Write(ev); werr != nil {
				return // client disconnected
			}
		}
		flusher.Flush()
	}

	for _, ev := range streamCodec.RenderStreamDone(state) {
		_, _ = w.Write(ev)
	}
	flusher.Flush()
}

// writeProviderError maps a structured provider error to an HTTP status.
func (s *Server) writeProviderError(w http.ResponseWriter, err error) {
	pe := core.AsProviderError(err)
	status := http.StatusBadGateway
	switch pe.Kind {
	case core.ErrBadRequest:
		status = http.StatusBadRequest
	case core.ErrAuth:
		status = http.StatusUnauthorized
	case core.ErrRateLimit:
		status = http.StatusTooManyRequests
	case core.ErrQuotaExhausted, core.ErrBudgetBlocked:
		status = http.StatusPaymentRequired
	case core.ErrTimeout:
		status = http.StatusGatewayTimeout
	case core.ErrInternal:
		status = http.StatusInternalServerError
	}
	writeError(w, status, pe.Message)
}

// detectClient identifies the calling tool from request headers, used for
// telemetry and client-specific quirks. Best-effort; empty when unknown.
func detectClient(r *http.Request) string {
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	switch {
	case strings.Contains(ua, "claude"):
		return "claude-code"
	case strings.Contains(ua, "cursor"):
		return "cursor"
	case strings.Contains(ua, "codex"):
		return "codex"
	case r.Header.Get("x-stainless-lang") != "":
		return "openai-sdk"
	default:
		return ""
	}
}
