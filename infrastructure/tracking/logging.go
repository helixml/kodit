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

	var event *zerolog.Event
	if state == task.ReportingStateFailed {
		event = r.logger.Debug()
	} else {
		event = r.logger.Info()
	}

	event.Str("state", string(state)).
		Float64("completion_percent", status.CompletionPercent())

	if status.TrackableID() != 0 {
		event.Int64("repository_id", status.TrackableID())
	}

	for k, v := range status.Labels() {
		event.Str(k, v)
	}

	if state == task.ReportingStateFailed {
		event.Str("error", status.Error())
	}

	event.Msg(status.Operation().String())

	return nil
}
