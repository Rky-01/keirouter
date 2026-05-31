package transform

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// oaiStreamChunk is one SSE "data:" payload from an OpenAI streaming response.
type oaiStreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *oaiUsage `json:"usage"`
}

// ParseStreamLine decodes one upstream SSE data payload into canonical chunks.
// The caller strips the "data: " prefix before calling; the special "[DONE]"
// sentinel is handled here and yields no chunks.
func (OpenAICodec) ParseStreamLine(line []byte, model string) ([]core.StreamChunk, error) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || bytes.Equal(line, []byte("[DONE]")) {
		return nil, nil
	}

	var raw oaiStreamChunk
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, fmt.Errorf("openai: parse stream chunk: %w", err)
	}

	var chunks []core.StreamChunk
	if len(raw.Choices) > 0 {
		c := raw.Choices[0]
		if c.Delta.Content != "" {
			chunks = append(chunks, core.StreamChunk{Type: core.ChunkText, Delta: c.Delta.Content})
		}
		for _, tc := range c.Delta.ToolCalls {
			chunks = append(chunks, core.StreamChunk{
				Type:  core.ChunkToolCall,
				Index: tc.Index,
				ToolCall: &core.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: json.RawMessage(tc.Function.Arguments),
				},
			})
		}
		if c.FinishReason != nil {
			chunks = append(chunks, core.StreamChunk{
				Type:         core.ChunkFinish,
				FinishReason: mapOAIFinish(*c.FinishReason),
			})
		}
	}

	if raw.Usage != nil {
		chunks = append(chunks, core.StreamChunk{
			Type: core.ChunkUsage,
			Usage: &core.Usage{
				PromptTokens:     raw.Usage.PromptTokens,
				CompletionTokens: raw.Usage.CompletionTokens,
				TotalTokens:      raw.Usage.TotalTokens,
			},
		})
	}
	return chunks, nil
}

// RenderStreamChunk encodes a canonical chunk as an OpenAI SSE event. The first
// emitted event carries the assistant role per the OpenAI contract.
func (OpenAICodec) RenderStreamChunk(chunk core.StreamChunk, state *StreamState) ([][]byte, error) {
	delta := map[string]any{}
	switch chunk.Type {
	case core.ChunkText:
		if !state.SentRole {
			delta["role"] = "assistant"
			state.SentRole = true
		}
		delta["content"] = chunk.Delta
	case core.ChunkToolCall:
		if !state.SentRole {
			delta["role"] = "assistant"
			state.SentRole = true
		}
		tc := map[string]any{
			"index": chunk.Index,
			"type":  "function",
			"function": map[string]string{
				"name":      chunk.ToolCall.Name,
				"arguments": string(chunk.ToolCall.Arguments),
			},
		}
		if chunk.ToolCall.ID != "" {
			tc["id"] = chunk.ToolCall.ID
		}
		delta["tool_calls"] = []any{tc}
	case core.ChunkFinish:
		return [][]byte{encodeOAIEvent(state, map[string]any{}, ptr(string(chunk.FinishReason)), nil)}, nil
	case core.ChunkUsage:
		// Emit a usage-only chunk (OpenAI sends this as a final empty-choices event).
		return [][]byte{encodeOAIUsageEvent(state, chunk.Usage)}, nil
	default:
		return nil, nil
	}
	return [][]byte{encodeOAIEvent(state, delta, nil, nil)}, nil
}

// RenderStreamDone returns the OpenAI terminal sentinel.
func (OpenAICodec) RenderStreamDone(_ *StreamState) [][]byte {
	return [][]byte{[]byte("data: [DONE]\n\n")}
}

func encodeOAIEvent(state *StreamState, delta map[string]any, finish *string, _ any) []byte {
	choice := map[string]any{"index": 0, "delta": delta}
	if finish != nil {
		choice["finish_reason"] = *finish
	} else {
		choice["finish_reason"] = nil
	}
	payload := map[string]any{
		"id":      firstNonEmpty(state.MessageID, "chatcmpl-stream"),
		"object":  "chat.completion.chunk",
		"model":   state.Model,
		"choices": []any{choice},
	}
	b, _ := json.Marshal(payload)
	return append([]byte("data: "), append(b, '\n', '\n')...)
}

func encodeOAIUsageEvent(state *StreamState, usage *core.Usage) []byte {
	payload := map[string]any{
		"id":      firstNonEmpty(state.MessageID, "chatcmpl-stream"),
		"object":  "chat.completion.chunk",
		"model":   state.Model,
		"choices": []any{},
		"usage": map[string]int{
			"prompt_tokens":     usage.PromptTokens,
			"completion_tokens": usage.CompletionTokens,
			"total_tokens":      usage.TotalTokens,
		},
	}
	b, _ := json.Marshal(payload)
	return append([]byte("data: "), append(b, '\n', '\n')...)
}

func ptr[T any](v T) *T { return &v }