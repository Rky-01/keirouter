package store

import "time"

// DefaultTenantID is the implicit tenant used in local single-user mode.
const DefaultTenantID = "default"

// Tenant scopes all data in multi-tenant deployments.
type Tenant struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Project partitions usage and budgets within a tenant.
type Project struct {
	ID        string
	TenantID  string
	Name      string
	CreatedAt time.Time
}

// APIKey is a stored inbound credential. The plaintext is never persisted.
type APIKey struct {
	ID         string
	TenantID   string
	ProjectID  string
	Name       string
	KeyHash    string // argon2id verifier
	LookupHash string // sha-256 index
	Display    string // masked form
	Scopes     string
	Disabled   bool
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

// AuthKind classifies how an account authenticates upstream.
type AuthKind string

const (
	AuthAPIKey AuthKind = "api_key"
	AuthOAuth  AuthKind = "oauth"
	AuthNone   AuthKind = "none"
)

// Account holds an upstream provider credential. Secret material is stored as
// envelope-encrypted blobs (the *WrappedDEK / *Ciphertext pairs).
type Account struct {
	ID       string
	TenantID string
	Provider string
	Label    string
	AuthKind AuthKind

	SecretWrappedDEK string
	SecretCiphertext string

	TokenWrappedDEK string
	TokenCiphertext string

	RefreshWrappedDEK string
	RefreshCiphertext string

	TokenExpiresAt *time.Time

	Metadata      string // JSON: base_url, region, project_id, ...
	Priority      int
	Disabled      bool
	CooldownUntil *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Chain is an ordered fallback definition (routing chain).
type Chain struct {
	ID        string
	TenantID  string
	Name      string
	Strategy  string
	Steps     []ChainStep
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ChainStep is one candidate target within a chain.
type ChainStep struct {
	ID        string
	ChainID   string
	Position  int
	Provider  string
	Model     string
	CreatedAt time.Time
}

// UsageRecord meters one completed request.
type UsageRecord struct {
	ID               string
	TenantID         string
	ProjectID        string
	APIKeyID         string
	Provider         string
	Model            string
	AccountID        string
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	CostMicros       int64
	CacheHit         bool
	LatencyMS        int
	CreatedAt        time.Time
}

// BudgetScope identifies what a budget applies to.
type BudgetScope string

const (
	ScopeTenant  BudgetScope = "tenant"
	ScopeProject BudgetScope = "project"
	ScopeAPIKey  BudgetScope = "api_key"
)

// Budget enforces a spend limit over a period.
type Budget struct {
	ID          string
	TenantID    string
	ScopeKind   BudgetScope
	ScopeID     string
	LimitMicros int64
	Period      string
	AlertPct    int
	HardCutoff  bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AuditEntry is one append-only audit record.
type AuditEntry struct {
	ID        string
	TenantID  string
	Actor     string
	Action    string
	Target    string
	Detail    string
	CreatedAt time.Time
}