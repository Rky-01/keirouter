package transform

import (
	"encoding/json"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/stretchr/testify/require"
)

func TestOpenAI_ParseRequest_BasicChat(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "be helpful"},
			{"role": "user", "content": "hello"}
		],
		"stream": true,
		"max_tokens": 256
	}`)

	req, err := OpenAICodec{}.ParseRequest(body)
	require.NoError(t, err)
	require.Equal(t, "gpt-4o", req.Model)
	require.Equal(t, "be helpful", req.System)
	require.True(t, req.Stream)
	require.NotNil(t, req.MaxTokens)
	require.Equal(t, 256, *req.MaxTokens)
	require.Len(t, req.Messages, 1)
	require.Equal(t, core.RoleUser, req.Messages[0].Role)
	require.Equal(t, "hello", req.Messages[0].TextContent())
}

func TestOpenAI_ParseRequest_ToolCallAndResult(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "assistant", "content": null, "tool_calls": [
				{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"SF\"}"}}
			]},
			{"role": "tool", "tool_call_id": "call_1", "content": "sunny, 22C"}
		]
	}`)

	req, err := OpenAICodec{}.ParseRequest(body)
	require.NoError(t, err)
	require.Len(t, req.Messages, 2)

	tc := req.Messages[0].Content[0]
	require.Equal(t, core.PartToolCall, tc.Type)
	require.Equal(t, "get_weather", tc.ToolCall.Name)

	tr := req.Messages[1].Content[0]
	require.Equal(t, core.PartToolResult, tr.Type)
	require.Equal(t, "call_1", tr.ToolResult.CallID)
	require.Equal(t, "sunny, 22C", tr.ToolResult.Content)
}

// Cross-dialect: an OpenAI request rendered to Anthropic and parsed back must
// preserve system, messages, and tool structure.
func TestCrossDialect_OpenAIToAnthropicRoundTrip(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4.5",
		"messages": [
			{"role": "system", "content": "you are precise"},
			{"role": "user", "content": "what is 2+2?"},
			{"role": "assistant", "content": "4"},
			{"role": "user", "content": "and 3+3?"}
		],
		"max_tokens": 100
	}`)

	canonical, err := OpenAICodec{}.ParseRequest(body)
	require.NoError(t, err)

	antBody, err := AnthropicCodec{}.RenderRequest(canonical)
	require.NoError(t, err)

	// Anthropic body must hoist system and carry alternating roles.
	var antReq map[string]any
	require.NoError(t, json.Unmarshal(antBody, &antReq))
	require.Equal(t, "you are precise", antReq["system"])
	require.Equal(t, float64(100), antReq["max_tokens"])

	// Parse the Anthropic body back to canonical and compare essentials.
	back, err := AnthropicCodec{}.ParseRequest(antBody)
	require.NoError(t, err)
	require.Equal(t, "you are precise", back.System)
	require.Len(t, back.Messages, 3)
	require.Equal(t, "what is 2+2?", back.Messages[0].TextContent())
	require.Equal(t, core.RoleAssistant, back.Messages[1].Role)
	require.Equal(t, "and 3+3?", back.Messages[2].TextContent())
}

func TestAnthropic_MergesConsecutiveSameRole(t *testing.T) {
	// Two consecutive user messages (e.g. text + tool result) must merge into
	// one Anthropic message, since the API forbids consecutive same-role turns.
	req := &core.ChatRequest{
		Model: "claude-x",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "first"}}},
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "second"}}},
		},
	}
	body, err := AnthropicCodec{}.RenderRequest(req)
	require.NoError(t, err)

	var parsed struct {
		Messages []json.RawMessage `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.Len(t, parsed.Messages, 1, "consecutive user messages must merge")
}

func TestOpenAI_ResponseRoundTrip(t *testing.T) {
	resp := &core.ChatResponse{
		ID:    "resp1",
		Model: "gpt-4o",
		Message: core.Message{
			Role:    core.RoleAssistant,
			Content: []core.ContentPart{{Type: core.PartText, Text: "the answer"}},
		},
		FinishReason: core.FinishStop,
		Usage:        core.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	body, err := OpenAICodec{}.RenderResponse(resp)
	require.NoError(t, err)

	back, err := OpenAICodec{}.ParseResponse(body, "gpt-4o")
	require.NoError(t, err)
	require.Equal(t, "the answer", back.Message.TextContent())
	require.Equal(t, core.FinishStop, back.FinishReason)
	require.Equal(t, 15, back.Usage.TotalTokens)
}

func TestOpenAI_ParseStreamLine(t *testing.T) {
	line := []byte(`{"id":"x","model":"gpt-4o","choices":[{"delta":{"content":"hel"},"finish_reason":null}]}`)
	chunks, err := OpenAICodec{}.ParseStreamLine(line, "gpt-4o")
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Equal(t, core.ChunkText, chunks[0].Type)
	require.Equal(t, "hel", chunks[0].Delta)

	done, err := OpenAICodec{}.ParseStreamLine([]byte("[DONE]"), "gpt-4o")
	require.NoError(t, err)
	require.Empty(t, done)
}

func TestAnthropic_ParseStreamEvents(t *testing.T) {
	textDelta := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`)
	chunks, err := AnthropicCodec{}.ParseStreamLine(textDelta, "claude")
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Equal(t, core.ChunkText, chunks[0].Type)
	require.Equal(t, "hi", chunks[0].Delta)

	stop := []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":42}}`)
	chunks, err = AnthropicCodec{}.ParseStreamLine(stop, "claude")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(chunks), 1)
	require.Equal(t, core.ChunkFinish, chunks[0].Type)
	require.Equal(t, core.FinishStop, chunks[0].FinishReason)
}

// Stream re-render: canonical text chunks rendered to OpenAI SSE must start with
// the assistant role then carry content deltas.
func TestOpenAI_RenderStreamChunk_SequencesRole(t *testing.T) {
	state := &StreamState{Model: "gpt-4o", MessageID: "id1"}
	first, err := OpenAICodec{}.RenderStreamChunk(core.StreamChunk{Type: core.ChunkText, Delta: "a"}, state)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Contains(t, string(first[0]), `"role":"assistant"`)
	require.Contains(t, string(first[0]), `"content":"a"`)

	second, err := OpenAICodec{}.RenderStreamChunk(core.StreamChunk{Type: core.ChunkText, Delta: "b"}, state)
	require.NoError(t, err)
	require.NotContains(t, string(second[0]), `"role"`, "role only sent once")

	done := OpenAICodec{}.RenderStreamDone(state)
	require.Contains(t, string(done[0]), "[DONE]")
}

func TestRegistry_ResolvesCodecs(t *testing.T) {
	reg := DefaultRegistry()

	_, err := reg.Codec(core.DialectOpenAI)
	require.NoError(t, err)

	_, err = reg.StreamCodec(core.DialectAnthropic)
	require.NoError(t, err)

	_, err = reg.Codec(core.DialectGemini)
	require.Error(t, err, "unregistered dialect must error")
}