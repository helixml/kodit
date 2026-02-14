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
// Priority: failed > in_progress > pending_queue > completed > pending.
// If there are pending queue tasks, the status is IN_PROGRESS even if
// all current task.Status records are terminal.
func StatusSummaryFromTasks(tasks []task.Status, pendingTaskCount int) RepositoryStatusSummary {
	now := time.Now()

	if len(tasks) == 0 {
		if pendingTaskCount > 0 {
			return NewRepositoryStatusSummary(snippet.IndexStatusInProgress, "", now)
		}
		return NewRepositoryStatusSummary(snippet.IndexStatusPending, "", now)
	}

	// Check for failed tasks
	var mostRecentFailed *task.Status
	for i := range tasks {
		t := &tasks[i]
		if t.State() == task.ReportingStateFailed {
			if mostRecentFailed == nil || t.UpdatedAt().After(mostRecentFailed.UpdatedAt()) {
				mostRecentFailed = t
			}
		}
	}
	if mostRecentFailed != nil {
		return NewRepositoryStatusSummary(
			snippet.IndexStatusFailed,
			mostRecentFailed.Error(),
			mostRecentFailed.UpdatedAt(),
		)
	}

	// Check for in-progress tasks
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

	// If we have pending queue tasks but all task statuses are terminal,
	// still report as in progress
	if pendingTaskCount > 0 {
		return NewRepositoryStatusSummary(snippet.IndexStatusInProgress, "", now)
	}

	// Check for completed tasks
	var mostRecentCompleted *task.Status
	for i := range tasks {
		t := &tasks[i]
		if t.State() == task.ReportingStateCompleted {
			if mostRecentCompleted == nil || t.UpdatedAt().After(mostRecentCompleted.UpdatedAt()) {
				mostRecentCompleted = t
			}
		}
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
