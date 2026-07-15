package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProviderHealthCurrent_UpsertAndGet(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := db.ProviderHealth()

	now := time.Now().UTC()
	cur := ProviderHealthCurrent{
		ID:                  "h1",
		Provider:            "openai",
		ProviderAccountID:   "acc_1",
		Model:               "gpt-4o",
		Capability:          "chat_completions",
		HealthStatus:        "degraded",
		HealthScore:         72,
		SuccessRate:         0.912,
		ErrorRate:           0.088,
		RequestCount:        1000,
		FallbackCount:       37,
		LatencyP95Ms:        intPtr9400(),
		ConsecutiveFailures: 2,
		MainIssue:           strPtr("rate_limited"),
		Recommendation:      strPtr("lower concurrency"),
		LastSuccessAt:       &now,
		LastFailureAt:       &now,
		LastUpdatedAt:       now,
	}
	require.NoError(t, repo.UpsertCurrent(ctx, cur))

	got, err := repo.GetCurrent(ctx, "openai", "acc_1", "gpt-4o", "chat_completions")
	require.NoError(t, err)
	require.Equal(t, "degraded", got.HealthStatus)
	require.Equal(t, 72, got.HealthScore)
	require.NotNil(t, got.LatencyP95Ms)
	require.Equal(t, 9400, *got.LatencyP95Ms)
	require.NotNil(t, got.MainIssue)
	require.Equal(t, "rate_limited", *got.MainIssue)

	// Upsert again (same key) replaces the row.
	cur.HealthScore = 50
	cur.HealthStatus = "unhealthy"
	require.NoError(t, repo.UpsertCurrent(ctx, cur))
	got2, err := repo.GetCurrent(ctx, "openai", "acc_1", "gpt-4o", "chat_completions")
	require.NoError(t, err)
	require.Equal(t, "unhealthy", got2.HealthStatus)
	require.Equal(t, 50, got2.HealthScore)
}

func TestProviderHealthCurrent_ListByStatus(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := db.ProviderHealth()

	upsert := func(id, provider, status string) {
		require.NoError(t, repo.UpsertCurrent(ctx, ProviderHealthCurrent{
			ID: id, Provider: provider, HealthStatus: status, HealthScore: 80, LastUpdatedAt: time.Now(),
		}))
	}
	upsert("a", "openai", "healthy")
	upsert("b", "anthropic", "degraded")
	upsert("c", "gemini", "unhealthy")

	all, err := repo.ListCurrent(ctx, "")
	require.NoError(t, err)
	require.Len(t, all, 3)

	degraded, err := repo.ListCurrent(ctx, "degraded")
	require.NoError(t, err)
	require.Len(t, degraded, 1)
	require.Equal(t, "anthropic", degraded[0].Provider)

	byProv, err := repo.ListCurrentByProvider(ctx, "openai")
	require.NoError(t, err)
	require.Len(t, byProv, 1)
}

func TestProviderProbeResult_InsertAndList(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := db.ProviderHealth()

	p := ProviderProbeResult{
		ID:                "p1",
		Provider:          "openai",
		ProviderAccountID: "acc_1",
		Model:             "gpt-4o-mini",
		Capability:        "chat_completions",
		Status:            "success",
		HTTPStatus:        intPtr200(),
		LatencyMs:         intPtr1200(),
		TriggeredBy:       "manual",
		CreatedAt:         time.Now(),
	}
	require.NoError(t, repo.InsertProbeResult(ctx, p))

	items, total, err := repo.ListProbeResults(ctx, "openai", time.Now().Add(-time.Hour), 10, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, "success", items[0].Status)
	require.NotNil(t, items[0].HTTPStatus)
	require.Equal(t, 200, *items[0].HTTPStatus)
}

func TestProviderHealthSnapshot_InsertAndList(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := db.ProviderHealth()

	now := time.Now().UTC()
	snap := ProviderHealthSnapshot{
		ID:                "s1",
		BucketStart:       now.Add(-time.Minute),
		BucketSizeSeconds: 60,
		Provider:          "openai",
		Model:             "gpt-4o",
		Capability:        "chat_completions",
		RequestCount:      100,
		SuccessCount:      92,
		FailureCount:      8,
		FallbackCount:     3,
		HealthScore:       72,
		HealthStatus:      "degraded",
		CreatedAt:         now,
	}
	require.NoError(t, repo.InsertSnapshot(ctx, snap))

	snaps, err := repo.ListSnapshots(ctx, "openai", "", "gpt-4o", "", now.Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	require.Equal(t, int64(100), snaps[0].RequestCount)

	// A late event corrects the same bucket identity instead of appending a
	// duplicate historical row.
	snap.ID = "s2"
	snap.RequestCount = 125
	snap.SuccessCount = 117
	require.NoError(t, repo.InsertSnapshot(ctx, snap))
	snaps, err = repo.ListSnapshots(ctx, "openai", "", "gpt-4o", "", now.Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	require.Equal(t, "s2", snaps[0].ID)
	require.Equal(t, int64(125), snaps[0].RequestCount)
}

func TestProviderHealthCurrent_DeleteTrafficPreservesProbeOnlyRow(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := db.ProviderHealth()
	now := time.Now().UTC()

	require.NoError(t, repo.UpsertCurrent(ctx, ProviderHealthCurrent{
		ID: "traffic", Provider: "openai", Model: "gpt-4o", Capability: "chat_completions",
		HealthStatus: "healthy", HealthScore: 100, RequestCount: 4, LastUpdatedAt: now.Add(-time.Minute),
	}))
	require.NoError(t, repo.UpsertCurrent(ctx, ProviderHealthCurrent{
		ID: "probe", Provider: "anthropic", Model: "claude", Capability: "chat_completions",
		HealthStatus: "healthy", HealthScore: 100, RequestCount: 0, LastProbeAt: &now, LastUpdatedAt: now,
	}))

	require.NoError(t, repo.DeleteTrafficCurrent(ctx, "openai", "", "gpt-4o", "chat_completions", now))
	_, err := repo.GetCurrent(ctx, "openai", "", "gpt-4o", "chat_completions")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = repo.GetCurrent(ctx, "anthropic", "", "claude", "chat_completions")
	require.NoError(t, err)
}

// helpers local to this test file to avoid touching repo helpers.
func intPtr9400() *int        { v := 9400; return &v }
func intPtr200() *int         { v := 200; return &v }
func intPtr1200() *int        { v := 1200; return &v }
func strPtr(s string) *string { return &s }
