package pipeline

import (
	"testing"
	"time"

	"github.com/mydisha/keirouter/backend/internal/core"
)

func TestShouldRetryStreamRateLimit(t *testing.T) {
	tests := []struct {
		name    string
		error   *core.ProviderError
		retries int
		want    bool
	}{
		{name: "transient other provider", error: &core.ProviderError{Kind: core.ErrRateLimit, Provider: "openai"}, want: true},
		{name: "kiro account limit", error: &core.ProviderError{Kind: core.ErrRateLimit, Provider: "kiro"}, want: false},
		{name: "explicit reset", error: &core.ProviderError{Kind: core.ErrRateLimit, Provider: "openai", RetryAfter: time.Minute}, want: false},
		{name: "retry budget exhausted", error: &core.ProviderError{Kind: core.ErrRateLimit, Provider: "openai"}, retries: maxRateLimitRetries, want: false},
		{name: "not a rate limit", error: &core.ProviderError{Kind: core.ErrUpstream, Provider: "openai"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetryStreamRateLimit(tt.error, tt.retries); got != tt.want {
				t.Fatalf("shouldRetryStreamRateLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractUsageFromStream_OpenAI(t *testing.T) {
	// OpenAI format: usage in the last chunk before [DONE].
	raw := []byte(`data: {"id":"1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hi"}}]}

data: {"id":"2","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}

data: [DONE]
`)
	usage := extractUsageFromStream(raw)
	if usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", usage.PromptTokens)
	}
	if usage.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d, want 20", usage.CompletionTokens)
	}
}

func TestExtractUsageFromStream_Anthropic(t *testing.T) {
	// Anthropic format: input_tokens in message_start, output_tokens in message_delta.
	raw := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":50,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}

event: message_stop
data: {"type":"message_stop"}
`)
	usage := extractUsageFromStream(raw)
	if usage.PromptTokens != 50 {
		t.Errorf("PromptTokens = %d, want 50", usage.PromptTokens)
	}
	if usage.CompletionTokens != 15 {
		t.Errorf("CompletionTokens = %d, want 15", usage.CompletionTokens)
	}
}

func TestExtractUsageFromStream_AnthropicIncludesCachedInput(t *testing.T) {
	raw := []byte(`event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":100,"output_tokens":0,"cache_creation_input_tokens":25,"cache_read_input_tokens":900}}}

event: message_delta
data: {"type":"message_delta","usage":{"output_tokens":15}}
`)

	usage := extractUsageFromStream(raw)
	if usage.PromptTokens != 1025 {
		t.Errorf("PromptTokens = %d, want 1025", usage.PromptTokens)
	}
	if usage.CachedTokens != 900 {
		t.Errorf("CachedTokens = %d, want 900", usage.CachedTokens)
	}
	if usage.CacheWriteTokens != 25 {
		t.Errorf("CacheWriteTokens = %d, want 25", usage.CacheWriteTokens)
	}
	if usage.TotalTokens != 1040 {
		t.Errorf("TotalTokens = %d, want 1040", usage.TotalTokens)
	}
}

func TestExtractUsageFromStream_Empty(t *testing.T) {
	raw := []byte(`data: {"choices":[{"delta":{"content":"no usage here"}}]}

data: [DONE]
`)
	usage := extractUsageFromStream(raw)
	if usage.PromptTokens != 0 || usage.CompletionTokens != 0 {
		t.Errorf("expected zero usage, got prompt=%d completion=%d", usage.PromptTokens, usage.CompletionTokens)
	}
}

func TestExtractUsageFromSSEData_TopLevelUsage(t *testing.T) {
	data := []byte(`{"usage":{"prompt_tokens":100,"completion_tokens":200}}`)
	u := extractUsageFromSSEData(data)
	if u == nil {
		t.Fatal("expected non-nil usage")
	}
	if u.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", u.PromptTokens)
	}
	if u.CompletionTokens != 200 {
		t.Errorf("CompletionTokens = %d, want 200", u.CompletionTokens)
	}
}

func TestExtractUsageFromSSEData_AnthropicMessageStart(t *testing.T) {
	data := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":50,"output_tokens":0}}}`)
	u := extractUsageFromSSEData(data)
	if u == nil {
		t.Fatal("expected non-nil usage")
	}
	if u.PromptTokens != 50 {
		t.Errorf("PromptTokens = %d, want 50", u.PromptTokens)
	}
}

func TestExtractUsageFromSSEData_NoUsage(t *testing.T) {
	data := []byte(`{"choices":[{"delta":{"content":"hello"}}]}`)
	u := extractUsageFromSSEData(data)
	if u != nil {
		t.Errorf("expected nil, got %+v", u)
	}
}

func TestMergeUsage(t *testing.T) {
	old := core.Usage{PromptTokens: 10}
	new := core.Usage{CompletionTokens: 20}
	merged := mergeUsage(old, new)
	if merged.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", merged.PromptTokens)
	}
	if merged.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d, want 20", merged.CompletionTokens)
	}
}

func TestSafeBuffer_SmallStream(t *testing.T) {
	var buf safeBuffer
	data := []byte("hello world")
	buf.Write(data)
	got := buf.Bytes()
	if string(got) != "hello world" {
		t.Errorf("Bytes() = %q, want %q", got, "hello world")
	}
}
