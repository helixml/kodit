package tracking

import (
	"time"

	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

// RepositoryStatusSummary provides a summary of repository indexing status.
// It aggregates task status information into a high-level view.
type RepositoryStatusSummary struct {
	status    snippet.IndexStatus
	message   string
	updatedAt time.Time
}

// NewRepositoryStatusSummary creates a new RepositoryStatusSummary.
func NewRepositoryStatusSummary(status snippet.IndexStatus, message string, updatedAt time.Time) RepositoryStatusSummary {
	return RepositoryStatusSummary{
		status:    status,
		message:   message,
		updatedAt: updatedAt,
	}
}

// Status returns the overall indexing status.
func (s RepositoryStatusSummary) Status() snippet.IndexStatus {
	return s.status
}

// Message returns the status message (typically error message if failed).
func (s RepositoryStatusSummary) Message() string {
	return s.message
}

// UpdatedAt returns the timestamp of the most recent activity.
func (s RepositoryStatusSummary) UpdatedAt() time.Time {
	return s.updatedAt
}

// StatusSummaryFromTasks derives a RepositoryStatusSummary from task statuses.
// Priority: in_progress/started > pending_queue > completed_with_errors/failed > completed > pending.
// When all tasks are terminal and failures exist, returns completed_with_errors
// if more tasks succeeded than failed, otherwise returns failed.
func StatusSummaryFromTasks(tasks []task.Status, pendingTaskCount int) RepositoryStatusSummary {
	now := time.Now()

	if len(tasks) == 0 {
		if pendingTaskCount > 0 {
			return NewRepositoryStatusSummary(snippet.IndexStatusInProgress, "", now)
		}
		return NewRepositoryStatusSummary(snippet.IndexStatusPending, "", now)
	}

	// Check for in-progress tasks (highest priority — work is still running)
	var mostRecentInProgress *task.Status
	for i := range tasks {
		t := &tasks[i]
		state := t.State()
		if state == task.ReportingStateInProgress || state == task.ReportingStateStarted {
			if mostRecentInProgress == nil || t.UpdatedAt().After(mostRecentInProgress.UpdatedAt()) {
				mostRecentInProgress = t
			}
		}
	}
	if mostRecentInProgress != nil {
		return NewRepositoryStatusSummary(
			snippet.IndexStatusInProgress,
			"",
			mostRecentInProgress.UpdatedAt(),
		)
	}

	// If we have pending queue tasks, work is still running
	if pendingTaskCount > 0 {
		return NewRepositoryStatusSummary(snippet.IndexStatusInProgress, "", now)
	}

	// Count terminal states and track most recent of each
	var (
		completedCount      int
		failedCount         int
		mostRecentFailed    *task.Status
		mostRecentCompleted *task.Status
	)
	for i := range tasks {
		t := &tasks[i]
		switch t.State() {
		case task.ReportingStateCompleted, task.ReportingStateSkipped:
			completedCount++
			if mostRecentCompleted == nil || t.UpdatedAt().After(mostRecentCompleted.UpdatedAt()) {
				mostRecentCompleted = t
			}
		case task.ReportingStateFailed:
			failedCount++
			if mostRecentFailed == nil || t.UpdatedAt().After(mostRecentFailed.UpdatedAt()) {
				mostRecentFailed = t
			}
		}
	}

	if mostRecentFailed != nil && completedCount > failedCount {
		return NewRepositoryStatusSummary(
			snippet.IndexStatusCompletedWithErrors,
			mostRecentFailed.Error(),
			mostRecentFailed.UpdatedAt(),
		)
	}
	if mostRecentFailed != nil {
		return NewRepositoryStatusSummary(
			snippet.IndexStatusFailed,
			mostRecentFailed.Error(),
			mostRecentFailed.UpdatedAt(),
		)
	}
	if mostRecentCompleted != nil {
		return NewRepositoryStatusSummary(
			snippet.IndexStatusCompleted,
			"",
			mostRecentCompleted.UpdatedAt(),
		)
	}

	// Default to pending
	return NewRepositoryStatusSummary(snippet.IndexStatusPending, "", now)
}
