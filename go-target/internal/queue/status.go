package queue

import (
	"fmt"
	"time"

	"github.com/helixml/kodit/internal/domain"
)

// TaskStatus represents the status of a task with progress tracking.
type TaskStatus struct {
	id            string
	state         domain.ReportingState
	operation     TaskOperation
	message       string
	createdAt     time.Time
	updatedAt     time.Time
	total         int
	current       int
	errorMessage  string
	parent        *TaskStatus
	trackableID   int64
	trackableType domain.TrackableType
}

// NewTaskStatus creates a new TaskStatus for the given operation.
func NewTaskStatus(
	operation TaskOperation,
	parent *TaskStatus,
	trackableType domain.TrackableType,
	trackableID int64,
) TaskStatus {
	now := time.Now().UTC()
	return TaskStatus{
		id:            createStatusID(operation, trackableType, trackableID),
		operation:     operation,
		parent:        parent,
		trackableType: trackableType,
		trackableID:   trackableID,
		state:         domain.ReportingStateStarted,
		createdAt:     now,
		updatedAt:     now,
	}
}

// NewTaskStatusWithDefaults creates a TaskStatus with default tracking.
func NewTaskStatusWithDefaults(operation TaskOperation) TaskStatus {
	return NewTaskStatus(operation, nil, "", 0)
}

// NewTaskStatusFull creates a TaskStatus with all fields (used by repository).
func NewTaskStatusFull(
	id string,
	state domain.ReportingState,
	operation TaskOperation,
	message string,
	createdAt, updatedAt time.Time,
	total, current int,
	errorMessage string,
	parent *TaskStatus,
	trackableID int64,
	trackableType domain.TrackableType,
) TaskStatus {
	return TaskStatus{
		id:            id,
		state:         state,
		operation:     operation,
		message:       message,
		createdAt:     createdAt,
		updatedAt:     updatedAt,
		total:         total,
		current:       current,
		errorMessage:  errorMessage,
		parent:        parent,
		trackableID:   trackableID,
		trackableType: trackableType,
	}
}

// ID returns the status ID.
func (s TaskStatus) ID() string { return s.id }

// State returns the current state.
func (s TaskStatus) State() domain.ReportingState { return s.state }

// Operation returns the task operation.
func (s TaskStatus) Operation() TaskOperation { return s.operation }

// Message returns the status message.
func (s TaskStatus) Message() string { return s.message }

// CreatedAt returns when the status was created.
func (s TaskStatus) CreatedAt() time.Time { return s.createdAt }

// UpdatedAt returns when the status was last updated.
func (s TaskStatus) UpdatedAt() time.Time { return s.updatedAt }

// Total returns the total count for progress tracking.
func (s TaskStatus) Total() int { return s.total }

// Current returns the current count for progress tracking.
func (s TaskStatus) Current() int { return s.current }

// Error returns the error message if failed.
func (s TaskStatus) Error() string { return s.errorMessage }

// Parent returns the parent status.
func (s TaskStatus) Parent() *TaskStatus { return s.parent }

// TrackableID returns the trackable entity ID.
func (s TaskStatus) TrackableID() int64 { return s.trackableID }

// TrackableType returns the trackable entity type.
func (s TaskStatus) TrackableType() domain.TrackableType { return s.trackableType }

// CompletionPercent calculates the completion percentage.
func (s TaskStatus) CompletionPercent() float64 {
	if s.total == 0 {
		return 0.0
	}
	percent := float64(s.current) / float64(s.total) * 100.0
	if percent < 0 {
		return 0.0
	}
	if percent > 100 {
		return 100.0
	}
	return percent
}

// Skip marks the task as skipped with the given message.
func (s TaskStatus) Skip(message string) TaskStatus {
	s.state = domain.ReportingStateSkipped
	s.message = message
	s.updatedAt = time.Now().UTC()
	return s
}

// Fail marks the task as failed with the given error message.
func (s TaskStatus) Fail(errorMsg string) TaskStatus {
	s.state = domain.ReportingStateFailed
	s.errorMessage = errorMsg
	s.updatedAt = time.Now().UTC()
	return s
}

// SetTotal sets the total count for progress tracking.
func (s TaskStatus) SetTotal(total int) TaskStatus {
	s.total = total
	s.updatedAt = time.Now().UTC()
	return s
}

// SetCurrent sets the current progress and optionally updates the message.
func (s TaskStatus) SetCurrent(current int, message string) TaskStatus {
	s.state = domain.ReportingStateInProgress
	s.current = current
	if message != "" {
		s.message = message
	}
	s.updatedAt = time.Now().UTC()
	return s
}

// SetTrackingInfo sets the tracking information.
func (s TaskStatus) SetTrackingInfo(trackableID int64, trackableType domain.TrackableType) TaskStatus {
	s.trackableID = trackableID
	s.trackableType = trackableType
	s.updatedAt = time.Now().UTC()
	return s
}

// Complete marks the task as completed.
// If already in a terminal state, no change is made.
func (s TaskStatus) Complete() TaskStatus {
	if s.state.IsTerminal() {
		return s
	}
	s.state = domain.ReportingStateCompleted
	s.current = s.total // Ensure progress shows 100%
	s.updatedAt = time.Now().UTC()
	return s
}

// createStatusID creates a unique ID for a task status.
// Format: "{trackable_type}-{trackable_id}-{operation}" or just "{operation}"
func createStatusID(operation TaskOperation, trackableType domain.TrackableType, trackableID int64) string {
	var parts []string
	if trackableType != "" {
		parts = append(parts, string(trackableType))
	}
	if trackableID != 0 {
		parts = append(parts, fmt.Sprintf("%d", trackableID))
	}
	parts = append(parts, string(operation))

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "-"
		}
		result += part
	}
	return result
}
