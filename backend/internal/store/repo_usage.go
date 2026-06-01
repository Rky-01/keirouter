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

// ProviderUsage aggregates usage for a single upstream provider. It powers the
// routing-activity map and per-provider breakdown on the Usage page.
type ProviderUsage struct {
	Provider         string
	TotalRequests    int64
	PromptTokens     int64
	CompletionTokens int64
	CostMicros       int64
}

// Breakdown returns per-provider aggregate usage for a tenant since the given
// time, ordered by request volume (busiest first).
func (r *UsageRepo) Breakdown(ctx context.Context, tenantID string, since time.Time) ([]ProviderUsage, error) {
	q := r.db.rebind(`
		SELECT
			provider,
			COUNT(*),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(cost_micros), 0)
		FROM usage_records
		WHERE tenant_id = ? AND created_at >= ?
		GROUP BY provider
		ORDER BY COUNT(*) DESC`)
	rows, err := r.db.sql.QueryContext(ctx, q, tenantID, formatTime(since))
	if err != nil {
		return nil, fmt.Errorf("store: usage breakdown: %w", err)
	}
	defer rows.Close()

	var out []ProviderUsage
	for rows.Next() {
		var p ProviderUsage
		if err := rows.Scan(&p.Provider, &p.TotalRequests, &p.PromptTokens, &p.CompletionTokens, &p.CostMicros); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecentRecord is a single recent request row for the activity feed and console.
type RecentRecord struct {
	ID               string
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	CostMicros       int64
	CacheHit         bool
	LatencyMS        int
	CreatedAt        time.Time
}

// Recent returns the most recent usage records for a tenant, newest first.
func (r *UsageRepo) Recent(ctx context.Context, tenantID string, limit int) ([]RecentRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	q := r.db.rebind(`
		SELECT id, provider, model, prompt_tokens, completion_tokens, cached_tokens,
		       cost_micros, cache_hit, latency_ms, created_at
		FROM usage_records
		WHERE tenant_id = ?
		ORDER BY created_at DESC
		LIMIT ?`)
	rows, err := r.db.sql.QueryContext(ctx, q, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: recent usage: %w", err)
	}
	defer rows.Close()

	var out []RecentRecord
	for rows.Next() {
		var (
			rec       RecentRecord
			cacheHit  int
			createdAt string
		)
		if err := rows.Scan(&rec.ID, &rec.Provider, &rec.Model, &rec.PromptTokens,
			&rec.CompletionTokens, &rec.CachedTokens, &rec.CostMicros, &cacheHit,
			&rec.LatencyMS, &createdAt); err != nil {
			return nil, err
		}
		rec.CacheHit = cacheHit != 0
		rec.CreatedAt = parseTime(createdAt)
		out = append(out, rec)
	}
	return out, rows.Err()
}

// AccountUsage aggregates usage keyed by upstream account. It powers the quota
// tracker, which shows how much each connected account has consumed.
type AccountUsage struct {
	AccountID        string
	TotalRequests    int64
	PromptTokens     int64
	CompletionTokens int64
	CachedTokens     int64
	CostMicros       int64
}

// ByAccount returns per-account aggregate usage for a tenant since the given
// time. Records with no associated account are grouped under an empty id.
func (r *UsageRepo) ByAccount(ctx context.Context, tenantID string, since time.Time) ([]AccountUsage, error) {
	q := r.db.rebind(`
		SELECT
			COALESCE(account_id, ''),
			COUNT(*),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(cached_tokens), 0),
			COALESCE(SUM(cost_micros), 0)
		FROM usage_records
		WHERE tenant_id = ? AND created_at >= ?
		GROUP BY account_id`)
	rows, err := r.db.sql.QueryContext(ctx, q, tenantID, formatTime(since))
	if err != nil {
		return nil, fmt.Errorf("store: usage by account: %w", err)
	}
	defer rows.Close()

	var out []AccountUsage
	for rows.Next() {
		var a AccountUsage
		if err := rows.Scan(&a.AccountID, &a.TotalRequests, &a.PromptTokens,
			&a.CompletionTokens, &a.CachedTokens, &a.CostMicros); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// TimePoint is the created_at + cost of a single record, used to build the
// activity-over-time series in the handler (bucketed in Go for engine
// portability).
type TimePoint struct {
	CreatedAt  time.Time
	CostMicros int64
}

// Timeline returns the (created_at, cost) of records for a tenant since the
// given time, oldest first, capped to a sane limit for a local dashboard.
func (r *UsageRepo) Timeline(ctx context.Context, tenantID string, since time.Time) ([]TimePoint, error) {
	q := r.db.rebind(`
		SELECT created_at, cost_micros
		FROM usage_records
		WHERE tenant_id = ? AND created_at >= ?
		ORDER BY created_at ASC
		LIMIT 20000`)
	rows, err := r.db.sql.QueryContext(ctx, q, tenantID, formatTime(since))
	if err != nil {
		return nil, fmt.Errorf("store: usage timeline: %w", err)
	}
	defer rows.Close()

	var out []TimePoint
	for rows.Next() {
		var (
			tp        TimePoint
			createdAt string
		)
		if err := rows.Scan(&createdAt, &tp.CostMicros); err != nil {
			return nil, err
		}
		tp.CreatedAt = parseTime(createdAt)
		out = append(out, tp)
	}
	return out, rows.Err()
}
