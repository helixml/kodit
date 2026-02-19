package tracking

import (
	"testing"
	"time"

	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

func statusWithState(state task.ReportingState, updatedAt time.Time) task.Status {
	return task.NewStatusFull(
		"test-id",
		state,
		task.OperationScanCommit,
		"",
		time.Now(), updatedAt,
		0, 0,
		"",
		nil, 0, "",
	)
}

func failedStatus(errorMsg string, updatedAt time.Time) task.Status {
	return task.NewStatusFull(
		"test-id",
		task.ReportingStateFailed,
		task.OperationScanCommit,
		"",
		time.Now(), updatedAt,
		0, 0,
		errorMsg,
		nil, 0, "",
	)
}

func TestStatusSummaryFromTasks_Empty_NoPending(t *testing.T) {
	summary := StatusSummaryFromTasks(nil, 0)

	if summary.Status() != snippet.IndexStatusPending {
		t.Errorf("Status() = %v, want %v", summary.Status(), snippet.IndexStatusPending)
	}
}

func TestStatusSummaryFromTasks_Empty_WithPendingTasks(t *testing.T) {
	summary := StatusSummaryFromTasks(nil, 5)

	if summary.Status() != snippet.IndexStatusInProgress {
		t.Errorf("Status() = %v, want %v", summary.Status(), snippet.IndexStatusInProgress)
	}
}

func TestStatusSummaryFromTasks_FailedTakesPriority(t *testing.T) {
	now := time.Now()
	tasks := []task.Status{
		statusWithState(task.ReportingStateCompleted, now.Add(-time.Minute)),
		failedStatus("disk full", now),
		statusWithState(task.ReportingStateInProgress, now.Add(-2*time.Minute)),
	}

	summary := StatusSummaryFromTasks(tasks, 0)

	if summary.Status() != snippet.IndexStatusFailed {
		t.Errorf("Status() = %v, want %v", summary.Status(), snippet.IndexStatusFailed)
	}
	if summary.Message() != "disk full" {
		t.Errorf("Message() = %q, want %q", summary.Message(), "disk full")
	}
}

func TestStatusSummaryFromTasks_MostRecentFailedWins(t *testing.T) {
	now := time.Now()
	tasks := []task.Status{
		failedStatus("old error", now.Add(-time.Hour)),
		failedStatus("recent error", now),
	}

	summary := StatusSummaryFromTasks(tasks, 0)

	if summary.Message() != "recent error" {
		t.Errorf("Message() = %q, want %q", summary.Message(), "recent error")
	}
	if !summary.UpdatedAt().Equal(now) {
		t.Errorf("UpdatedAt() = %v, want %v", summary.UpdatedAt(), now)
	}
}

func TestStatusSummaryFromTasks_InProgressOverCompleted(t *testing.T) {
	now := time.Now()
	tasks := []task.Status{
		statusWithState(task.ReportingStateCompleted, now.Add(-time.Minute)),
		statusWithState(task.ReportingStateInProgress, now),
	}

	summary := StatusSummaryFromTasks(tasks, 0)

	if summary.Status() != snippet.IndexStatusInProgress {
		t.Errorf("Status() = %v, want %v", summary.Status(), snippet.IndexStatusInProgress)
	}
}

func TestStatusSummaryFromTasks_StartedCountsAsInProgress(t *testing.T) {
	now := time.Now()
	tasks := []task.Status{
		statusWithState(task.ReportingStateStarted, now),
	}

	summary := StatusSummaryFromTasks(tasks, 0)

	if summary.Status() != snippet.IndexStatusInProgress {
		t.Errorf("Status() = %v, want %v", summary.Status(), snippet.IndexStatusInProgress)
	}
}

func TestStatusSummaryFromTasks_PendingQueueOverridesTerminal(t *testing.T) {
	now := time.Now()
	tasks := []task.Status{
		statusWithState(task.ReportingStateCompleted, now),
	}

	summary := StatusSummaryFromTasks(tasks, 3)

	if summary.Status() != snippet.IndexStatusInProgress {
		t.Errorf("Status() = %v, want %v (pending queue tasks should override)", summary.Status(), snippet.IndexStatusInProgress)
	}
}

func TestStatusSummaryFromTasks_AllCompleted(t *testing.T) {
	now := time.Now()
	tasks := []task.Status{
		statusWithState(task.ReportingStateCompleted, now.Add(-time.Minute)),
		statusWithState(task.ReportingStateCompleted, now),
	}

	summary := StatusSummaryFromTasks(tasks, 0)

	if summary.Status() != snippet.IndexStatusCompleted {
		t.Errorf("Status() = %v, want %v", summary.Status(), snippet.IndexStatusCompleted)
	}
	if !summary.UpdatedAt().Equal(now) {
		t.Errorf("UpdatedAt() should reflect most recent completed task")
	}
}

func TestStatusSummaryFromTasks_AllSkipped(t *testing.T) {
	now := time.Now()
	tasks := []task.Status{
		statusWithState(task.ReportingStateSkipped, now),
	}

	// Skipped tasks are not completed/failed/in-progress, so default to pending
	summary := StatusSummaryFromTasks(tasks, 0)

	if summary.Status() != snippet.IndexStatusPending {
		t.Errorf("Status() = %v, want %v", summary.Status(), snippet.IndexStatusPending)
	}
}

func TestRepositoryStatusSummary_Accessors(t *testing.T) {
	now := time.Now()
	summary := NewRepositoryStatusSummary(snippet.IndexStatusCompleted, "all done", now)

	if summary.Status() != snippet.IndexStatusCompleted {
		t.Errorf("Status() = %v, want %v", summary.Status(), snippet.IndexStatusCompleted)
	}
	if summary.Message() != "all done" {
		t.Errorf("Message() = %q, want %q", summary.Message(), "all done")
	}
	if !summary.UpdatedAt().Equal(now) {
		t.Errorf("UpdatedAt() = %v, want %v", summary.UpdatedAt(), now)
	}
}
