package task

import (
	"testing"
	"time"
)

func TestReportingState_IsTerminal(t *testing.T) {
	tests := []struct {
		state    ReportingState
		terminal bool
	}{
		{ReportingStateStarted, false},
		{ReportingStateInProgress, false},
		{ReportingStateCompleted, true},
		{ReportingStateFailed, true},
		{ReportingStateSkipped, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.terminal {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.terminal)
			}
		})
	}
}

func TestNewStatus(t *testing.T) {
	s := NewStatus(OperationScanCommit, nil, TrackableTypeCommit, 42)

	if s.State() != ReportingStateStarted {
		t.Errorf("State() = %v, want %v", s.State(), ReportingStateStarted)
	}
	if s.Operation() != OperationScanCommit {
		t.Errorf("Operation() = %v, want %v", s.Operation(), OperationScanCommit)
	}
	if s.TrackableID() != 42 {
		t.Errorf("TrackableID() = %v, want 42", s.TrackableID())
	}
	if s.TrackableType() != TrackableTypeCommit {
		t.Errorf("TrackableType() = %v, want %v", s.TrackableType(), TrackableTypeCommit)
	}
	if s.Parent() != nil {
		t.Error("Parent() should be nil")
	}
	if s.ID() == "" {
		t.Error("ID() should not be empty")
	}
	if s.Total() != 0 {
		t.Errorf("Total() = %v, want 0", s.Total())
	}
	if s.Current() != 0 {
		t.Errorf("Current() = %v, want 0", s.Current())
	}
}

func TestNewStatusWithDefaults(t *testing.T) {
	s := NewStatusWithDefaults(OperationCloneRepository)

	if s.Operation() != OperationCloneRepository {
		t.Errorf("Operation() = %v, want %v", s.Operation(), OperationCloneRepository)
	}
	if s.TrackableID() != 0 {
		t.Errorf("TrackableID() = %v, want 0", s.TrackableID())
	}
	if s.TrackableType() != "" {
		t.Errorf("TrackableType() = %q, want empty", s.TrackableType())
	}
}

func TestStatus_Skip(t *testing.T) {
	original := NewStatusWithDefaults(OperationScanCommit)
	skipped := original.Skip("already indexed")

	if skipped.State() != ReportingStateSkipped {
		t.Errorf("State() = %v, want %v", skipped.State(), ReportingStateSkipped)
	}
	if skipped.Message() != "already indexed" {
		t.Errorf("Message() = %q, want %q", skipped.Message(), "already indexed")
	}
	// Original should be unchanged (value type)
	if original.State() != ReportingStateStarted {
		t.Errorf("original State() = %v, want %v", original.State(), ReportingStateStarted)
	}
}

func TestStatus_Fail(t *testing.T) {
	original := NewStatusWithDefaults(OperationScanCommit)
	failed := original.Fail("connection timeout")

	if failed.State() != ReportingStateFailed {
		t.Errorf("State() = %v, want %v", failed.State(), ReportingStateFailed)
	}
	if failed.Error() != "connection timeout" {
		t.Errorf("Error() = %q, want %q", failed.Error(), "connection timeout")
	}
	if original.State() != ReportingStateStarted {
		t.Errorf("original State() = %v, want %v", original.State(), ReportingStateStarted)
	}
}

func TestStatus_SetTotal(t *testing.T) {
	s := NewStatusWithDefaults(OperationScanCommit).SetTotal(50)

	if s.Total() != 50 {
		t.Errorf("Total() = %v, want 50", s.Total())
	}
}

func TestStatus_SetCurrent(t *testing.T) {
	s := NewStatusWithDefaults(OperationScanCommit).SetTotal(10)

	updated := s.SetCurrent(5, "processing file 5")
	if updated.State() != ReportingStateInProgress {
		t.Errorf("State() = %v, want %v", updated.State(), ReportingStateInProgress)
	}
	if updated.Current() != 5 {
		t.Errorf("Current() = %v, want 5", updated.Current())
	}
	if updated.Message() != "processing file 5" {
		t.Errorf("Message() = %q, want %q", updated.Message(), "processing file 5")
	}
}

func TestStatus_SetCurrent_EmptyMessage(t *testing.T) {
	s := NewStatusWithDefaults(OperationScanCommit).
		SetCurrent(1, "first").
		SetCurrent(2, "")

	if s.Message() != "first" {
		t.Errorf("Message() = %q, want %q (should retain previous)", s.Message(), "first")
	}
	if s.Current() != 2 {
		t.Errorf("Current() = %v, want 2", s.Current())
	}
}

func TestStatus_Complete(t *testing.T) {
	s := NewStatusWithDefaults(OperationScanCommit).SetTotal(10).SetCurrent(7, "")

	completed := s.Complete()
	if completed.State() != ReportingStateCompleted {
		t.Errorf("State() = %v, want %v", completed.State(), ReportingStateCompleted)
	}
	if completed.Current() != completed.Total() {
		t.Errorf("Current() = %v, want Total() = %v", completed.Current(), completed.Total())
	}
}

func TestStatus_Complete_AlreadyTerminal(t *testing.T) {
	failed := NewStatusWithDefaults(OperationScanCommit).Fail("broken")
	completed := failed.Complete()

	if completed.State() != ReportingStateFailed {
		t.Errorf("State() = %v, want %v (should not override terminal)", completed.State(), ReportingStateFailed)
	}

	skipped := NewStatusWithDefaults(OperationScanCommit).Skip("not needed")
	completedSkipped := skipped.Complete()

	if completedSkipped.State() != ReportingStateSkipped {
		t.Errorf("State() = %v, want %v (should not override terminal)", completedSkipped.State(), ReportingStateSkipped)
	}
}

func TestStatus_SetTrackingInfo(t *testing.T) {
	s := NewStatusWithDefaults(OperationScanCommit)
	updated := s.SetTrackingInfo(99, TrackableTypeRepository)

	if updated.TrackableID() != 99 {
		t.Errorf("TrackableID() = %v, want 99", updated.TrackableID())
	}
	if updated.TrackableType() != TrackableTypeRepository {
		t.Errorf("TrackableType() = %v, want %v", updated.TrackableType(), TrackableTypeRepository)
	}
}

func TestStatus_CompletionPercent(t *testing.T) {
	tests := []struct {
		name    string
		total   int
		current int
		want    float64
	}{
		{"zero total", 0, 0, 0.0},
		{"zero current", 10, 0, 0.0},
		{"half done", 100, 50, 50.0},
		{"fully done", 10, 10, 100.0},
		{"over 100 clamped", 10, 15, 100.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStatusWithDefaults(OperationScanCommit).
				SetTotal(tt.total).
				SetCurrent(tt.current, "")
			got := s.CompletionPercent()
			if got != tt.want {
				t.Errorf("CompletionPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_UpdatedAtAdvances(t *testing.T) {
	s := NewStatusWithDefaults(OperationScanCommit)
	before := s.UpdatedAt()

	time.Sleep(time.Millisecond)
	updated := s.SetCurrent(1, "tick")

	if !updated.UpdatedAt().After(before) {
		t.Error("UpdatedAt should advance after SetCurrent")
	}
}

func TestNewStatusFull(t *testing.T) {
	now := time.Now()
	parent := NewStatusWithDefaults(OperationRoot)
	s := NewStatusFull(
		"custom-id",
		ReportingStateInProgress,
		OperationScanCommit,
		"scanning",
		now.Add(-time.Hour), now,
		100, 50,
		"",
		&parent,
		7,
		TrackableTypeCommit,
	)

	if s.ID() != "custom-id" {
		t.Errorf("ID() = %q, want %q", s.ID(), "custom-id")
	}
	if s.State() != ReportingStateInProgress {
		t.Errorf("State() = %v, want %v", s.State(), ReportingStateInProgress)
	}
	if s.Message() != "scanning" {
		t.Errorf("Message() = %q, want %q", s.Message(), "scanning")
	}
	if s.Parent() == nil {
		t.Error("Parent() should not be nil")
	}
}

func TestCreateStatusID(t *testing.T) {
	tests := []struct {
		name          string
		operation     Operation
		trackableType TrackableType
		trackableID   int64
		want          string
	}{
		{"full", OperationScanCommit, TrackableTypeCommit, 42, "kodit.commit-42-kodit.commit.scan"},
		{"no trackable", OperationCloneRepository, "", 0, "kodit.repository.clone"},
		{"type only", OperationScanCommit, TrackableTypeCommit, 0, "kodit.commit-kodit.commit.scan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createStatusID(tt.operation, tt.trackableType, tt.trackableID)
			if got != tt.want {
				t.Errorf("createStatusID() = %q, want %q", got, tt.want)
			}
		})
	}
}
