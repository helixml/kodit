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
	return s.statusStore.LoadWithHierarchy(
		ctx,
		task.TrackableTypeRepository,
		repositoryID,
	)
}

// Summary returns an aggregated status summary for a repository.
func (s *Tracking) Summary(ctx context.Context, repositoryID int64) (tracking.RepositoryStatusSummary, error) {
	statuses, err := s.Statuses(ctx, repositoryID)
	if err != nil {
		return tracking.RepositoryStatusSummary{}, err
	}

	pendingCount, err := s.pendingTaskCount(ctx, repositoryID)
	if err != nil {
		return tracking.RepositoryStatusSummary{}, err
	}

	return tracking.StatusSummaryFromTasks(statuses, pendingCount), nil
}

func (s *Tracking) pendingTaskCount(ctx context.Context, _ int64) (int, error) {
	if s.taskStore == nil {
		return 0, nil
	}

	count, err := s.taskStore.CountPending(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
