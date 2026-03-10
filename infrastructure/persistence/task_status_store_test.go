package persistence

import (
	"context"
	"testing"

	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMigratedTestDB(t *testing.T) database.Database {
	t.Helper()
	ctx := context.Background()
	db, err := database.NewDatabase(ctx, "sqlite:///:memory:")
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestStatusStore_Find_WithActiveState(t *testing.T) {
	db := newMigratedTestDB(t)
	store := NewStatusStore(db)
	ctx := context.Background()

	// Seed statuses in various states.
	started := task.NewStatus("op_a", nil, task.TrackableTypeRepository, 1)
	inProgress := task.NewStatus("op_b", nil, task.TrackableTypeRepository, 2).
		SetCurrent(5, "working")
	completed := task.NewStatus("op_c", nil, task.TrackableTypeRepository, 3).
		Complete()
	failed := task.NewStatus("op_d", nil, task.TrackableTypeRepository, 4).
		Fail("previous error")

	for _, s := range []task.Status{started, inProgress, completed, failed} {
		_, err := store.Save(ctx, s)
		require.NoError(t, err)
	}

	// Find only started and in_progress statuses.
	stale, err := store.Find(ctx, task.WithActiveState())
	require.NoError(t, err)
	assert.Len(t, stale, 2)

	states := map[task.ReportingState]bool{}
	for _, s := range stale {
		states[s.State()] = true
	}
	assert.True(t, states[task.ReportingStateStarted])
	assert.True(t, states[task.ReportingStateInProgress])
}

func TestStatusStore_Find_NoMatches(t *testing.T) {
	db := newMigratedTestDB(t)
	store := NewStatusStore(db)
	ctx := context.Background()

	completed := task.NewStatus("op_a", nil, task.TrackableTypeRepository, 1).
		Complete()
	_, err := store.Save(ctx, completed)
	require.NoError(t, err)

	stale, err := store.Find(ctx, task.WithActiveState())
	require.NoError(t, err)
	assert.Empty(t, stale)
}
