package tracking

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/task"
)

// LoggingReporter implements Reporter by logging status changes.
type LoggingReporter struct {
	logger zerolog.Logger
}

// NewLoggingReporter creates a new LoggingReporter.
func NewLoggingReporter(logger zerolog.Logger) *LoggingReporter {
	return &LoggingReporter{
		logger: logger,
	}
}

// OnChange logs the task status change.
func (r *LoggingReporter) OnChange(_ context.Context, status task.Status) error {
	state := status.State()

	if state == task.ReportingStateFailed {
		r.logger.Debug().
			Str("state", string(state)).
			Float64("completion_percent", status.CompletionPercent()).
			Str("error", status.Error()).
			Msg(status.Operation().String())
	} else {
		r.logger.Info().
			Str("state", string(state)).
			Float64("completion_percent", status.CompletionPercent()).
			Msg(status.Operation().String())
	}

	return nil
}
