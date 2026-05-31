package connectors

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/stretchr/testify/require"
)

func textReq(model string, stream bool) *core.ChatRequest {
	return &core.ChatRequest{
		Model:  model,
		Stream: stream,
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "hello"}}},
		},
	}
}

func TestOpenAICompatible_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"c1","model":"gpt-4o","choices":[{"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)
	}))
	defer srv.Close()

	c := NewOpenAICompatible("openai", srv.URL)
	resp, err := c.Chat(context.Background(), textReq("gpt-4o", false), core.Credentials{APIKey: "sk-test"})
	require.NoError(t, err)
	require.Equal(t, "hi there", resp.Message.TextContent())
	require.Equal(t, core.FinishStop, resp.FinishReason)
	require.Equal(t, 5, resp.Usage.TotalTokens)
}

func TestOpenAICompatible_Stream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flush, _ := w.(http.Flusher)
		lines := []string{
			`data: {"choices":[{"delta":{"role":"assistant","content":"he"}}]}`,
			`data: {"choices":[{"delta":{"content":"llo"}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}
		for _, l := range lines {
			fmt.Fprintf(w, "%s\n\n", l)
			if flush != nil {
				flush.Flush()
			}
		}
	}))
	defer srv.Close()

	c := NewOpenAICompatible("openai", srv.URL)
	ch, err := c.Stream(context.Background(), textReq("gpt-4o", true), core.Credentials{APIKey: "sk-test"})
	require.NoError(t, err)

	var text string
	var finished bool
	for chunk := range ch {
		switch chunk.Type {
		case core.ChunkText:
			text += chunk.Delta
		case core.ChunkFinish:
			finished = true
		case core.ChunkError:
			t.Fatalf("unexpected stream error: %v", chunk.Err)
		}
	}
	require.Equal(t, "hello", text)
	require.True(t, finished)
}

func TestOpenAICompatible_MapsRateLimitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limited"}`)
	}))
	defer srv.Close()

	c := NewOpenAICompatible("openai", srv.URL)
	_, err := c.Chat(context.Background(), textReq("gpt-4o", false), core.Credentials{APIKey: "sk-test"})
	require.Error(t, err)

	pe := core.AsProviderError(err)
	require.Equal(t, core.ErrRateLimit, pe.Kind)
	require.Equal(t, 429, pe.StatusCode)
	require.Equal(t, "openai", pe.Provider)
	require.True(t, pe.Fallbackable())
}

func TestOpenAICompatible_BadRequestNotFallbackable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"bad model"}`)
	}))
	defer srv.Close()

	c := NewOpenAICompatible("openai", srv.URL)
	_, err := c.Chat(context.Background(), textReq("bogus", false), core.Credentials{APIKey: "k"})
	pe := core.AsProviderError(err)
	require.Equal(t, core.ErrBadRequest, pe.Kind)
	require.False(t, pe.Fallbackable(), "4xx request errors must not trigger fallback")
}

func TestAnthropic_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/messages", r.URL.Path)
		require.Equal(t, "sk-ant", r.Header.Get("x-api-key"))
		require.Equal(t, anthropicVersion, r.Header.Get("anthropic-version"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"m1","model":"claude-x","content":[{"type":"text","text":"hello back"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":3}}`)
	}))
	defer srv.Close()

	c := NewAnthropic("anthropic", srv.URL)
	resp, err := c.Chat(context.Background(), textReq("claude-x", false), core.Credentials{APIKey: "sk-ant"})
	require.NoError(t, err)
	require.Equal(t, "hello back", resp.Message.TextContent())
	require.Equal(t, core.FinishStop, resp.FinishReason)
	require.Equal(t, 7, resp.Usage.TotalTokens)
}

func TestRegistry_DefaultProviders(t *testing.T) {
	reg := DefaultRegistry()
	require.True(t, reg.Has("openai"))
	require.True(t, reg.Has("anthropic"))

	c, err := reg.Get("anthropic")
	require.NoError(t, err)
	require.Equal(t, core.DialectAnthropic, c.Dialect())

	_, err = reg.Get("nonexistent")
	require.Error(t, err)
}