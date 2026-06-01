package connectors

import (
	"context"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/transform"
)

// OpenAIResponses drives the OpenAI Responses API (/v1/responses), the dialect
// used by Codex and Responses-native clients. The base URL points directly at
// the responses endpoint (e.g. https://chatgpt.com/backend-api/codex/responses
// for Codex, or https://api.openai.com/v1/responses for the public API), so the
// connector POSTs to the base URL itself rather than appending a path. It reads
// the rich Responses SSE event stream via the codec.
type OpenAIResponses struct {
	id          string
	defaultBase string
	codec       transform.OpenAIResponsesCodec
}

// NewOpenAIResponses builds a Responses connector.
func NewOpenAIResponses(id, defaultBaseURL string) *OpenAIResponses {
	return &OpenAIResponses{id: id, defaultBase: defaultBaseURL}
}

func (c *OpenAIResponses) ID() string            { return c.id }
func (c *OpenAIResponses) Dialect() core.Dialect { return core.DialectOpenAIResponses }

func (c *OpenAIResponses) baseURL(creds core.Credentials) string {
	if creds.BaseURL != "" {
		return creds.BaseURL
	}
	return c.defaultBase
}

// endpoint returns the responses URL. If the configured base already ends in
// "/responses" it is used as-is; otherwise "/responses" is appended (so a plain
// "https://api.openai.com/v1" base also works).
func (c *OpenAIResponses) endpoint(creds core.Credentials) string {
	base := c.baseURL(creds)
	if hasResponsesSuffix(base) {
		return base
	}
	return joinURL(base, "responses")
}

func hasResponsesSuffix(u string) bool {
	const suf = "/responses"
	if len(u) < len(suf) {
		return false
	}
	return u[len(u)-len(suf):] == suf
}

func (c *OpenAIResponses) headers(creds core.Credentials) map[string]string {
	h := map[string]string{}
	switch {
	case creds.AccessToken != "":
		h["Authorization"] = bearer(creds.AccessToken)
	case creds.APIKey != "":
		h["Authorization"] = bearer(creds.APIKey)
	}
	return mergeHeaders(h, creds.Headers)
}

// Chat performs a non-streaming Responses call.
func (c *OpenAIResponses) Chat(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (*core.ChatResponse, error) {
	req.Stream = false
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}

	respBody, err := doJSON(ctx, c.id, req.Model, c.endpoint(creds), body, c.headers(creds))
	if err != nil {
		return nil, err
	}

	resp, err := c.codec.ParseResponse(respBody, req.Model)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}
	return resp, nil
}

// Stream performs a streaming Responses call, reading the typed SSE event stream.
func (c *OpenAIResponses) Stream(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (<-chan core.StreamChunk, error) {
	req.Stream = true
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}

	resp, err := openStream(ctx, c.id, req.Model, c.endpoint(creds), body, c.headers(creds))
	if err != nil {
		return nil, err
	}

	out := make(chan core.StreamChunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := sseScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			payload, ok := parseSSEData(scanner.Text())
			if !ok {
				continue
			}
			chunks, perr := c.codec.ParseStreamLine([]byte(payload), req.Model)
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