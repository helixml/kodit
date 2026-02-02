package tracking

import (
	"context"
	"log/slog"

	"github.com/helixml/kodit/internal/queue"
)

// DBReporter implements Reporter by persisting status changes to the database.
type DBReporter struct {
	repo   queue.TaskStatusRepository
	logger *slog.Logger
}

// NewDBReporter creates a new DBReporter.
func NewDBReporter(repo queue.TaskStatusRepository, logger *slog.Logger) *DBReporter {
	return &DBReporter{
		repo:   repo,
		logger: logger,
	}
}

// OnChange persists the task status to the database.
func (r *DBReporter) OnChange(ctx context.Context, status queue.TaskStatus) error {
	_, err := r.repo.Save(ctx, status)
	if err != nil {
		r.logger.Error("failed to save task status",
			slog.String("error", err.Error()),
			slog.String("operation", status.Operation().String()),
		)
		return err
	}
	return nil
}
