package tracking

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/task"
)

// DBReporter implements Reporter by persisting status changes to the database.
type DBReporter struct {
	repo   task.StatusStore
	logger zerolog.Logger
}

// NewDBReporter creates a new DBReporter.
func NewDBReporter(repo task.StatusStore, logger zerolog.Logger) *DBReporter {
	return &DBReporter{
		repo:   repo,
		logger: logger,
	}
}

// OnChange persists the task status to the database.
func (r *DBReporter) OnChange(ctx context.Context, status task.Status) error {
	_, err := r.repo.Save(ctx, status)
	if err != nil {
		r.logger.Error().Str("error", err.Error()).Str("operation", status.Operation().String()).Msg("failed to save task status")
		return err
	}
	return nil
}
