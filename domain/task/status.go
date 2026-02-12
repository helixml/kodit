package task

import (
	"fmt"
	"time"
)

// ReportingState represents the state of task reporting.
type ReportingState string

// ReportingState values.
const (
	ReportingStateStarted    ReportingState = "started"
	ReportingStateInProgress ReportingState = "in_progress"
	ReportingStateCompleted  ReportingState = "completed"
	ReportingStateFailed     ReportingState = "failed"
	ReportingStateSkipped    ReportingState = "skipped"
)

// IsTerminal returns true if the state represents a terminal (final) state.
func (s ReportingState) IsTerminal() bool {
	return s == ReportingStateCompleted ||
		s == ReportingStateFailed ||
		s == ReportingStateSkipped
}

// TrackableType represents types of trackable entities.
type TrackableType string

// TrackableType values.
const (
	TrackableTypeIndex      TrackableType = "indexes"
	TrackableTypeRepository TrackableType = "kodit.repository"
	TrackableTypeCommit     TrackableType = "kodit.commit"
)

// Status represents the status of a task with progress tracking.
type Status struct {
	id            string
	state         ReportingState
	operation     Operation
	message       string
	createdAt     time.Time
	updatedAt     time.Time
	total         int
	current       int
	errorMessage  string
	parent        *Status
	trackableID   int64
	trackableType TrackableType
}

// NewStatus creates a new Status for the given operation.
func NewStatus(
	operation Operation,
	parent *Status,
	trackableType TrackableType,
	trackableID int64,
) Status {
	now := time.Now().UTC()
	return Status{
		id:            createStatusID(operation, trackableType, trackableID),
		operation:     operation,
		parent:        parent,
		trackableType: trackableType,
		trackableID:   trackableID,
		state:         ReportingStateStarted,
		createdAt:     now,
		updatedAt:     now,
	}
}

// NewStatusWithDefaults creates a Status with default tracking.
func NewStatusWithDefaults(operation Operation) Status {
	return NewStatus(operation, nil, "", 0)
}

// NewStatusFull creates a Status with all fields (used by repository).
func NewStatusFull(
	id string,
	state ReportingState,
	operation Operation,
	message string,
	createdAt, updatedAt time.Time,
	total, current int,
	errorMessage string,
	parent *Status,
	trackableID int64,
	trackableType TrackableType,
) Status {
	return Status{
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
func (s Status) ID() string { return s.id }

// State returns the current state.
func (s Status) State() ReportingState { return s.state }

// Operation returns the task operation.
func (s Status) Operation() Operation { return s.operation }

// Message returns the status message.
func (s Status) Message() string { return s.message }

// CreatedAt returns when the status was created.
func (s Status) CreatedAt() time.Time { return s.createdAt }

// UpdatedAt returns when the status was last updated.
func (s Status) UpdatedAt() time.Time { return s.updatedAt }

// Total returns the total count for progress tracking.
func (s Status) Total() int { return s.total }

// Current returns the current count for progress tracking.
func (s Status) Current() int { return s.current }

// Error returns the error message if failed.
func (s Status) Error() string { return s.errorMessage }

// Parent returns the parent status.
func (s Status) Parent() *Status { return s.parent }

// TrackableID returns the trackable entity ID.
func (s Status) TrackableID() int64 { return s.trackableID }

// TrackableType returns the trackable entity type.
func (s Status) TrackableType() TrackableType { return s.trackableType }

// CompletionPercent calculates the completion percentage.
func (s Status) CompletionPercent() float64 {
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
func (s Status) Skip(message string) Status {
	s.state = ReportingStateSkipped
	s.message = message
	s.updatedAt = time.Now().UTC()
	return s
}

// Fail marks the task as failed with the given error message.
func (s Status) Fail(errorMsg string) Status {
	s.state = ReportingStateFailed
	s.errorMessage = errorMsg
	s.updatedAt = time.Now().UTC()
	return s
}

// SetTotal sets the total count for progress tracking.
func (s Status) SetTotal(total int) Status {
	s.total = total
	s.updatedAt = time.Now().UTC()
	return s
}

// SetCurrent sets the current progress and optionally updates the message.
func (s Status) SetCurrent(current int, message string) Status {
	s.state = ReportingStateInProgress
	s.current = current
	if message != "" {
		s.message = message
	}
	s.updatedAt = time.Now().UTC()
	return s
}

// SetTrackingInfo sets the tracking information.
func (s Status) SetTrackingInfo(trackableID int64, trackableType TrackableType) Status {
	s.trackableID = trackableID
	s.trackableType = trackableType
	s.updatedAt = time.Now().UTC()
	return s
}

// Complete marks the task as completed.
// If already in a terminal state, no change is made.
func (s Status) Complete() Status {
	if s.state.IsTerminal() {
		return s
	}
	s.state = ReportingStateCompleted
	s.current = s.total // Ensure progress shows 100%
	s.updatedAt = time.Now().UTC()
	return s
}

// createStatusID creates a unique ID for a task status.
// Format: "{trackable_type}-{trackable_id}-{operation}" or just "{operation}"
func createStatusID(operation Operation, trackableType TrackableType, trackableID int64) string {
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
