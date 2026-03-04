package service

import (
	"context"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStatusStore struct {
	statuses map[int64][]task.Status
}

func (f *fakeStatusStore) Get(_ context.Context, _ string) (task.Status, error) {
	return task.Status{}, nil
}

func (f *fakeStatusStore) FindByTrackable(_ context.Context, _ task.TrackableType, _ int64) ([]task.Status, error) {
	return nil, nil
}

func (f *fakeStatusStore) Save(_ context.Context, s task.Status) (task.Status, error) {
	return s, nil
}

func (f *fakeStatusStore) SaveBulk(_ context.Context, statuses []task.Status) ([]task.Status, error) {
	return statuses, nil
}

func (f *fakeStatusStore) Delete(_ context.Context, _ task.Status) error {
	return nil
}

func (f *fakeStatusStore) DeleteByTrackable(_ context.Context, _ task.TrackableType, _ int64) error {
	return nil
}

func (f *fakeStatusStore) Count(_ context.Context) (int64, error) {
	return 0, nil
}

func (f *fakeStatusStore) LoadWithHierarchy(_ context.Context, _ task.TrackableType, trackableID int64) ([]task.Status, error) {
	if f.statuses == nil {
		return nil, nil
	}
	return f.statuses[trackableID], nil
}

func TestTracking_Summary_PendingCountScopedToRepository(t *testing.T) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Repo 1 has a completed status — its indexing is done.
	statusStore := &fakeStatusStore{
		statuses: map[int64][]task.Status{
			1: {
				task.NewStatusFull("s1", task.ReportingStateCompleted, "index", "", epoch, epoch, 0, 0, "", nil, 1, task.TrackableTypeRepository),
			},
		},
	}

	// Only repo 2 has pending tasks in the queue.
	taskStore := &fakeTaskStore{
		tasks: []task.Task{
			task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(2)}),
			task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(2)}),
		},
	}

	tracking := NewTracking(statusStore, taskStore)

	summary, err := tracking.Summary(context.Background(), 1)
	require.NoError(t, err)

	// Repo 1 should be completed — it has no pending tasks of its own.
	// BUG: before the fix this returns InProgress because pending
	// tasks for repo 2 are counted globally.
	assert.Equal(t, snippet.IndexStatusCompleted, summary.Status())
}
