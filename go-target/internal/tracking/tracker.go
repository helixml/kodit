package tracking

import (
	"context"
	"log/slog"
	"sync"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/queue"
)

// Tracker provides progress tracking with automatic notification to subscribers.
// It wraps TaskStatus and propagates state changes to registered reporters.
type Tracker struct {
	status      queue.TaskStatus
	subscribers []Reporter
	logger      *slog.Logger
	mu          sync.RWMutex
}

// NewTracker creates a new progress tracker wrapping the given TaskStatus.
func NewTracker(status queue.TaskStatus, logger *slog.Logger) *Tracker {
	return &Tracker{
		status:      status,
		subscribers: make([]Reporter, 0),
		logger:      logger,
	}
}

// TrackerForOperation creates a new Tracker for the given operation.
func TrackerForOperation(
	operation queue.TaskOperation,
	logger *slog.Logger,
	trackableType domain.TrackableType,
	trackableID int64,
) *Tracker {
	status := queue.NewTaskStatus(operation, nil, trackableType, trackableID)
	return NewTracker(status, logger)
}

// Status returns a copy of the current TaskStatus.
func (t *Tracker) Status() queue.TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// Subscribe adds a reporter to receive status change notifications.
func (t *Tracker) Subscribe(reporter Reporter) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.subscribers = append(t.subscribers, reporter)
}

// SetTotal sets the total count for progress tracking.
func (t *Tracker) SetTotal(ctx context.Context, total int) error {
	t.mu.Lock()
	t.status = t.status.SetTotal(total)
	status := t.status
	t.mu.Unlock()

	return t.notifySubscribers(ctx, status)
}

// SetCurrent updates the current progress count and optionally a message.
func (t *Tracker) SetCurrent(ctx context.Context, current int, message string) error {
	t.mu.Lock()
	t.status = t.status.SetCurrent(current, message)
	status := t.status
	t.mu.Unlock()

	return t.notifySubscribers(ctx, status)
}

// Skip marks the task as skipped with a reason.
func (t *Tracker) Skip(ctx context.Context, reason string) error {
	t.mu.Lock()
	t.status = t.status.Skip(reason)
	status := t.status
	t.mu.Unlock()

	return t.notifySubscribers(ctx, status)
}

// Fail marks the task as failed with an error message.
func (t *Tracker) Fail(ctx context.Context, errMsg string) error {
	t.mu.Lock()
	t.status = t.status.Fail(errMsg)
	status := t.status
	t.mu.Unlock()

	return t.notifySubscribers(ctx, status)
}

// Complete marks the task as completed.
func (t *Tracker) Complete(ctx context.Context) error {
	t.mu.Lock()
	t.status = t.status.Complete()
	status := t.status
	t.mu.Unlock()

	return t.notifySubscribers(ctx, status)
}

// Child creates a child tracker for a sub-operation.
// The child inherits the parent's subscribers and trackable info.
func (t *Tracker) Child(operation queue.TaskOperation) *Tracker {
	t.mu.RLock()
	parentStatus := t.status
	subscribers := make([]Reporter, len(t.subscribers))
	copy(subscribers, t.subscribers)
	t.mu.RUnlock()

	childStatus := queue.NewTaskStatus(
		operation,
		&parentStatus,
		parentStatus.TrackableType(),
		parentStatus.TrackableID(),
	)

	child := &Tracker{
		status:      childStatus,
		subscribers: subscribers,
		logger:      t.logger,
	}

	return child
}

// notifySubscribers sends the status update to all registered reporters.
func (t *Tracker) notifySubscribers(ctx context.Context, status queue.TaskStatus) error {
	t.mu.RLock()
	subscribers := make([]Reporter, len(t.subscribers))
	copy(subscribers, t.subscribers)
	t.mu.RUnlock()

	for _, subscriber := range subscribers {
		if err := subscriber.OnChange(ctx, status); err != nil {
			t.logger.Error("failed to notify subscriber",
				slog.String("error", err.Error()),
				slog.String("operation", status.Operation().String()),
			)
			// Continue notifying other subscribers even if one fails
		}
	}
	return nil
}

// Notify explicitly notifies all subscribers of the current status.
// Use this after initial setup to announce the tracker's existence.
func (t *Tracker) Notify(ctx context.Context) error {
	t.mu.RLock()
	status := t.status
	t.mu.RUnlock()

	return t.notifySubscribers(ctx, status)
}
