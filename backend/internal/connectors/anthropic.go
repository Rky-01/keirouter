package connectors

import (
	"context"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/transform"
)

// anthropicVersion is the API version header Anthropic requires.
const anthropicVersion = "2023-06-01"

// Anthropic drives the native Anthropic Messages API (/v1/messages). It also
// backs Anthropic-compatible gateways via a custom base URL.
type Anthropic struct {
	id          string
	defaultBase string
	codec       transform.AnthropicCodec
}

// NewAnthropic builds an Anthropic connector.
func NewAnthropic(id, defaultBaseURL string) *Anthropic {
	return &Anthropic{id: id, defaultBase: defaultBaseURL}
}

func (c *Anthropic) ID() string            { return c.id }
func (c *Anthropic) Dialect() core.Dialect { return core.DialectAnthropic }

func (c *Anthropic) baseURL(creds core.Credentials) string {
	if creds.BaseURL != "" {
		return creds.BaseURL
	}
	return c.defaultBase
}

func (c *Anthropic) headers(creds core.Credentials) map[string]string {
	h := map[string]string{"anthropic-version": anthropicVersion}
	// Anthropic uses x-api-key for keys and Authorization: Bearer for OAuth.
	switch {
	case creds.AccessToken != "":
		h["Authorization"] = bearer(creds.AccessToken)
		// Claude subscription OAuth tokens require the full Claude Code CLI
		// fingerprint, or Anthropic rejects/flags them. Merge the spoof headers.
		if isClaudeOAuthToken(creds.AccessToken) {
			h = mergeHeaders(h, claudeCLISpoofHeaders())
		}
	case creds.APIKey != "":
		h["x-api-key"] = creds.APIKey
	}
	return mergeHeaders(h, creds.Headers)
}

// Chat performs a non-streaming completion.
func (c *Anthropic) Chat(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (*core.ChatResponse, error) {
	req.Stream = false
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}

	// Cloak the request to look like Claude Code when using a subscription OAuth
	// token. toolNameMap reverses the "_ide" tool renaming on the response.
	body, toolNameMap := applyClaudeCloaking(body, creds.AccessToken)

	url := joinURL(c.baseURL(creds), "messages")
	respBody, err := doJSON(ctx, c.id, req.Model, url, body, c.headers(creds))
	if err != nil {
		return nil, err
	}

	respBody = decloakClaudeToolNames(respBody, toolNameMap)
	resp, err := c.codec.ParseResponse(respBody, req.Model)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}
	return resp, nil
}

// Stream performs a streaming completion. Anthropic emits named SSE events; the
// codec maps each event's data payload to canonical chunks.
func (c *Anthropic) Stream(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (<-chan core.StreamChunk, error) {
	req.Stream = true
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}

	body, toolNameMap := applyClaudeCloaking(body, creds.AccessToken)

	url := joinURL(c.baseURL(creds), "messages")
	resp, err := openStream(ctx, c.id, req.Model, url, body, c.headers(creds))
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
				// Decloak streamed tool-call names ("foo_ide" → "foo").
				if ch.Type == core.ChunkToolCall && ch.ToolCall != nil && len(toolNameMap) > 0 {
					if orig, ok := toolNameMap[ch.ToolCall.Name]; ok {
						ch.ToolCall.Name = orig
					}
				}
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