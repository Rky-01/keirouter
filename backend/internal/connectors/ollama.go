package connectors

import (
	"context"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/transform"
)

// Ollama drives Ollama's native /api/chat endpoint (both Ollama Cloud and a
// local daemon). Ollama differs from OpenAI in two ways the connector handles:
// generation knobs live under "options" (mapped by the codec), and streaming is
// newline-delimited JSON (NDJSON) rather than SSE — so each response line is a
// bare JSON object with no "data:" prefix.
type Ollama struct {
	id          string
	defaultBase string
	codec       transform.OllamaCodec
}

// NewOllama builds an Ollama connector for the given provider id and default
// base URL (e.g. https://ollama.com or http://localhost:11434).
func NewOllama(id, defaultBaseURL string) *Ollama {
	return &Ollama{id: id, defaultBase: defaultBaseURL}
}

func (c *Ollama) ID() string            { return c.id }
func (c *Ollama) Dialect() core.Dialect { return core.DialectOllama }

func (c *Ollama) baseURL(creds core.Credentials) string {
	if creds.BaseURL != "" {
		return creds.BaseURL
	}
	return c.defaultBase
}

func (c *Ollama) headers(creds core.Credentials) map[string]string {
	h := map[string]string{}
	switch {
	case creds.AccessToken != "":
		h["Authorization"] = bearer(creds.AccessToken)
	case creds.APIKey != "":
		h["Authorization"] = bearer(creds.APIKey)
	}
	return mergeHeaders(h, creds.Headers)
}

// Chat performs a non-streaming /api/chat call.
func (c *Ollama) Chat(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (*core.ChatResponse, error) {
	req.Stream = false
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}

	url := joinURL(c.baseURL(creds), "api/chat")
	respBody, err := doJSON(ctx, c.id, req.Model, url, body, c.headers(creds))
	if err != nil {
		return nil, err
	}

	resp, err := c.codec.ParseResponse(respBody, req.Model)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}
	return resp, nil
}

// Stream performs a streaming /api/chat call. Ollama emits NDJSON, so each
// scanned line is passed directly to the codec (no SSE "data:" framing).
func (c *Ollama) Stream(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (<-chan core.StreamChunk, error) {
	req.Stream = true
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}

	url := joinURL(c.baseURL(creds), "api/chat")
	resp, err := openStream(ctx, c.id, req.Model, url, body, c.headers(creds))
	if err != nil {
		return nil, err
	}

	out := make(chan core.StreamChunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := sseScanner(resp.Body) // reuse the generous-buffer line scanner
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// NDJSON: pass the raw line straight to the codec.
			line := scanner.Bytes()
			chunks, perr := c.codec.ParseStreamLine(line, req.Model)
			if perr != nil {
				continue
			}
			for _, ch := range chunks {
				select {
				case out <- ch:
				case <-ctx.Done():
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- core.StreamChunk{
				Type: core.ChunkError,
				Err:  &core.ProviderError{Kind: core.ErrTimeout, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err},
			}
		}
	}()
	return out, nil
}