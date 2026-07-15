package health

import (
	"context"
	"testing"
	"time"

	"github.com/mydisha/keirouter/backend/internal/config"
	"github.com/mydisha/keirouter/backend/internal/store"
	"github.com/stretchr/testify/require"
)

func TestCompletedHealthBucketRemainsInRollingWindowAndAcceptsLateEvents(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, config.DatabaseConfig{Driver: "sqlite", DSN: ":memory:"}, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(ctx))

	repo := db.ProviderHealth()
	service := New(Config{Enabled: true, RollingWindow: 15 * time.Minute}, nil, repo)
	bucketTime := time.Now().UTC().Truncate(time.Minute).Add(-time.Minute)
	event := ProviderTelemetryEvent{
		Timestamp: bucketTime.Add(5 * time.Second), Provider: "anthropic", Model: "claude-sonnet",
		Capability: "chat_completions", Status: "success", LatencyMs: 1200,
	}
	service.ingest(event)
	service.flushSnapshots(ctx, false)

	key := store.HealthKey("anthropic", "", "claude-sonnet", "chat_completions")
	service.mu.Lock()
	state := service.states[key]
	require.NotNil(t, state)
	require.Len(t, state.buckets, 1, "completed bucket must remain available to rolling current")
	service.mu.Unlock()

	service.flushCurrent(ctx)
	current, err := repo.GetCurrent(ctx, "anthropic", "", "claude-sonnet", "chat_completions")
	require.NoError(t, err)
	require.Equal(t, int64(1), current.RequestCount)

	// The event arrives after the first snapshot. The retained bucket becomes
	// dirty and replaces that snapshot on the next flush.
	event.Timestamp = bucketTime.Add(45 * time.Second)
	event.LatencyMs = 1800
	service.ingest(event)
	service.flushSnapshots(ctx, false)
	service.flushCurrent(ctx)

	snapshots, err := repo.ListSnapshots(ctx, "anthropic", "", "claude-sonnet", "", bucketTime.Add(-time.Minute))
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Equal(t, int64(2), snapshots[0].RequestCount)
	current, err = repo.GetCurrent(ctx, "anthropic", "", "claude-sonnet", "chat_completions")
	require.NoError(t, err)
	require.Equal(t, int64(2), current.RequestCount)
}
