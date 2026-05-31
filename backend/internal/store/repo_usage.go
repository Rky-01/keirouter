package store

import (
	"context"
	"fmt"
	"time"
)

// UsageRepo records and aggregates per-request metering data.
type UsageRepo struct{ db *DB }

// Usage returns the usage repository.
func (db *DB) Usage() *UsageRepo { return &UsageRepo{db: db} }

// Record inserts a usage row for a completed request.
func (r *UsageRepo) Record(ctx context.Context, u UsageRecord) error {
	q := r.db.rebind(`
		INSERT INTO usage_records
			(id, tenant_id, project_id, api_key_id, provider, model, account_id,
			 prompt_tokens, completion_tokens, cached_tokens, cost_micros,
			 cache_hit, latency_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := r.db.sql.ExecContext(ctx, q,
		u.ID, u.TenantID, nullString(u.ProjectID), nullString(u.APIKeyID),
		u.Provider, u.Model, nullString(u.AccountID),
		u.PromptTokens, u.CompletionTokens, u.CachedTokens, u.CostMicros,
		boolToInt(u.CacheHit), u.LatencyMS, formatTime(u.CreatedAt))
	if err != nil {
		return fmt.Errorf("store: record usage: %w", err)
	}
	return nil
}

// SpendSince returns total cost in micros for a budget scope since the given
// time. Used by the budget engine to enforce hard limits.
func (r *UsageRepo) SpendSince(ctx context.Context, scope BudgetScope, scopeID string, since time.Time) (int64, error) {
	var column string
	switch scope {
	case ScopeTenant:
		column = "tenant_id"
	case ScopeProject:
		column = "project_id"
	case ScopeAPIKey:
		column = "api_key_id"
	default:
		return 0, fmt.Errorf("store: unknown budget scope %q", scope)
	}

	q := r.db.rebind(fmt.Sprintf(
		`SELECT COALESCE(SUM(cost_micros), 0) FROM usage_records WHERE %s = ? AND created_at >= ?`,
		column))
	var total int64
	if err := r.db.sql.QueryRowContext(ctx, q, scopeID, formatTime(since)).Scan(&total); err != nil {
		return 0, fmt.Errorf("store: spend since: %w", err)
	}
	return total, nil
}

// Summary aggregates usage for a tenant over a time window.
type Summary struct {
	TotalRequests    int64
	PromptTokens     int64
	CompletionTokens int64
	CachedTokens     int64
	CostMicros       int64
	CacheHits        int64
}

// Summarize returns aggregate usage for a tenant since the given time.
func (r *UsageRepo) Summarize(ctx context.Context, tenantID string, since time.Time) (Summary, error) {
	q := r.db.rebind(`
		SELECT
			COUNT(*),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(cached_tokens), 0),
			COALESCE(SUM(cost_micros), 0),
			COALESCE(SUM(cache_hit), 0)
		FROM usage_records
		WHERE tenant_id = ? AND created_at >= ?`)
	var s Summary
	err := r.db.sql.QueryRowContext(ctx, q, tenantID, formatTime(since)).Scan(
		&s.TotalRequests, &s.PromptTokens, &s.CompletionTokens,
		&s.CachedTokens, &s.CostMicros, &s.CacheHits)
	if err != nil {
		return Summary{}, fmt.Errorf("store: summarize usage: %w", err)
	}
	return s, nil
}