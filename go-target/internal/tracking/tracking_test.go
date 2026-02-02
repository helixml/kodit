package tracking

import (
	"testing"
	"time"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/queue"
	"github.com/stretchr/testify/assert"
)

func TestNewTrackable(t *testing.T) {
	trackable := NewTrackable(ReferenceTypeBranch, "main", 42)

	assert.Equal(t, ReferenceTypeBranch, trackable.Type())
	assert.Equal(t, "main", trackable.Identifier())
	assert.Equal(t, int64(42), trackable.RepoID())
}

func TestTrackable_IsBranch(t *testing.T) {
	branch := NewTrackable(ReferenceTypeBranch, "main", 1)
	tag := NewTrackable(ReferenceTypeTag, "v1.0.0", 1)
	commit := NewTrackable(ReferenceTypeCommitSHA, "abc123", 1)

	assert.True(t, branch.IsBranch())
	assert.False(t, tag.IsBranch())
	assert.False(t, commit.IsBranch())
}

func TestTrackable_IsTag(t *testing.T) {
	branch := NewTrackable(ReferenceTypeBranch, "main", 1)
	tag := NewTrackable(ReferenceTypeTag, "v1.0.0", 1)
	commit := NewTrackable(ReferenceTypeCommitSHA, "abc123", 1)

	assert.False(t, branch.IsTag())
	assert.True(t, tag.IsTag())
	assert.False(t, commit.IsTag())
}

func TestTrackable_IsCommitSHA(t *testing.T) {
	branch := NewTrackable(ReferenceTypeBranch, "main", 1)
	tag := NewTrackable(ReferenceTypeTag, "v1.0.0", 1)
	commit := NewTrackable(ReferenceTypeCommitSHA, "abc123", 1)

	assert.False(t, branch.IsCommitSHA())
	assert.False(t, tag.IsCommitSHA())
	assert.True(t, commit.IsCommitSHA())
}

func TestReferenceType_String(t *testing.T) {
	assert.Equal(t, "branch", ReferenceTypeBranch.String())
	assert.Equal(t, "tag", ReferenceTypeTag.String())
	assert.Equal(t, "commit_sha", ReferenceTypeCommitSHA.String())
}

func TestRepositoryStatusSummary(t *testing.T) {
	now := time.Now()
	summary := NewRepositoryStatusSummary(domain.IndexStatusInProgress, "indexing", now)

	assert.Equal(t, domain.IndexStatusInProgress, summary.Status())
	assert.Equal(t, "indexing", summary.Message())
	assert.Equal(t, now, summary.UpdatedAt())
}

func TestStatusSummaryFromTasks_NoTasks(t *testing.T) {
	summary := StatusSummaryFromTasks(nil, 0)
	assert.Equal(t, domain.IndexStatusPending, summary.Status())
}

func TestStatusSummaryFromTasks_NoTasksButPending(t *testing.T) {
	summary := StatusSummaryFromTasks(nil, 5)
	assert.Equal(t, domain.IndexStatusInProgress, summary.Status())
}

func TestStatusSummaryFromTasks_Failed(t *testing.T) {
	tasks := []queue.TaskStatus{
		queue.NewTaskStatusWithDefaults(queue.OperationCloneRepository).
			Fail("clone failed"),
		queue.NewTaskStatusWithDefaults(queue.OperationSyncRepository).
			Complete(),
	}

	summary := StatusSummaryFromTasks(tasks, 0)
	assert.Equal(t, domain.IndexStatusFailed, summary.Status())
	assert.Equal(t, "clone failed", summary.Message())
}

func TestStatusSummaryFromTasks_InProgress(t *testing.T) {
	tasks := []queue.TaskStatus{
		queue.NewTaskStatusWithDefaults(queue.OperationCloneRepository).
			SetCurrent(5, "processing"),
		queue.NewTaskStatusWithDefaults(queue.OperationSyncRepository).
			Complete(),
	}

	summary := StatusSummaryFromTasks(tasks, 0)
	assert.Equal(t, domain.IndexStatusInProgress, summary.Status())
}

func TestStatusSummaryFromTasks_CompletedButPendingQueue(t *testing.T) {
	tasks := []queue.TaskStatus{
		queue.NewTaskStatusWithDefaults(queue.OperationCloneRepository).
			Complete(),
	}

	// Even with all tasks completed, pending queue tasks mean in progress
	summary := StatusSummaryFromTasks(tasks, 3)
	assert.Equal(t, domain.IndexStatusInProgress, summary.Status())
}

func TestStatusSummaryFromTasks_Completed(t *testing.T) {
	tasks := []queue.TaskStatus{
		queue.NewTaskStatusWithDefaults(queue.OperationCloneRepository).
			Complete(),
		queue.NewTaskStatusWithDefaults(queue.OperationSyncRepository).
			Complete(),
	}

	summary := StatusSummaryFromTasks(tasks, 0)
	assert.Equal(t, domain.IndexStatusCompleted, summary.Status())
}
