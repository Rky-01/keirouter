package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("store: not found")

// APIKeyRepo persists inbound API keys.
type APIKeyRepo struct{ db *DB }

// APIKeys returns the API key repository.
func (db *DB) APIKeys() *APIKeyRepo { return &APIKeyRepo{db: db} }

// Create inserts a new API key record.
func (r *APIKeyRepo) Create(ctx context.Context, k APIKey) error {
	q := r.db.rebind(`
		INSERT INTO api_keys
			(id, tenant_id, project_id, name, key_hash, lookup_hash, display, scopes, disabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := r.db.sql.ExecContext(ctx, q,
		k.ID, k.TenantID, nullString(k.ProjectID), k.Name, k.KeyHash, k.LookupHash,
		k.Display, k.Scopes, boolToInt(k.Disabled), formatTime(k.CreatedAt))
	if err != nil {
		return fmt.Errorf("store: create api key: %w", err)
	}
	return nil
}

// FindByLookup returns the enabled key matching the sha-256 lookup index. The
// caller still verifies the argon2 hash against the presented plaintext.
func (r *APIKeyRepo) FindByLookup(ctx context.Context, lookup string) (APIKey, error) {
	q := r.db.rebind(`
		SELECT id, tenant_id, project_id, name, key_hash, lookup_hash, display, scopes, disabled, last_used_at, created_at
		FROM api_keys WHERE lookup_hash = ?`)
	return r.scanOne(r.db.sql.QueryRowContext(ctx, q, lookup))
}

// List returns all keys for a tenant, newest first.
func (r *APIKeyRepo) List(ctx context.Context, tenantID string) ([]APIKey, error) {
	q := r.db.rebind(`
		SELECT id, tenant_id, project_id, name, key_hash, lookup_hash, display, scopes, disabled, last_used_at, created_at
		FROM api_keys WHERE tenant_id = ? ORDER BY created_at DESC`)
	rows, err := r.db.sql.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list api keys: %w", err)
	}
	defer rows.Close()

	var out []APIKey
	for rows.Next() {
		k, err := r.scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// TouchLastUsed records that a key authenticated a request.
func (r *APIKeyRepo) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	q := r.db.rebind(`UPDATE api_keys SET last_used_at = ? WHERE id = ?`)
	_, err := r.db.sql.ExecContext(ctx, q, formatTime(at), id)
	return err
}

// SetDisabled enables or disables a key.
func (r *APIKeyRepo) SetDisabled(ctx context.Context, id string, disabled bool) error {
	q := r.db.rebind(`UPDATE api_keys SET disabled = ? WHERE id = ?`)
	_, err := r.db.sql.ExecContext(ctx, q, boolToInt(disabled), id)
	return err
}

// Delete removes a key.
func (r *APIKeyRepo) Delete(ctx context.Context, id string) error {
	q := r.db.rebind(`DELETE FROM api_keys WHERE id = ?`)
	_, err := r.db.sql.ExecContext(ctx, q, id)
	return err
}

func (r *APIKeyRepo) scanOne(row *sql.Row) (APIKey, error) {
	var (
		k          APIKey
		projectID  sql.NullString
		lastUsed   sql.NullString
		disabled   int
		createdRaw string
	)
	err := row.Scan(&k.ID, &k.TenantID, &projectID, &k.Name, &k.KeyHash, &k.LookupHash,
		&k.Display, &k.Scopes, &disabled, &lastUsed, &createdRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return APIKey{}, ErrNotFound
	}
	if err != nil {
		return APIKey{}, fmt.Errorf("store: scan api key: %w", err)
	}
	k.ProjectID = projectID.String
	k.Disabled = disabled != 0
	k.CreatedAt = parseTime(createdRaw)
	if lastUsed.Valid {
		t := parseTime(lastUsed.String)
		k.LastUsedAt = &t
	}
	return k, nil
}

func (r *APIKeyRepo) scanRows(rows *sql.Rows) (APIKey, error) {
	var (
		k          APIKey
		projectID  sql.NullString
		lastUsed   sql.NullString
		disabled   int
		createdRaw string
	)
	err := rows.Scan(&k.ID, &k.TenantID, &projectID, &k.Name, &k.KeyHash, &k.LookupHash,
		&k.Display, &k.Scopes, &disabled, &lastUsed, &createdRaw)
	if err != nil {
		return APIKey{}, fmt.Errorf("store: scan api key: %w", err)
	}
	k.ProjectID = projectID.String
	k.Disabled = disabled != 0
	k.CreatedAt = parseTime(createdRaw)
	if lastUsed.Valid {
		t := parseTime(lastUsed.String)
		k.LastUsedAt = &t
	}
	return k, nil
}