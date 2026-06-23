package transform

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/stretchr/testify/require"
)

// TestOpenAI_RenderStreamChunk_Thinking verifies structured reasoning is
// emitted to the client as reasoning_content (issue #17: DeepSeek thinking
// mode requires it on follow-up turns).
func TestOpenAI_RenderStreamChunk_Thinking(t *testing.T) {
	state := &StreamState{Model: "deepseek-reasoner", MessageID: "id1"}
	events, err := OpenAICodec{}.RenderStreamChunk(
		core.StreamChunk{Type: core.ChunkThinking, Delta: "let me think"}, state)
	require.NoError(t, err)
	require.Len(t, events, 1)

	payload := strings.TrimPrefix(string(events[0]), "data: ")
	var got struct {
		Choices []struct {
			Delta struct {
				Role             string `json:"role"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(payload)), &got))
	require.Len(t, got.Choices, 1)
	require.Equal(t, "assistant", got.Choices[0].Delta.Role)
	require.Equal(t, "let me think", got.Choices[0].Delta.ReasoningContent)
}

// TestOpenAI_StreamReasoning_RoundTrip verifies a reasoning_content delta from
// upstream is parsed and re-rendered back to the client without loss.
func TestOpenAI_StreamReasoning_RoundTrip(t *testing.T) {
	line := []byte(`{"id":"c1","model":"deepseek-reasoner","choices":[{"delta":{"reasoning_content":"step one"}}]}`)
	chunks, err := OpenAICodec{}.ParseStreamLine(line, "deepseek-reasoner")
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Equal(t, core.ChunkThinking, chunks[0].Type)
	require.Equal(t, "step one", chunks[0].Delta)

	state := &StreamState{Model: "deepseek-reasoner"}
	events, err := OpenAICodec{}.RenderStreamChunk(chunks[0], state)
	require.NoError(t, err)
	require.Contains(t, string(events[0]), "reasoning_content")
	require.Contains(t, string(events[0]), "step one")
}

// TestOpenAI_RenderResponse_Reasoning verifies non-streaming responses surface
// reasoning_content for clients that replay it.
func TestOpenAI_RenderResponse_Reasoning(t *testing.T) {
	resp := &core.ChatResponse{
		Model: "deepseek-reasoner",
		Message: core.Message{
			Role: core.RoleAssistant,
			Content: []core.ContentPart{
				{Type: core.PartThinking, Text: "internal reasoning"},
				{Type: core.PartText, Text: "the answer"},
			},
		},
		FinishReason: core.FinishStop,
	}
	body, err := OpenAICodec{}.RenderResponse(resp)
	require.NoError(t, err)

	var got struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Len(t, got.Choices, 1)
	require.Equal(t, "the answer", got.Choices[0].Message.Content)
	require.Equal(t, "internal reasoning", got.Choices[0].Message.ReasoningContent)
}

// TestOpenAI_RenderRequest_InjectReasoningPlaceholder verifies the safety net:
// for DeepSeek targets, assistant messages without reasoning get a placeholder
// reasoning_content so the upstream doesn't reject the turn with a 400.
func TestOpenAI_RenderRequest_InjectReasoningPlaceholder(t *testing.T) {
	req := &core.ChatRequest{
		Model: "deepseek-chat",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "hi"}}},
			{Role: core.RoleAssistant, Content: []core.ContentPart{{Type: core.PartText, Text: "hello"}}},
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "continue"}}},
		},
	}
	body, err := OpenAICodec{}.RenderRequestForProvider(req, "deepseek")
	require.NoError(t, err)

	var got oaiRequest
	require.NoError(t, json.Unmarshal(body, &got))
	// Assistant message (index 1) should carry the placeholder.
	require.Equal(t, "assistant", got.Messages[1].Role)
	require.Equal(t, reasoningPlaceholder, got.Messages[1].ReasoningContent)
}

// TestOpenAI_RenderRequest_PreservesRealReasoning verifies genuine reasoning is
// kept intact (not overwritten by the placeholder).
func TestOpenAI_RenderRequest_PreservesRealReasoning(t *testing.T) {
	req := &core.ChatRequest{
		Model: "deepseek-chat",
		Messages: []core.Message{
			{Role: core.RoleAssistant, Content: []core.ContentPart{
				{Type: core.PartThinking, Text: "real chain of thought"},
				{Type: core.PartText, Text: "hello"},
			}},
		},
	}
	body, err := OpenAICodec{}.RenderRequestForProvider(req, "deepseek")
	require.NoError(t, err)

	var got oaiRequest
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "real chain of thought", got.Messages[0].ReasoningContent)
}

// TestOpenAI_RenderRequest_NoInjectForNonDeepSeek verifies non-DeepSeek targets
// are untouched (avoid sending reasoning_content to providers that reject it).
func TestOpenAI_RenderRequest_NoInjectForNonDeepSeek(t *testing.T) {
	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleAssistant, Content: []core.ContentPart{{Type: core.PartText, Text: "hello"}}},
		},
	}
	body, err := OpenAICodec{}.RenderRequestForProvider(req, "openai")
	require.NoError(t, err)

	var got oaiRequest
	require.NoError(t, json.Unmarshal(body, &got))
	require.Empty(t, got.Messages[0].ReasoningContent)
}

// TestRequiresReasoningEcho covers provider- and model-based detection.
func TestRequiresReasoningEcho(t *testing.T) {
	require.True(t, requiresReasoningEcho("deepseek", "deepseek-chat"))
	require.True(t, requiresReasoningEcho("openrouter", "deepseek/deepseek-chat"))
	require.True(t, requiresReasoningEcho("siliconflow", "deepseek-ai/DeepSeek-V3.2"))
	require.False(t, requiresReasoningEcho("openai", "gpt-4o"))
	require.False(t, requiresReasoningEcho("groq", "llama-3.3-70b"))
}

func TestOpenAI_RenderRequest_DeepSeekToolCallFixes(t *testing.T) {
	req := &core.ChatRequest{
		Model: "deepseek-v4-flash",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "hi"}}},
			{Role: core.RoleAssistant, Content: []core.ContentPart{
				{Type: core.PartToolCall, ToolCall: &core.ToolCall{Name: "get_weather", Arguments: json.RawMessage(`{"city":"Jakarta"}`)}},
			}},
		},
	}
	body, err := OpenAICodec{}.RenderRequestForProvider(req, "deepseek")
	require.NoError(t, err)

	var got oaiRequest
	require.NoError(t, json.Unmarshal(body, &got))
	require.Len(t, got.Messages, 3)
	assistant := got.Messages[1]
	require.Equal(t, reasoningPlaceholder, assistant.ReasoningContent)
	require.Len(t, assistant.ToolCalls, 1)
	require.Equal(t, "call_msg1_tc0_get_weather", assistant.ToolCalls[0].ID)
	require.Equal(t, "function", assistant.ToolCalls[0].Type)

	var args string
	require.NoError(t, json.Unmarshal(assistant.ToolCalls[0].Function.Arguments, &args))
	require.JSONEq(t, `{"city":"Jakarta"}`, args)
	require.Equal(t, "tool", got.Messages[2].Role)
	require.Equal(t, assistant.ToolCalls[0].ID, got.Messages[2].ToolCallID)
}

func TestOpenAI_RenderRequest_NonDeepSeekToolCallsUntouched(t *testing.T) {
	req := &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: core.RoleAssistant, Content: []core.ContentPart{
				{Type: core.PartToolCall, ToolCall: &core.ToolCall{Name: "get_weather", Arguments: json.RawMessage(`{"city":"Jakarta"}`)}},
			}},
		},
	}
	body, err := OpenAICodec{}.RenderRequestForProvider(req, "openai")
	require.NoError(t, err)

	var got oaiRequest
	require.NoError(t, json.Unmarshal(body, &got))
	require.Len(t, got.Messages, 1)
	require.Empty(t, got.Messages[0].ReasoningContent)
	require.Empty(t, got.Messages[0].ToolCalls[0].ID)
}

func TestOpenAI_RenderRequest_DeepSeekV4ProAliases(t *testing.T) {
	cases := []struct {
		model        string
		wantEffort   string
		wantThinking string
	}{
		{model: "deepseek-v4-pro-max", wantEffort: "max", wantThinking: "enabled"},
		{model: "deepseek-v4-pro-none", wantEffort: "", wantThinking: "disabled"},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			req := &core.ChatRequest{Model: tc.model, Messages: []core.Message{{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "hi"}}}}}
			body, err := OpenAICodec{}.RenderRequestForProvider(req, "deepseek")
			require.NoError(t, err)

			var got oaiRequest
			require.NoError(t, json.Unmarshal(body, &got))
			require.Equal(t, "deepseek-v4-pro", got.Model)
			require.Equal(t, tc.wantEffort, got.ReasoningEffort)
			require.NotNil(t, got.ExtraBody["thinking"])
			thinking := got.ExtraBody["thinking"].(map[string]any)
			require.Equal(t, tc.wantThinking, thinking["type"])
		})
	}
}

func TestOpenAI_RenderRequest_DeepSeekReasoningEffortMapping(t *testing.T) {
	cases := []struct {
		effort string
		want   string
	}{
		{effort: "low", want: "high"},
		{effort: "medium", want: "high"},
		{effort: "high", want: "high"},
		{effort: "xhigh", want: "max"},
		{effort: "max", want: "max"},
	}
	for _, tc := range cases {
		t.Run(tc.effort, func(t *testing.T) {
			req := &core.ChatRequest{Model: "deepseek-v4-flash", Reasoning: &core.ReasoningConfig{Effort: tc.effort}}
			body, err := OpenAICodec{}.RenderRequestForProvider(req, "custom-openai")
			require.NoError(t, err)

			var got oaiRequest
			require.NoError(t, json.Unmarshal(body, &got))
			require.Equal(t, tc.want, got.ReasoningEffort)
			require.NotNil(t, got.Thinking)
			require.Equal(t, "enabled", got.Thinking.Type)
		})
	}
}
