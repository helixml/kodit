package task

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// TaskStore defines the interface for Task persistence operations.
type TaskStore interface {
	// Get retrieves a task by ID.
	Get(ctx context.Context, id int64) (Task, error)

	// FindAll retrieves all tasks.
	FindAll(ctx context.Context) ([]Task, error)

	// FindPending retrieves pending tasks ordered by priority.
	FindPending(ctx context.Context, options ...repository.Option) ([]Task, error)

	// Save creates a new task or updates an existing one.
	// Uses dedup_key for conflict resolution - if a task with the same
	// dedup_key exists, it will be returned instead of creating a duplicate.
	Save(ctx context.Context, task Task) (Task, error)

	// SaveBulk creates or updates multiple tasks.
	SaveBulk(ctx context.Context, tasks []Task) ([]Task, error)

	// Delete removes a task.
	Delete(ctx context.Context, task Task) error

	// DeleteAll removes all tasks.
	DeleteAll(ctx context.Context) error

	// CountPending returns the number of pending tasks.
	CountPending(ctx context.Context, options ...repository.Option) (int64, error)

	// Exists checks if a task with the given ID exists.
	Exists(ctx context.Context, id int64) (bool, error)

	// Dequeue retrieves and removes the highest priority task.
	// Returns the task and true if one was found, or zero-value and false if queue is empty.
	Dequeue(ctx context.Context) (Task, bool, error)

	// DequeueByOperation retrieves and removes the highest priority task
	// of a specific operation type.
	DequeueByOperation(ctx context.Context, operation Operation) (Task, bool, error)
}

// StatusStore defines the interface for Status persistence operations.
type StatusStore interface {
	// Get retrieves a task status by ID.
	Get(ctx context.Context, id string) (Status, error)

	// FindByTrackable retrieves task statuses for a trackable entity.
	FindByTrackable(ctx context.Context, trackableType TrackableType, trackableID int64) ([]Status, error)

	// Save creates a new task status or updates an existing one.
	// If the status has a parent, the parent chain is saved first.
	Save(ctx context.Context, status Status) (Status, error)

	// SaveBulk creates or updates multiple task statuses.
	SaveBulk(ctx context.Context, statuses []Status) ([]Status, error)

	// Delete removes a task status.
	Delete(ctx context.Context, status Status) error

	// DeleteByTrackable removes task statuses for a trackable entity.
	DeleteByTrackable(ctx context.Context, trackableType TrackableType, trackableID int64) error

	// Count returns the total number of task statuses.
	Count(ctx context.Context) (int64, error)

	// LoadWithHierarchy loads all task statuses for a trackable entity
	// with their parent-child relationships reconstructed.
	LoadWithHierarchy(
		ctx context.Context,
		trackableType TrackableType,
		trackableID int64,
	) ([]Status, error)
}
