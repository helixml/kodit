package service

import (
	"context"

	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/domain/tracking"
)

// Tracking provides read-only access to task status information.
type Tracking struct {
	statusStore task.StatusStore
	taskStore   task.TaskStore
}

// NewTracking creates a new Tracking service.
func NewTracking(
	statusStore task.StatusStore,
	taskStore task.TaskStore,
) *Tracking {
	return &Tracking{
		statusStore: statusStore,
		taskStore:   taskStore,
	}
}

// Statuses returns all task statuses for a repository.
func (s *Tracking) Statuses(ctx context.Context, repositoryID int64) ([]task.Status, error) {
	return s.StatusesForRepository(ctx, repositoryID)
}

// Summary returns an aggregated status summary for a repository.
func (s *Tracking) Summary(ctx context.Context, repositoryID int64) (tracking.RepositoryStatusSummary, error) {
	return s.SummaryForRepository(ctx, repositoryID)
}

// StatusesForRepository returns all task statuses for a specific repository.
func (s *Tracking) StatusesForRepository(ctx context.Context, repoID int64) ([]task.Status, error) {
	return s.statusStore.LoadWithHierarchy(
		ctx,
		task.TrackableTypeRepository,
		repoID,
	)
}

// PendingTaskCountForRepository returns the count of pending queue tasks for a repository.
func (s *Tracking) PendingTaskCountForRepository(ctx context.Context, repoID int64) (int, error) {
	if s.taskStore == nil {
		return 0, nil
	}

	count, err := s.taskStore.CountPending(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// SummaryForRepository returns an aggregated status summary for a repository.
func (s *Tracking) SummaryForRepository(ctx context.Context, repoID int64) (tracking.RepositoryStatusSummary, error) {
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
