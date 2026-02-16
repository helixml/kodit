package tracking

import (
	"context"
	"log/slog"
	"sync"

	"github.com/helixml/kodit/domain/task"
)

// Tracker provides progress tracking with automatic notification to subscribers.
// It wraps Status and propagates state changes to registered reporters.
type Tracker struct {
	status      task.Status
	subscribers []Reporter
	logger      *slog.Logger
	mu          sync.RWMutex
}

// NewTracker creates a new progress tracker wrapping the given Status.
func NewTracker(status task.Status, logger *slog.Logger) *Tracker {
	return &Tracker{
		status:      status,
		subscribers: make([]Reporter, 0),
		logger:      logger,
	}
}

// TrackerForOperation creates a new Tracker for the given operation.
func TrackerForOperation(
	operation task.Operation,
	logger *slog.Logger,
	trackableType task.TrackableType,
	trackableID int64,
) *Tracker {
	status := task.NewStatus(operation, nil, trackableType, trackableID)
	return NewTracker(status, logger)
}

// Status returns a copy of the current Status.
func (t *Tracker) Status() task.Status {
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
func (t *Tracker) SetTotal(ctx context.Context, total int) {
	t.mu.Lock()
	t.status = t.status.SetTotal(total)
	status := t.status
	t.mu.Unlock()

	t.notifySubscribers(ctx, status)
}

// SetCurrent updates the current progress count and optionally a message.
func (t *Tracker) SetCurrent(ctx context.Context, current int, message string) {
	t.mu.Lock()
	t.status = t.status.SetCurrent(current, message)
	status := t.status
	t.mu.Unlock()

	t.notifySubscribers(ctx, status)
}

// Skip marks the task as skipped with a reason.
func (t *Tracker) Skip(ctx context.Context, reason string) {
	t.mu.Lock()
	t.status = t.status.Skip(reason)
	status := t.status
	t.mu.Unlock()

	t.notifySubscribers(ctx, status)
}

// Fail marks the task as failed with an error message.
func (t *Tracker) Fail(ctx context.Context, errMsg string) {
	t.mu.Lock()
	t.status = t.status.Fail(errMsg)
	status := t.status
	t.mu.Unlock()

	t.notifySubscribers(ctx, status)
}

// Complete marks the task as completed.
func (t *Tracker) Complete(ctx context.Context) {
	t.mu.Lock()
	t.status = t.status.Complete()
	status := t.status
	t.mu.Unlock()

	t.notifySubscribers(ctx, status)
}

// Child creates a child tracker for a sub-operation.
// The child inherits the parent's subscribers and trackable info.
func (t *Tracker) Child(operation task.Operation) *Tracker {
	t.mu.RLock()
	parentStatus := t.status
	subscribers := make([]Reporter, len(t.subscribers))
	copy(subscribers, t.subscribers)
	t.mu.RUnlock()

	childStatus := task.NewStatus(
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
func (t *Tracker) notifySubscribers(ctx context.Context, status task.Status) {
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
}

// Notify explicitly notifies all subscribers of the current status.
// Use this after initial setup to announce the tracker's existence.
func (t *Tracker) Notify(ctx context.Context) {
	t.mu.RLock()
	status := t.status
	t.mu.RUnlock()

	t.notifySubscribers(ctx, status)
}
