package task

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// TaskStore defines the interface for Task persistence operations.
type TaskStore interface {
	repository.Store[Task]

	// Exists checks if any task matches the given options.
	Exists(ctx context.Context, options ...repository.Option) (bool, error)

	// DeleteBy removes all tasks matching the given options.
	DeleteBy(ctx context.Context, options ...repository.Option) error

	// Dequeue retrieves and removes the highest priority task.
	// Returns the task and true if one was found, or zero-value and false if queue is empty.
	Dequeue(ctx context.Context) (Task, bool, error)

	// DequeueByOperation retrieves and removes the highest priority task
	// of a specific operation type.
	DequeueByOperation(ctx context.Context, operation Operation) (Task, bool, error)
}

// StatusStore defines the interface for Status persistence operations.
type StatusStore interface {
	Find(ctx context.Context, options ...repository.Option) ([]Status, error)
	FindOne(ctx context.Context, options ...repository.Option) (Status, error)
	Count(ctx context.Context, options ...repository.Option) (int64, error)
	Exists(ctx context.Context, options ...repository.Option) (bool, error)
	Save(ctx context.Context, status Status) (Status, error)
	DeleteBy(ctx context.Context, options ...repository.Option) error

	// LoadWithHierarchy loads all task statuses for a trackable entity
	// with their parent-child relationships reconstructed.
	LoadWithHierarchy(
		ctx context.Context,
		trackableType TrackableType,
		trackableID int64,
	) ([]Status, error)
}
