package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/helixml/kodit/internal/domain"
)

func TestNewTaskStatus(t *testing.T) {
	status := NewTaskStatus(
		OperationScanCommit,
		nil,
		domain.TrackableTypeCommit,
		123,
	)

	assert.Equal(t, "kodit.commit-123-kodit.commit.scan", status.ID())
	assert.Equal(t, domain.ReportingStateStarted, status.State())
	assert.Equal(t, OperationScanCommit, status.Operation())
	assert.Equal(t, int64(123), status.TrackableID())
	assert.Equal(t, domain.TrackableTypeCommit, status.TrackableType())
	assert.Nil(t, status.Parent())
	assert.Empty(t, status.Message())
	assert.Empty(t, status.Error())
	assert.Zero(t, status.Total())
	assert.Zero(t, status.Current())
}

func TestNewTaskStatusWithDefaults(t *testing.T) {
	status := NewTaskStatusWithDefaults(OperationCloneRepository)

	assert.Equal(t, "kodit.repository.clone", status.ID())
	assert.Equal(t, domain.ReportingStateStarted, status.State())
	assert.Nil(t, status.Parent())
}

func TestTaskStatus_Skip(t *testing.T) {
	status := NewTaskStatusWithDefaults(OperationScanCommit)
	skipped := status.Skip("already indexed")

	assert.Equal(t, domain.ReportingStateSkipped, skipped.State())
	assert.Equal(t, "already indexed", skipped.Message())
	// Original is unchanged
	assert.Equal(t, domain.ReportingStateStarted, status.State())
}

func TestTaskStatus_Fail(t *testing.T) {
	status := NewTaskStatusWithDefaults(OperationScanCommit)
	failed := status.Fail("repository not found")

	assert.Equal(t, domain.ReportingStateFailed, failed.State())
	assert.Equal(t, "repository not found", failed.Error())
}

func TestTaskStatus_SetTotal(t *testing.T) {
	status := NewTaskStatusWithDefaults(OperationScanCommit)
	updated := status.SetTotal(100)

	assert.Equal(t, 100, updated.Total())
	assert.Zero(t, status.Total()) // Original unchanged
}

func TestTaskStatus_SetCurrent(t *testing.T) {
	status := NewTaskStatusWithDefaults(OperationScanCommit).SetTotal(100)
	updated := status.SetCurrent(50, "processing files")

	assert.Equal(t, domain.ReportingStateInProgress, updated.State())
	assert.Equal(t, 50, updated.Current())
	assert.Equal(t, "processing files", updated.Message())
}

func TestTaskStatus_SetCurrent_WithoutMessage(t *testing.T) {
	status := NewTaskStatusWithDefaults(OperationScanCommit).SetTotal(100)
	updated := status.SetCurrent(25, "")

	assert.Equal(t, 25, updated.Current())
	assert.Empty(t, updated.Message()) // Original message preserved
}

func TestTaskStatus_SetTrackingInfo(t *testing.T) {
	status := NewTaskStatusWithDefaults(OperationScanCommit)
	updated := status.SetTrackingInfo(456, domain.TrackableTypeRepository)

	assert.Equal(t, int64(456), updated.TrackableID())
	assert.Equal(t, domain.TrackableTypeRepository, updated.TrackableType())
}

func TestTaskStatus_Complete(t *testing.T) {
	status := NewTaskStatusWithDefaults(OperationScanCommit).SetTotal(100).SetCurrent(50, "halfway")
	completed := status.Complete()

	assert.Equal(t, domain.ReportingStateCompleted, completed.State())
	assert.Equal(t, 100, completed.Current()) // Set to total
}

func TestTaskStatus_Complete_AlreadyTerminal(t *testing.T) {
	status := NewTaskStatusWithDefaults(OperationScanCommit).Fail("error")
	completed := status.Complete()

	// Should not change state if already terminal
	assert.Equal(t, domain.ReportingStateFailed, completed.State())
}

func TestTaskStatus_CompletionPercent(t *testing.T) {
	tests := []struct {
		name    string
		total   int
		current int
		want    float64
	}{
		{"zero total", 0, 0, 0.0},
		{"zero current", 100, 0, 0.0},
		{"half complete", 100, 50, 50.0},
		{"complete", 100, 100, 100.0},
		{"over complete (capped)", 100, 150, 100.0},
		{"negative (capped)", 100, -10, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := NewTaskStatusWithDefaults(OperationScanCommit).
				SetTotal(tt.total).
				SetCurrent(tt.current, "")
			// For zero total case, we need to set current first
			if tt.total == 0 {
				status = NewTaskStatusWithDefaults(OperationScanCommit)
			}

			assert.InDelta(t, tt.want, status.CompletionPercent(), 0.01)
		})
	}
}

func TestTaskStatus_WithParent(t *testing.T) {
	parent := NewTaskStatus(
		OperationRepository,
		nil,
		domain.TrackableTypeRepository,
		1,
	)

	child := NewTaskStatus(
		OperationCloneRepository,
		&parent,
		domain.TrackableTypeRepository,
		1,
	)

	assert.NotNil(t, child.Parent())
	assert.Equal(t, parent.ID(), child.Parent().ID())
}

func TestNewTaskStatusFull(t *testing.T) {
	now := time.Now()
	parent := NewTaskStatusWithDefaults(OperationRepository)

	status := NewTaskStatusFull(
		"test-id",
		domain.ReportingStateInProgress,
		OperationScanCommit,
		"processing",
		now,
		now.Add(time.Hour),
		100,
		50,
		"",
		&parent,
		123,
		domain.TrackableTypeCommit,
	)

	assert.Equal(t, "test-id", status.ID())
	assert.Equal(t, domain.ReportingStateInProgress, status.State())
	assert.Equal(t, OperationScanCommit, status.Operation())
	assert.Equal(t, "processing", status.Message())
	assert.Equal(t, now, status.CreatedAt())
	assert.Equal(t, now.Add(time.Hour), status.UpdatedAt())
	assert.Equal(t, 100, status.Total())
	assert.Equal(t, 50, status.Current())
	assert.Empty(t, status.Error())
	assert.NotNil(t, status.Parent())
	assert.Equal(t, int64(123), status.TrackableID())
	assert.Equal(t, domain.TrackableTypeCommit, status.TrackableType())
}

func TestCreateStatusID(t *testing.T) {
	tests := []struct {
		name          string
		operation     TaskOperation
		trackableType domain.TrackableType
		trackableID   int64
		want          string
	}{
		{
			name:          "operation only",
			operation:     OperationScanCommit,
			trackableType: "",
			trackableID:   0,
			want:          "kodit.commit.scan",
		},
		{
			name:          "with trackable type only",
			operation:     OperationScanCommit,
			trackableType: domain.TrackableTypeCommit,
			trackableID:   0,
			want:          "kodit.commit-kodit.commit.scan",
		},
		{
			name:          "with trackable type and ID",
			operation:     OperationScanCommit,
			trackableType: domain.TrackableTypeCommit,
			trackableID:   123,
			want:          "kodit.commit-123-kodit.commit.scan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := NewTaskStatus(tt.operation, nil, tt.trackableType, tt.trackableID)
			assert.Equal(t, tt.want, status.ID())
		})
	}
}
