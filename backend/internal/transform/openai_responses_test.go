package transform

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/stretchr/testify/require"
)

func TestResponses_ParseRequest_InputItems(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5-codex",
		"stream": true,
		"instructions": "be precise",
		"input": [
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "hello"}]},
			{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": "{\"city\":\"SF\"}"},
			{"type": "function_call_output", "call_id": "call_1", "output": "sunny"}
		],
		"tools": [
			{"type": "function", "name": "get_weather", "description": "weather", "parameters": {"type": "object", "properties": {}}}
		]
	}`)

	req, err := OpenAIResponsesCodec{}.ParseRequest(body)
	require.NoError(t, err)
	require.Equal(t, "gpt-5-codex", req.Model)
	require.True(t, req.Stream)
	require.Equal(t, "be precise", req.System)
	require.Len(t, req.Tools, 1)
	require.Equal(t, "get_weather", req.Tools[0].Name)

	// user message + assistant(tool_call) + tool(result)
	require.Len(t, req.Messages, 3)
	require.Equal(t, "hello", req.Messages[0].TextContent())
	require.Equal(t, core.RoleAssistant, req.Messages[1].Role)
	require.Equal(t, core.PartToolCall, req.Messages[1].Content[0].Type)
	require.Equal(t, "get_weather", req.Messages[1].Content[0].ToolCall.Name)
	require.Equal(t, core.RoleTool, req.Messages[2].Role)
	require.Equal(t, "sunny", req.Messages[2].Content[0].ToolResult.Content)
}

func TestResponses_ParseRequest_StringInput(t *testing.T) {
	body := []byte(`{"model":"gpt-5","input":"just a prompt"}`)
	req, err := OpenAIResponsesCodec{}.ParseRequest(body)
	require.NoError(t, err)
	require.Len(t, req.Messages, 1)
	require.Equal(t, "just a prompt", req.Messages[0].TextContent())
}

func TestResponses_RenderRequest_Shape(t *testing.T) {
	req := &core.ChatRequest{
		Model:  "gpt-5-codex",
		System: "sys",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "hi"}}},
			{Role: core.RoleAssistant, Content: []core.ContentPart{
				{Type: core.PartToolCall, ToolCall: &core.ToolCall{ID: "c1", Name: "f", Arguments: json.RawMessage(`{}`)}},
			}},
			{Role: core.RoleTool, Content: []core.ContentPart{
				{Type: core.PartToolResult, ToolResult: &core.ToolResult{CallID: "c1", Content: "out"}},
			}},
		},
	}
	body, err := OpenAIResponsesCodec{}.RenderRequest(req)
	require.NoError(t, err)

	var parsed struct {
		Instructions string `json:"instructions"`
		Input        []struct {
			Type   string `json:"type"`
			Role   string `json:"role"`
			CallID string `json:"call_id"`
			Output string `json:"output"`
		} `json:"input"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.Equal(t, "sys", parsed.Instructions)
	require.Len(t, parsed.Input, 3)
	require.Equal(t, "message", parsed.Input[0].Type)
	require.Equal(t, "function_call", parsed.Input[1].Type)
	require.Equal(t, "function_call_output", parsed.Input[2].Type)
	require.Equal(t, "out", parsed.Input[2].Output)
}

func TestResponses_ParseResponse_Unary(t *testing.T) {
	body := []byte(`{
		"id": "resp_123",
		"output": [
			{"type": "message", "role": "assistant", "content": [{"type": "output_text", "text": "the answer"}]}
		],
		"usage": {"input_tokens": 10, "output_tokens": 5, "input_tokens_details": {"cached_tokens": 3}}
	}`)
	resp, err := OpenAIResponsesCodec{}.ParseResponse(body, "gpt-5-codex")
	require.NoError(t, err)
	require.Equal(t, "the answer", resp.Message.TextContent())
	require.Equal(t, 15, resp.Usage.TotalTokens)
	require.Equal(t, 3, resp.Usage.CachedTokens)
}

func TestResponses_ParseStreamLine_Events(t *testing.T) {
	codec := OpenAIResponsesCodec{}

	text := []byte(`{"type":"response.output_text.delta","delta":"hel"}`)
	chunks, err := codec.ParseStreamLine(text, "gpt-5")
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Equal(t, core.ChunkText, chunks[0].Type)
	require.Equal(t, "hel", chunks[0].Delta)

	fcAdded := []byte(`{"type":"response.output_item.added","item":{"type":"function_call","call_id":"c1","name":"f"}}`)
	chunks, err = codec.ParseStreamLine(fcAdded, "gpt-5")
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Equal(t, core.ChunkToolCall, chunks[0].Type)
	require.Equal(t, "f", chunks[0].ToolCall.Name)

	completed := []byte(`{"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":5}}}`)
	chunks, err = codec.ParseStreamLine(completed, "gpt-5")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(chunks), 2)
	require.Equal(t, core.ChunkFinish, chunks[0].Type)
	require.Equal(t, core.ChunkUsage, chunks[1].Type)
	require.Equal(t, 15, chunks[1].Usage.TotalTokens)
}

func TestResponses_RenderStream_EventSequence(t *testing.T) {
	codec := OpenAIResponsesCodec{}
	state := &StreamState{Model: "gpt-5", MessageID: "abc"}

	first, err := codec.RenderStreamChunk(core.StreamChunk{Type: core.ChunkText, Delta: "hi"}, state)
	require.NoError(t, err)
	joined := strings.Join(toStrings(first), "")
	// First text chunk must open the response and the message item.
	require.Contains(t, joined, "response.created")
	require.Contains(t, joined, "response.output_item.added")
	require.Contains(t, joined, "response.output_text.delta")

	usage, err := codec.RenderStreamChunk(core.StreamChunk{
		Type: core.ChunkUsage, Usage: &core.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	}, state)
	require.NoError(t, err)
	require.Contains(t, strings.Join(toStrings(usage), ""), "response.completed")
}

func TestCrossDialect_OpenAIToResponsesRoundTrip(t *testing.T) {
	body := []byte(`{
		"model": "gpt-5-codex",
		"messages": [
			{"role": "system", "content": "be terse"},
			{"role": "user", "content": "ping"}
		]
	}`)
	canonical, err := OpenAICodec{}.ParseRequest(body)
	require.NoError(t, err)

	respBody, err := OpenAIResponsesCodec{}.RenderRequest(canonical)
	require.NoError(t, err)

	back, err := OpenAIResponsesCodec{}.ParseRequest(respBody)
	require.NoError(t, err)
	require.Equal(t, "be terse", back.System)
	require.Len(t, back.Messages, 1)
	require.Equal(t, "ping", back.Messages[0].TextContent())
}

func toStrings(b [][]byte) []string {
	out := make([]string, len(b))
	for i, x := range b {
		out[i] = string(x)
	}
	return out
}