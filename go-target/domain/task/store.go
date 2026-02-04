package task

import (
	"context"

	"github.com/helixml/kodit/internal/database"
)

// TaskStore defines the interface for Task persistence operations.
type TaskStore interface {
	// Get retrieves a task by ID.
	Get(ctx context.Context, id int64) (Task, error)

	// Find retrieves tasks matching a query.
	Find(ctx context.Context, query database.Query) ([]Task, error)

	// Save creates a new task or updates an existing one.
	// Uses dedup_key for conflict resolution - if a task with the same
	// dedup_key exists, it will be returned instead of creating a duplicate.
	Save(ctx context.Context, task Task) (Task, error)

	// SaveBulk creates or updates multiple tasks.
	SaveBulk(ctx context.Context, tasks []Task) ([]Task, error)

	// Delete removes a task.
	Delete(ctx context.Context, task Task) error

	// DeleteByQuery removes tasks matching a query.
	DeleteByQuery(ctx context.Context, query database.Query) error

	// Count returns the number of tasks matching a query.
	Count(ctx context.Context, query database.Query) (int64, error)

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

	// Find retrieves task statuses matching a query.
	Find(ctx context.Context, query database.Query) ([]Status, error)

	// Save creates a new task status or updates an existing one.
	// If the status has a parent, the parent chain is saved first.
	Save(ctx context.Context, status Status) (Status, error)

	// SaveBulk creates or updates multiple task statuses.
	SaveBulk(ctx context.Context, statuses []Status) ([]Status, error)

	// Delete removes a task status.
	Delete(ctx context.Context, status Status) error

	// DeleteByQuery removes task statuses matching a query.
	DeleteByQuery(ctx context.Context, query database.Query) error

	// Count returns the number of task statuses matching a query.
	Count(ctx context.Context, query database.Query) (int64, error)

	// LoadWithHierarchy loads all task statuses for a trackable entity
	// with their parent-child relationships reconstructed.
	LoadWithHierarchy(
		ctx context.Context,
		trackableType TrackableType,
		trackableID int64,
	) ([]Status, error)
}
