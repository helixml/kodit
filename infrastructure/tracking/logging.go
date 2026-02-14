package tracking

import (
	"context"
	"log/slog"

	"github.com/helixml/kodit/domain/task"
)

// LoggingReporter implements Reporter by logging status changes.
type LoggingReporter struct {
	logger *slog.Logger
}

// NewLoggingReporter creates a new LoggingReporter.
func NewLoggingReporter(logger *slog.Logger) *LoggingReporter {
	return &LoggingReporter{
		logger: logger,
	}
}

// OnChange logs the task status change.
func (r *LoggingReporter) OnChange(_ context.Context, status task.Status) error {
	state := status.State()

	if state == task.ReportingStateFailed {
		r.logger.Error(status.Operation().String(),
			slog.String("state", string(state)),
			slog.Float64("completion_percent", status.CompletionPercent()),
			slog.String("error", status.Error()),
		)
	} else {
		r.logger.Info(status.Operation().String(),
			slog.String("state", string(state)),
			slog.Float64("completion_percent", status.CompletionPercent()),
		)
	}

	return nil
}
