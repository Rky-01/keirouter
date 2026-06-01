package connectors

import (
	"context"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/transform"
)

// CommandCode drives the Command Code /alpha/generate endpoint. The base URL
// points directly at the generate endpoint, so the connector POSTs to it as-is.
// The request is a wrapped generate envelope (built by the codec) and the
// response is an AI SDK v5 NDJSON event stream (bare JSON objects per line, no
// SSE "data:" framing), so each line is handed straight to the codec. Mirrors
// 9router's CommandCode executor.
type CommandCode struct {
	id          string
	defaultBase string
	codec       transform.CommandCodeCodec
}

// NewCommandCode builds a Command Code connector.
func NewCommandCode(id, defaultBaseURL string) *CommandCode {
	return &CommandCode{id: id, defaultBase: defaultBaseURL}
}

func (c *CommandCode) ID() string            { return c.id }
func (c *CommandCode) Dialect() core.Dialect { return core.DialectCommandCode }

func (c *CommandCode) baseURL(creds core.Credentials) string {
	if creds.BaseURL != "" {
		return creds.BaseURL
	}
	return c.defaultBase
}

func (c *CommandCode) headers(creds core.Credentials) map[string]string {
	h := map[string]string{}
	switch {
	case creds.AccessToken != "":
		h["Authorization"] = bearer(creds.AccessToken)
	case creds.APIKey != "":
		h["Authorization"] = bearer(creds.APIKey)
	}
	return mergeHeaders(h, creds.Headers)
}

// Chat performs a non-streaming generate call.
func (c *CommandCode) Chat(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (*core.ChatResponse, error) {
	req.Stream = false
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}

	respBody, err := doJSON(ctx, c.id, req.Model, c.baseURL(creds), body, c.headers(creds))
	if err != nil {
		return nil, err
	}

	resp, err := c.codec.ParseResponse(respBody, req.Model)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}
	return resp, nil
}

// Stream performs a streaming generate call. Command Code emits NDJSON, so each
// scanned line is passed directly to the codec (no SSE "data:" framing).
func (c *CommandCode) Stream(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (<-chan core.StreamChunk, error) {
	req.Stream = true
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}

	resp, err := openStream(ctx, c.id, req.Model, c.baseURL(creds), body, c.headers(creds))
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

			// NDJSON: pass the raw line straight to the codec (it tolerates an
			// optional "data:" prefix).
			chunks, perr := c.codec.ParseStreamLine(scanner.Bytes(), req.Model)
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