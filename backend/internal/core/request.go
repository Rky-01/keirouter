package core

import "encoding/json"

// Dialect identifies an API wire format spoken by a client or a provider.
type Dialect string

const (
	DialectOpenAI          Dialect = "openai"           // /v1/chat/completions
	DialectOpenAIResponses Dialect = "openai_responses" // /v1/responses
	DialectAnthropic       Dialect = "anthropic"        // /v1/messages
	DialectGemini          Dialect = "gemini"           // :generateContent
	DialectOllama          Dialect = "ollama"           // /api/chat
)

// ChatRequest is the canonical, dialect-independent representation of a chat
// completion request. The transform layer parses inbound bodies into this and
// renders it back out per the target connector's dialect.
type ChatRequest struct {
	// Model is the requested model id, already stripped of any router alias
	// prefix (e.g. "kr/claude-sonnet-4.5" -> "claude-sonnet-4.5").
	Model string `json:"model"`

	Messages []Message `json:"messages"`

	// System holds top-level system instructions hoisted out of Messages for
	// dialects that carry them separately (Anthropic, Gemini).
	System string `json:"system,omitempty"`

	Tools      []Tool   `json:"tools,omitempty"`
	ToolChoice any      `json:"tool_choice,omitempty"`
	Stop       []string `json:"stop,omitempty"`

	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`

	// Stream requests an incremental SSE response.
	Stream bool `json:"stream"`

	// Reasoning controls extended thinking / reasoning effort where supported.
	Reasoning *ReasoningConfig `json:"reasoning,omitempty"`

	// ResponseFormat carries structured-output constraints (json_schema, etc).
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`

	// Metadata is router-internal context (not serialized upstream) such as the
	// originating client and the resolved capability requirements.
	Metadata RequestMetadata `json:"-"`

	// Extra preserves dialect-specific fields the canonical model does not model
	// explicitly, so they can be passed through on same-dialect routing.
	Extra map[string]json.RawMessage `json:"-"`
}

// ReasoningConfig expresses extended-thinking intent in a provider-neutral way.
type ReasoningConfig struct {
	// Effort is one of "low", "medium", "high" (maps to provider knobs).
	Effort string `json:"effort,omitempty"`
	// MaxTokens caps the thinking budget where the provider supports it.
	MaxTokens int `json:"max_tokens,omitempty"`
}

// Tool is a function/tool definition advertised to the model.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	// Parameters is a JSON Schema object describing the tool arguments.
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

// RequestMetadata is router-internal context attached during the pipeline.
type RequestMetadata struct {
	// ClientKind is the detected calling tool (claude-code, cursor, codex, ...).
	ClientKind string
	// SourceDialect is the wire format the client used.
	SourceDialect Dialect
	// APIKeyID is the id of the authenticated KeiRouter key (for metering/audit).
	APIKeyID string
	// TenantID / ProjectID scope the request for multi-tenant deployments.
	TenantID  string
	ProjectID string
	// RequiredCapabilities the chosen model must satisfy (anti-downgrade guard).
	RequiredCapabilities CapabilitySet
}