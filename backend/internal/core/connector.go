package core

import "context"

// Connector is the contract every provider driver implements. The pipeline
// selects a connector + credentials, hands it a canonical ChatRequest, and the
// connector is responsible for dialect rendering, HTTP transport, and parsing
// the upstream response back into canonical form.
//
// Implementations must be safe for concurrent use: a single Connector value
// serves many in-flight requests.
type Connector interface {
	// ID returns the stable provider identifier (e.g. "openai", "anthropic").
	ID() string

	// Dialect reports the wire format this connector speaks upstream. The
	// transform layer uses it to decide whether translation is needed.
	Dialect() Dialect

	// Chat performs a non-streaming completion.
	Chat(ctx context.Context, req *ChatRequest, creds Credentials) (*ChatResponse, error)

	// Stream performs a streaming completion, emitting canonical chunks on the
	// returned channel until it is closed. A terminal error is delivered as a
	// ChunkError followed by channel close, so callers select on ctx.Done and
	// range over the channel.
	Stream(ctx context.Context, req *ChatRequest, creds Credentials) (<-chan StreamChunk, error)
}

// Credentials carries the resolved secret material a connector needs for a
// single upstream call. The vault decrypts and supplies these; connectors must
// never persist them.
type Credentials struct {
	// AccountID identifies which provider account these belong to (metering).
	AccountID string
	// APIKey is set for key-based providers.
	APIKey string
	// AccessToken is set for OAuth/bearer providers.
	AccessToken string
	// BaseURL overrides the connector's default endpoint when non-empty.
	BaseURL string
	// Headers are extra headers to merge into the upstream request.
	Headers map[string]string
	// Extra holds provider-specific fields (project id, region, deployment...).
	Extra map[string]string
}

// MediaConnector is implemented by providers that support non-chat modalities.
// It is optional; chat-only connectors omit it.
type MediaConnector interface {
	// Embeddings produces vector embeddings for the given inputs.
	Embeddings(ctx context.Context, req *EmbeddingRequest, creds Credentials) (*EmbeddingResponse, error)
}

// EmbeddingRequest is a canonical embeddings request.
type EmbeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

// EmbeddingResponse holds the resulting vectors and token usage.
type EmbeddingResponse struct {
	Model   string      `json:"model"`
	Vectors [][]float32 `json:"vectors"`
	Usage   Usage       `json:"usage"`
}