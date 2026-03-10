package service

import (
	"context"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracking_ActiveStatuses(t *testing.T) {
	db := testdb.New(t)
	statusStore := persistence.NewStatusStore(db)
	ctx := context.Background()
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	statuses := []task.Status{
		task.NewStatusFull("s1", task.ReportingStateStarted, "sync", "", epoch, epoch, 0, 0, "", nil, 1, task.TrackableTypeRepository),
		task.NewStatusFull("s2", task.ReportingStateInProgress, "index", "", epoch, epoch, 10, 3, "", nil, 2, task.TrackableTypeRepository),
		task.NewStatusFull("s3", task.ReportingStateCompleted, "enrich", "", epoch, epoch, 5, 5, "", nil, 1, task.TrackableTypeRepository),
		task.NewStatusFull("s4", task.ReportingStateFailed, "sync", "boom", epoch, epoch, 0, 0, "", nil, 3, task.TrackableTypeRepository),
		task.NewStatusFull("s5", task.ReportingStateStarted, "enrich", "", epoch, epoch, 0, 0, "", nil, 3, task.TrackableTypeRepository),
	}
	for _, s := range statuses {
		_, err := statusStore.Save(ctx, s)
		require.NoError(t, err)
	}

	tracking := NewTracking(statusStore, nil)

	active, err := tracking.ActiveStatuses(ctx)
	require.NoError(t, err)

	assert.Len(t, active, 3)
	ids := make([]string, len(active))
	for i, s := range active {
		ids[i] = s.ID()
	}
	assert.ElementsMatch(t, []string{"s1", "s2", "s5"}, ids)
}

func TestTracking_Summary_PendingCountScopedToRepository(t *testing.T) {
	db := testdb.New(t)
	statusStore := persistence.NewStatusStore(db)
	taskStore := persistence.NewTaskStore(db)
	ctx := context.Background()
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Repo 1 has a completed status — its indexing is done.
	_, err := statusStore.Save(ctx, task.NewStatusFull(
		"s1", task.ReportingStateCompleted, "index", "", epoch, epoch,
		0, 0, "", nil, 1, task.TrackableTypeRepository,
	))
	require.NoError(t, err)

	// Only repo 2 has a pending task in the queue.
	_, err = taskStore.Save(ctx, task.NewTask(
		task.OperationSyncRepository, int(task.PriorityNormal),
		map[string]any{"repository_id": int64(2)},
	))
	require.NoError(t, err)

	tracking := NewTracking(statusStore, taskStore)

	summary, err := tracking.Summary(ctx, 1)
	require.NoError(t, err)

	// Repo 1 should be completed — it has no pending tasks of its own.
	assert.Equal(t, snippet.IndexStatusCompleted, summary.Status())
}
