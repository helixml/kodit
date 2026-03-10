package service

import (
	"context"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStatusStore struct {
	statuses map[int64][]task.Status
	all      []task.Status
}

func (f *fakeStatusStore) Find(_ context.Context, options ...repository.Option) ([]task.Status, error) {
	q := repository.Build(options...)
	result := make([]task.Status, 0, len(f.all))
	for _, s := range f.all {
		if matchesConditions(s, q.Conditions()) {
			result = append(result, s)
		}
	}
	return result, nil
}

func matchesConditions(s task.Status, conditions []repository.Condition) bool {
	for _, c := range conditions {
		if c.Field() != "state" {
			continue
		}
		if c.In() {
			vals, ok := c.Value().([]string)
			if !ok {
				return false
			}
			found := false
			for _, v := range vals {
				if string(s.State()) == v {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		} else if string(s.State()) != c.Value() {
			return false
		}
	}
	return true
}

func (f *fakeStatusStore) FindOne(_ context.Context, _ ...repository.Option) (task.Status, error) {
	return task.Status{}, nil
}

func (f *fakeStatusStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	return 0, nil
}

func (f *fakeStatusStore) Save(_ context.Context, s task.Status) (task.Status, error) {
	return s, nil
}

func (f *fakeStatusStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return false, nil
}

func (f *fakeStatusStore) DeleteBy(_ context.Context, _ ...repository.Option) error {
	return nil
}

func (f *fakeStatusStore) LoadWithHierarchy(_ context.Context, _ task.TrackableType, trackableID int64) ([]task.Status, error) {
	if f.statuses == nil {
		return nil, nil
	}
	return f.statuses[trackableID], nil
}

func TestTracking_ActiveStatuses(t *testing.T) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	statusStore := &fakeStatusStore{
		all: []task.Status{
			task.NewStatusFull("s1", task.ReportingStateStarted, "sync", "", epoch, epoch, 0, 0, "", nil, 1, task.TrackableTypeRepository),
			task.NewStatusFull("s2", task.ReportingStateInProgress, "index", "", epoch, epoch, 10, 3, "", nil, 2, task.TrackableTypeRepository),
			task.NewStatusFull("s3", task.ReportingStateCompleted, "enrich", "", epoch, epoch, 5, 5, "", nil, 1, task.TrackableTypeRepository),
			task.NewStatusFull("s4", task.ReportingStateFailed, "sync", "boom", epoch, epoch, 0, 0, "", nil, 3, task.TrackableTypeRepository),
			task.NewStatusFull("s5", task.ReportingStateStarted, "enrich", "", epoch, epoch, 0, 0, "", nil, 3, task.TrackableTypeRepository),
		},
	}

	tracking := NewTracking(statusStore, nil)

	statuses, err := tracking.ActiveStatuses(context.Background())
	require.NoError(t, err)

	assert.Len(t, statuses, 3)
	ids := make([]string, len(statuses))
	for i, s := range statuses {
		ids[i] = s.ID()
	}
	assert.ElementsMatch(t, []string{"s1", "s2", "s5"}, ids)
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
