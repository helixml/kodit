package tracking

import (
	"testing"
	"time"

	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
)

// statusAt builds a task.Status at a deterministic time offset with the given
// state and optional error message. Minutes are relative to a fixed epoch so
// "most recent" comparisons are stable.
func statusAt(state task.ReportingState, minutes int, errorMsg string) task.Status {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t := epoch.Add(time.Duration(minutes) * time.Minute)
	return task.NewStatusFull(
		"test", state, "index", "", t, t,
		0, 0, errorMsg, nil, 0, "",
	)
}

func TestStatusSummaryFromTasks_EmptyNoPending(t *testing.T) {
	summary := StatusSummaryFromTasks(nil, 0)
	if summary.Status() != snippet.IndexStatusPending {
		t.Fatalf("want pending, got %s", summary.Status())
	}
}

func TestStatusSummaryFromTasks_EmptyWithPending(t *testing.T) {
	summary := StatusSummaryFromTasks(nil, 3)
	if summary.Status() != snippet.IndexStatusInProgress {
		t.Fatalf("want in_progress, got %s", summary.Status())
	}
}

func TestStatusSummaryFromTasks_AllCompleted(t *testing.T) {
	tasks := []task.Status{
		statusAt(task.ReportingStateCompleted, 1, ""),
		statusAt(task.ReportingStateCompleted, 2, ""),
	}
	summary := StatusSummaryFromTasks(tasks, 0)
	if summary.Status() != snippet.IndexStatusCompleted {
		t.Fatalf("want completed, got %s", summary.Status())
	}
}

func TestStatusSummaryFromTasks_InProgressAndFailed(t *testing.T) {
	tasks := []task.Status{
		statusAt(task.ReportingStateFailed, 1, "boom"),
		statusAt(task.ReportingStateInProgress, 2, ""),
	}
	summary := StatusSummaryFromTasks(tasks, 0)
	if summary.Status() != snippet.IndexStatusInProgress {
		t.Fatalf("want in_progress, got %s", summary.Status())
	}
}

func TestStatusSummaryFromTasks_PendingQueueAndFailed(t *testing.T) {
	tasks := []task.Status{
		statusAt(task.ReportingStateFailed, 1, "boom"),
		statusAt(task.ReportingStateCompleted, 2, ""),
	}
	summary := StatusSummaryFromTasks(tasks, 5)
	if summary.Status() != snippet.IndexStatusInProgress {
		t.Fatalf("want in_progress, got %s", summary.Status())
	}
}

func TestStatusSummaryFromTasks_MajorityCompletedWithFailure(t *testing.T) {
	tasks := []task.Status{
		statusAt(task.ReportingStateCompleted, 1, ""),
		statusAt(task.ReportingStateCompleted, 2, ""),
		statusAt(task.ReportingStateCompleted, 3, ""),
		statusAt(task.ReportingStateFailed, 4, "transient error"),
	}
	summary := StatusSummaryFromTasks(tasks, 0)
	if summary.Status() != snippet.IndexStatusCompletedWithErrors {
		t.Fatalf("want completed_with_errors, got %s", summary.Status())
	}
	if summary.Message() != "transient error" {
		t.Fatalf("want error message 'transient error', got %q", summary.Message())
	}
}

func TestStatusSummaryFromTasks_MajorityFailed(t *testing.T) {
	tasks := []task.Status{
		statusAt(task.ReportingStateCompleted, 1, ""),
		statusAt(task.ReportingStateFailed, 2, "err1"),
		statusAt(task.ReportingStateFailed, 3, "err2"),
		statusAt(task.ReportingStateFailed, 4, "err3"),
	}
	summary := StatusSummaryFromTasks(tasks, 0)
	if summary.Status() != snippet.IndexStatusFailed {
		t.Fatalf("want failed, got %s", summary.Status())
	}
}

func TestStatusSummaryFromTasks_EqualCountsFailed(t *testing.T) {
	tasks := []task.Status{
		statusAt(task.ReportingStateCompleted, 1, ""),
		statusAt(task.ReportingStateFailed, 2, "equal"),
	}
	summary := StatusSummaryFromTasks(tasks, 0)
	if summary.Status() != snippet.IndexStatusFailed {
		t.Fatalf("want failed (tie goes to failed), got %s", summary.Status())
	}
}

func TestStatusSummaryFromTasks_SkippedCountsAsCompleted(t *testing.T) {
	tasks := []task.Status{
		statusAt(task.ReportingStateSkipped, 1, ""),
		statusAt(task.ReportingStateSkipped, 2, ""),
		statusAt(task.ReportingStateFailed, 3, "one fail"),
	}
	summary := StatusSummaryFromTasks(tasks, 0)
	if summary.Status() != snippet.IndexStatusCompletedWithErrors {
		t.Fatalf("want completed_with_errors (skipped counts as completed), got %s", summary.Status())
	}
}

func TestStatusSummaryFromTasks_SingleFailureAmongMany(t *testing.T) {
	tasks := make([]task.Status, 20)
	for i := range 19 {
		tasks[i] = statusAt(task.ReportingStateCompleted, i+1, "")
	}
	tasks[19] = statusAt(task.ReportingStateFailed, 20, "one bad apple")
	summary := StatusSummaryFromTasks(tasks, 0)
	if summary.Status() != snippet.IndexStatusCompletedWithErrors {
		t.Fatalf("want completed_with_errors, got %s", summary.Status())
	}
	if summary.Message() != "one bad apple" {
		t.Fatalf("want error 'one bad apple', got %q", summary.Message())
	}
}

func TestStatusSummaryFromTasks_MostRecentErrorPreserved(t *testing.T) {
	tasks := []task.Status{
		statusAt(task.ReportingStateCompleted, 1, ""),
		statusAt(task.ReportingStateCompleted, 2, ""),
		statusAt(task.ReportingStateFailed, 3, "old error"),
		statusAt(task.ReportingStateFailed, 10, "newest error"),
		statusAt(task.ReportingStateCompleted, 5, ""),
	}
	summary := StatusSummaryFromTasks(tasks, 0)
	if summary.Status() != snippet.IndexStatusCompletedWithErrors {
		t.Fatalf("want completed_with_errors, got %s", summary.Status())
	}
	if summary.Message() != "newest error" {
		t.Fatalf("want 'newest error', got %q", summary.Message())
	}
}

func TestStatusSummaryFromTasks_StartedTreatedAsInProgress(t *testing.T) {
	tasks := []task.Status{
		statusAt(task.ReportingStateCompleted, 1, ""),
		statusAt(task.ReportingStateStarted, 2, ""),
	}
	summary := StatusSummaryFromTasks(tasks, 0)
	if summary.Status() != snippet.IndexStatusInProgress {
		t.Fatalf("want in_progress (started = in_progress), got %s", summary.Status())
	}
}
