package store

import (
	"context"
	"testing"
	"time"

	"github.com/mydisha/keirouter/backend/internal/config"
	"github.com/stretchr/testify/require"
)

// newTestDB opens a migrated in-memory SQLite database for a single test.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, config.DatabaseConfig{Driver: "sqlite", DSN: ":memory:"}, t.TempDir())
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))
	require.NoError(t, db.Tenants().EnsureDefault(ctx))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMigrate_Idempotent(t *testing.T) {
	db := newTestDB(t)
	// Running again must be a no-op, not an error.
	require.NoError(t, db.Migrate(context.Background()))
}

func TestAPIKeyRepo_CRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	k := APIKey{
		ID:         "key1",
		TenantID:   DefaultTenantID,
		Name:       "laptop",
		KeyHash:    "$argon2id$...",
		LookupHash: "abc123",
		Display:    "kr_AbC1…7xQ2",
		CreatedAt:  time.Now(),
	}
	require.NoError(t, db.APIKeys().Create(ctx, k))

	got, err := db.APIKeys().FindByLookup(ctx, "abc123")
	require.NoError(t, err)
	require.Equal(t, "key1", got.ID)
	require.Equal(t, "laptop", got.Name)
	require.False(t, got.Disabled)

	_, err = db.APIKeys().FindByLookup(ctx, "missing")
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, db.APIKeys().SetDisabled(ctx, "key1", true))
	got, err = db.APIKeys().FindByLookup(ctx, "abc123")
	require.NoError(t, err)
	require.True(t, got.Disabled)

	list, err := db.APIKeys().List(ctx, DefaultTenantID)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, db.APIKeys().Delete(ctx, "key1"))
	_, err = db.APIKeys().FindByLookup(ctx, "abc123")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestAccountRepo_CRUDAndCooldown(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	a := Account{
		ID:               "acc1",
		TenantID:         DefaultTenantID,
		Provider:         "openai",
		Label:            "primary",
		AuthKind:         AuthAPIKey,
		SecretWrappedDEK: "dek",
		SecretCiphertext: "ct",
		Metadata:         "{}",
		Priority:         10,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	require.NoError(t, db.Accounts().Create(ctx, a))

	got, err := db.Accounts().Get(ctx, "acc1")
	require.NoError(t, err)
	require.Equal(t, "openai", got.Provider)
	require.Equal(t, "ct", got.SecretCiphertext)
	require.Nil(t, got.CooldownUntil)

	until := time.Now().Add(5 * time.Minute)
	require.NoError(t, db.Accounts().SetCooldown(ctx, "acc1", until))
	got, err = db.Accounts().Get(ctx, "acc1")
	require.NoError(t, err)
	require.NotNil(t, got.CooldownUntil)

	list, err := db.Accounts().ListByProvider(ctx, DefaultTenantID, "openai")
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, db.Accounts().Delete(ctx, "acc1"))
	_, err = db.Accounts().Get(ctx, "acc1")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestUsageRepo_RecordAndSummarize(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Usage().Record(ctx, UsageRecord{
			ID:               "u" + time.Now().Format("150405.000000000"),
			TenantID:         DefaultTenantID,
			APIKeyID:         "key1",
			Provider:         "openai",
			Model:            "gpt-4o",
			PromptTokens:     100,
			CompletionTokens: 50,
			CostMicros:       1500,
			CreatedAt:        now,
		}))
		time.Sleep(time.Millisecond)
	}

	sum, err := db.Usage().Summarize(ctx, DefaultTenantID, now.Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(3), sum.TotalRequests)
	require.Equal(t, int64(300), sum.PromptTokens)
	require.Equal(t, int64(4500), sum.CostMicros)

	spend, err := db.Usage().SpendSince(ctx, ScopeAPIKey, "key1", now.Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(4500), spend)
}

func TestSettingsRepo_GetSet(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.Settings().Get(ctx, "theme")
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, db.Settings().Set(ctx, "theme", "dark"))
	v, err := db.Settings().Get(ctx, "theme")
	require.NoError(t, err)
	require.Equal(t, "dark", v)

	require.NoError(t, db.Settings().Set(ctx, "theme", "light"))
	v, err = db.Settings().Get(ctx, "theme")
	require.NoError(t, err)
	require.Equal(t, "light", v)
}

func TestAuditRepo_AppendAndList(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		require.NoError(t, db.Audit().Append(ctx, AuditEntry{
			ID:        "a" + time.Now().Format("150405.000000000"),
			TenantID:  DefaultTenantID,
			Actor:     "key1",
			Action:    "proxy.request",
			Detail:    "{}",
			CreatedAt: time.Now(),
		}))
		time.Sleep(time.Millisecond)
	}

	entries, err := db.Audit().List(ctx, DefaultTenantID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 5)
	require.Equal(t, "proxy.request", entries[0].Action)
}