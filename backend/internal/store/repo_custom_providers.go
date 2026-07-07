package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CustomProviderRepo persists user-defined provider instances and the custom
// models attached to any provider (custom or built-in).
type CustomProviderRepo struct{ db *DB }

// CustomProviders returns the custom provider repository.
func (db *DB) CustomProviders() *CustomProviderRepo { return &CustomProviderRepo{db: db} }

// CustomProvider is a user-defined provider instance built on an OpenAI- or
// Anthropic-compatible dialect. Each instance has a unique id so multiple
// endpoints of the same base type stay isolated.
type CustomProvider struct {
	ID          string
	TenantID    string
	DisplayName string
	Alias       string
	Dialect     string
	BaseURL     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CustomModel is a user-registered model on a provider (custom or built-in).
type CustomModel struct {
	ID            string
	TenantID      string
	ProviderID    string
	ModelID       string
	DisplayName   string
	Kind          string
	ContextWindow int
	InputPerM     float64
	OutputPerM    float64
	Source        string // "manual" (user-entered) | "imported" (from /models)
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

const customProviderColumns = `id, tenant_id, display_name, alias, dialect, base_url, created_at, updated_at`

// ListProviders returns all custom provider instances for a tenant.
func (r *CustomProviderRepo) ListProviders(ctx context.Context, tenantID string) ([]CustomProvider, error) {
	q := r.db.rebind(`SELECT ` + customProviderColumns + ` FROM custom_providers WHERE tenant_id = ? ORDER BY display_name`)
	rows, err := r.db.sql.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list custom providers: %w", err)
	}
	defer rows.Close()

	var out []CustomProvider
	for rows.Next() {
		p, err := scanCustomProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProvider returns one custom provider by id.
func (r *CustomProviderRepo) GetProvider(ctx context.Context, id string) (CustomProvider, error) {
	q := r.db.rebind(`SELECT ` + customProviderColumns + ` FROM custom_providers WHERE id = ?`)
	p, err := scanCustomProvider(r.db.sql.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return CustomProvider{}, ErrNotFound
	}
	if err != nil {
		return CustomProvider{}, fmt.Errorf("store: get custom provider: %w", err)
	}
	return p, nil
}

// CreateProvider inserts a new custom provider instance.
func (r *CustomProviderRepo) CreateProvider(ctx context.Context, p CustomProvider) error {
	now := formatTime(time.Now())
	q := r.db.rebind(`INSERT INTO custom_providers (` + customProviderColumns + `)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := r.db.sql.ExecContext(ctx, q,
		p.ID, p.TenantID, p.DisplayName, p.Alias, p.Dialect, p.BaseURL, now, now)
	if err != nil {
		return fmt.Errorf("store: create custom provider: %w", err)
	}
	return nil
}

// UpdateProvider updates the mutable fields of a custom provider.
func (r *CustomProviderRepo) UpdateProvider(ctx context.Context, p CustomProvider) error {
	q := r.db.rebind(`UPDATE custom_providers
		SET display_name = ?, alias = ?, base_url = ?, updated_at = ?
		WHERE id = ?`)
	res, err := r.db.sql.ExecContext(ctx, q,
		p.DisplayName, p.Alias, p.BaseURL, formatTime(time.Now()), p.ID)
	if err != nil {
		return fmt.Errorf("store: update custom provider: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteProvider removes a custom provider and all of its custom models.
func (r *CustomProviderRepo) DeleteProvider(ctx context.Context, id string) error {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, r.db.rebind(`DELETE FROM custom_models WHERE provider_id = ?`), id); err != nil {
		return fmt.Errorf("store: delete custom provider models: %w", err)
	}
	res, err := tx.ExecContext(ctx, r.db.rebind(`DELETE FROM custom_providers WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("store: delete custom provider: %w", err)
	}
	// Explicit not-found signal so callers can map it to a404 without a
	// separate existence probe (which would race with concurrent deletes).
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

const customModelColumns = `id, tenant_id, provider_id, model_id, display_name, kind, context_window, input_per_m, output_per_m, source, created_at, updated_at`

// ListModels returns all custom models for a tenant.
func (r *CustomProviderRepo) ListModels(ctx context.Context, tenantID string) ([]CustomModel, error) {
	q := r.db.rebind(`SELECT ` + customModelColumns + ` FROM custom_models WHERE tenant_id = ? ORDER BY provider_id, model_id`)
	rows, err := r.db.sql.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("store: list custom models: %w", err)
	}
	defer rows.Close()

	var out []CustomModel
	for rows.Next() {
		m, err := scanCustomModel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListModelsByProvider returns custom models attached to a provider id.
func (r *CustomProviderRepo) ListModelsByProvider(ctx context.Context, providerID string) ([]CustomModel, error) {
	q := r.db.rebind(`SELECT ` + customModelColumns + ` FROM custom_models WHERE provider_id = ? ORDER BY model_id`)
	rows, err := r.db.sql.QueryContext(ctx, q, providerID)
	if err != nil {
		return nil, fmt.Errorf("store: list custom models by provider: %w", err)
	}
	defer rows.Close()

	var out []CustomModel
	for rows.Next() {
		m, err := scanCustomModel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListManualModelsByProvider returns only user-entered custom models (source =
// "manual") for a provider. Imported models from /models are excluded — they
// surface in the Available Models catalog, not the editable Custom Models list.
func (r *CustomProviderRepo) ListManualModelsByProvider(ctx context.Context, providerID string) ([]CustomModel, error) {
	q := r.db.rebind(`SELECT ` + customModelColumns + ` FROM custom_models
		WHERE provider_id = ? AND (source = 'manual' OR source = '')
		ORDER BY model_id`)
	rows, err := r.db.sql.QueryContext(ctx, q, providerID)
	if err != nil {
		return nil, fmt.Errorf("store: list manual custom models by provider: %w", err)
	}
	defer rows.Close()

	var out []CustomModel
	for rows.Next() {
		m, err := scanCustomModel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreateModel inserts a custom model.
func (r *CustomProviderRepo) CreateModel(ctx context.Context, m CustomModel) error {
	now := formatTime(time.Now())
	source := m.Source
	if source == "" {
		source = "manual"
	}
	q := r.db.rebind(`INSERT INTO custom_models (` + customModelColumns + `)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := r.db.sql.ExecContext(ctx, q,
		m.ID, m.TenantID, m.ProviderID, m.ModelID, m.DisplayName, m.Kind,
		m.ContextWindow, m.InputPerM, m.OutputPerM, source, now, now)
	if err != nil {
		return fmt.Errorf("store: create custom model: %w", err)
	}
	return nil
}

// UpdateModel updates a custom model's mutable fields.
func (r *CustomProviderRepo) UpdateModel(ctx context.Context, m CustomModel) error {
	q := r.db.rebind(`UPDATE custom_models
		SET model_id = ?, display_name = ?, kind = ?, context_window = ?, input_per_m = ?, output_per_m = ?, updated_at = ?
		WHERE id = ?`)
	res, err := r.db.sql.ExecContext(ctx, q,
		m.ModelID, m.DisplayName, m.Kind, m.ContextWindow, m.InputPerM, m.OutputPerM,
		formatTime(time.Now()), m.ID)
	if err != nil {
		return fmt.Errorf("store: update custom model: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetModel returns one custom model by id.
func (r *CustomProviderRepo) GetModel(ctx context.Context, id string) (CustomModel, error) {
	q := r.db.rebind(`SELECT ` + customModelColumns + ` FROM custom_models WHERE id = ?`)
	m, err := scanCustomModel(r.db.sql.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return CustomModel{}, ErrNotFound
	}
	if err != nil {
		return CustomModel{}, fmt.Errorf("store: get custom model: %w", err)
	}
	return m, nil
}

// DeleteModel removes a custom model by id.
func (r *CustomProviderRepo) DeleteModel(ctx context.Context, id string) error {
	q := r.db.rebind(`DELETE FROM custom_models WHERE id = ?`)
	res, err := r.db.sql.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("store: delete custom model: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner abstracts *sql.Row and *sql.Rows for shared scan helpers.
type scanner interface {
	Scan(dest ...any) error
}

func scanCustomProvider(s scanner) (CustomProvider, error) {
	var p CustomProvider
	var created, updated string
	if err := s.Scan(&p.ID, &p.TenantID, &p.DisplayName, &p.Alias, &p.Dialect, &p.BaseURL, &created, &updated); err != nil {
		return CustomProvider{}, err
	}
	p.CreatedAt = parseTime(created)
	p.UpdatedAt = parseTime(updated)
	return p, nil
}

func scanCustomModel(s scanner) (CustomModel, error) {
	var m CustomModel
	var created, updated string
	if err := s.Scan(&m.ID, &m.TenantID, &m.ProviderID, &m.ModelID, &m.DisplayName, &m.Kind,
		&m.ContextWindow, &m.InputPerM, &m.OutputPerM, &m.Source, &created, &updated); err != nil {
		return CustomModel{}, err
	}
	m.CreatedAt = parseTime(created)
	m.UpdatedAt = parseTime(updated)
	return m, nil
}
