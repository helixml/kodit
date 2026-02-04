package service

import (
	"context"

	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/domain/tracking"
)

// TrackingQuery provides read-only access to task status information.
type TrackingQuery struct {
	statusStore task.StatusStore
	taskStore   task.TaskStore
}

// NewTrackingQuery creates a new TrackingQuery.
func NewTrackingQuery(
	statusStore task.StatusStore,
	taskStore task.TaskStore,
) *TrackingQuery {
	return &TrackingQuery{
		statusStore: statusStore,
		taskStore:   taskStore,
	}
}

// StatusesForRepository returns all task statuses for a specific repository.
func (s *TrackingQuery) StatusesForRepository(ctx context.Context, repoID int64) ([]task.Status, error) {
	return s.statusStore.LoadWithHierarchy(
		ctx,
		task.TrackableTypeRepository,
		repoID,
	)
}

// PendingTaskCountForRepository returns the count of pending queue tasks for a repository.
func (s *TrackingQuery) PendingTaskCountForRepository(ctx context.Context, repoID int64) (int, error) {
	if s.taskStore == nil {
		return 0, nil
	}

	// Count pending tasks - proper filtering would need
	// repository-specific task operation filtering via payload
	count, err := s.taskStore.CountPending(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// SummaryForRepository returns an aggregated status summary for a repository.
func (s *TrackingQuery) SummaryForRepository(ctx context.Context, repoID int64) (tracking.RepositoryStatusSummary, error) {
	statuses, err := s.StatusesForRepository(ctx, repoID)
	if err != nil {
		return tracking.RepositoryStatusSummary{}, err
	}

	pendingCount, err := s.PendingTaskCountForRepository(ctx, repoID)
	if err != nil {
		return tracking.RepositoryStatusSummary{}, err
	}

	return tracking.StatusSummaryFromTasks(statuses, pendingCount), nil
}
