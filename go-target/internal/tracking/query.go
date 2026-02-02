package tracking

import (
	"context"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/queue"
)

// QueryService provides read-only access to task status information.
type QueryService struct {
	statusRepo queue.TaskStatusRepository
	taskRepo   queue.TaskRepository
}

// NewQueryService creates a new QueryService.
func NewQueryService(
	statusRepo queue.TaskStatusRepository,
	taskRepo queue.TaskRepository,
) *QueryService {
	return &QueryService{
		statusRepo: statusRepo,
		taskRepo:   taskRepo,
	}
}

// StatusesForRepository returns all task statuses for a specific repository.
func (s *QueryService) StatusesForRepository(ctx context.Context, repoID int64) ([]queue.TaskStatus, error) {
	return s.statusRepo.LoadWithHierarchy(
		ctx,
		domain.TrackableTypeRepository,
		repoID,
	)
}

// PendingTaskCountForRepository returns the count of pending queue tasks for a repository.
func (s *QueryService) PendingTaskCountForRepository(ctx context.Context, repoID int64) (int, error) {
	if s.taskRepo == nil {
		return 0, nil
	}

	// Build a query that filters for this repository
	// Tasks have payload with repo_id field
	query := database.NewQuery()
	// Note: Querying by JSON payload field requires specific implementation
	// For now, count all pending tasks - proper filtering would need
	// repository-specific task operation filtering

	count, err := s.taskRepo.Count(ctx, query)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// SummaryForRepository returns an aggregated status summary for a repository.
func (s *QueryService) SummaryForRepository(ctx context.Context, repoID int64) (RepositoryStatusSummary, error) {
	statuses, err := s.StatusesForRepository(ctx, repoID)
	if err != nil {
		return RepositoryStatusSummary{}, err
	}

	pendingCount, err := s.PendingTaskCountForRepository(ctx, repoID)
	if err != nil {
		return RepositoryStatusSummary{}, err
	}

	return StatusSummaryFromTasks(statuses, pendingCount), nil
}
