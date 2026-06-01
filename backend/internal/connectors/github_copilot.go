package connectors

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/transform"
)

// GitHub Copilot client fingerprint, mirroring 9router's GITHUB_COPILOT consts.
const (
	copilotVSCodeVersion = "1.110.0"
	copilotChatVersion   = "0.38.0"
	copilotUserAgent     = "GitHubCopilotChat/0.38.0"
	copilotAPIVersion    = "2025-04-01"
)

// GitHubCopilot drives GitHub Copilot's OpenAI-compatible /chat/completions
// endpoint. It speaks the OpenAI dialect but requires the full Copilot editor
// fingerprint headers, and applies model-specific request transforms (gpt-5/o-
// series need max_completion_tokens; some models reject temperature/thinking).
// This mirrors 9router's GithubExecutor. The copilot bearer token is supplied
// as the credential AccessToken (minted by the token refresher upstream).
type GitHubCopilot struct {
	id          string
	defaultBase string
	codec       transform.OpenAICodec
}

// NewGitHubCopilot builds a GitHub Copilot connector.
func NewGitHubCopilot(id, defaultBaseURL string) *GitHubCopilot {
	return &GitHubCopilot{id: id, defaultBase: defaultBaseURL}
}

func (c *GitHubCopilot) ID() string            { return c.id }
func (c *GitHubCopilot) Dialect() core.Dialect { return core.DialectOpenAI }

func (c *GitHubCopilot) baseURL(creds core.Credentials) string {
	if creds.BaseURL != "" {
		return creds.BaseURL
	}
	return c.defaultBase
}

func (c *GitHubCopilot) endpoint(creds core.Credentials) string {
	base := c.baseURL(creds)
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return joinURL(base, "chat/completions")
}

func (c *GitHubCopilot) headers(creds core.Credentials, stream bool) map[string]string {
	token := creds.AccessToken
	if token == "" {
		token = creds.APIKey
	}
	accept := "application/json"
	if stream {
		accept = "text/event-stream"
	}
	h := map[string]string{
		"Authorization":                       bearer(token),
		"copilot-integration-id":              "vscode-chat",
		"editor-version":                      "vscode/" + copilotVSCodeVersion,
		"editor-plugin-version":               "copilot-chat/" + copilotChatVersion,
		"user-agent":                          copilotUserAgent,
		"openai-intent":                       "conversation-panel",
		"x-github-api-version":                copilotAPIVersion,
		"x-request-id":                        uuid.NewString(),
		"x-vscode-user-agent-library-version": "electron-fetch",
		"X-Initiator":                         "user",
		"Accept":                              accept,
	}
	return mergeHeaders(h, creds.Headers)
}

var (
	reMaxCompletionModels = regexp.MustCompile(`(?i)gpt-5|o[134]-`)
	reNoTemperature       = regexp.MustCompile(`(?i)gpt-5\.4`)
	reClaude              = regexp.MustCompile(`(?i)claude`)
)

// transformBody applies Copilot's model-specific request adjustments to the
// rendered OpenAI body, mirroring GithubExecutor.transformRequest.
func (c *GitHubCopilot) transformBody(model string, body []byte) []byte {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	changed := false

	// gpt-5 / o-series require max_completion_tokens instead of max_tokens.
	if reMaxCompletionModels.MatchString(model) {
		if v, ok := m["max_tokens"]; ok {
			m["max_completion_tokens"] = v
			delete(m, "max_tokens")
			changed = true
		}
	}
	// Some models (gpt-5.4) reject temperature.
	if reNoTemperature.MatchString(model) {
		if _, ok := m["temperature"]; ok {
			delete(m, "temperature")
			changed = true
		}
	}
	// Claude models on Copilot reject Claude-style thinking payloads.
	if reClaude.MatchString(model) {
		if _, ok := m["thinking"]; ok {
			delete(m, "thinking")
			changed = true
		}
	}
	// "none" reasoning_effort is unsupported by some models.
	if m["reasoning_effort"] == "none" {
		delete(m, "reasoning_effort")
		changed = true
	}

	if !changed {
		return body
	}
	out, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return out
}

// Chat performs a non-streaming Copilot completion.
func (c *GitHubCopilot) Chat(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (*core.ChatResponse, error) {
	req.Stream = false
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}
	body = c.transformBody(req.Model, body)

	respBody, err := doJSON(ctx, c.id, req.Model, c.endpoint(creds), body, c.headers(creds, false))
	if err != nil {
		return nil, err
	}

	resp, err := c.codec.ParseResponse(respBody, req.Model)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}
	return resp, nil
}

// Stream performs a streaming Copilot completion.
func (c *GitHubCopilot) Stream(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (<-chan core.StreamChunk, error) {
	req.Stream = true
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}
	body = c.transformBody(req.Model, body)

	resp, err := openStream(ctx, c.id, req.Model, c.endpoint(creds), body, c.headers(creds, true))
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