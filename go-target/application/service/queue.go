package service

import (
	"context"
	"log/slog"

	"github.com/helixml/kodit/domain/task"
)

// TaskListParams configures task listing.
type TaskListParams struct {
	Operation *task.Operation
	Limit     int
}

// Queue provides the main interface for enqueuing and managing tasks.
type Queue struct {
	store  task.TaskStore
	logger *slog.Logger
}

// NewQueue creates a new queue service.
func NewQueue(store task.TaskStore, logger *slog.Logger) *Queue {
	return &Queue{
		store:  store,
		logger: logger,
	}
}

// Enqueue adds a task to the queue.
// If a task with the same dedup_key exists, it updates the priority instead.
func (s *Queue) Enqueue(ctx context.Context, t task.Task) error {
	_, err := s.store.Save(ctx, t)
	if err != nil {
		return err
	}

	s.logger.Info("task enqueued",
		slog.String("dedup_key", t.DedupKey()),
		slog.String("operation", t.Operation().String()),
	)
	return nil
}

// EnqueueOperations queues multiple operations with decreasing priority.
// The first operation in the list has the highest priority, ensuring
// operations are processed in order.
func (s *Queue) EnqueueOperations(
	ctx context.Context,
	operations []task.Operation,
	basePriority task.Priority,
	payload map[string]any,
) error {
	// Calculate priority offsets so first operation has highest priority
	priorityOffset := len(operations) * 10
	for _, op := range operations {
		t := task.NewTask(op, int(basePriority)+priorityOffset, payload)
		if err := s.Enqueue(ctx, t); err != nil {
			return err
		}
		priorityOffset -= 10
	}
	return nil
}

// List returns tasks matching the given params.
// Tasks are sorted by priority (highest first) then by created_at (oldest first).
func (s *Queue) List(ctx context.Context, params *TaskListParams) ([]task.Task, error) {
	tasks, err := s.store.FindPending(ctx)
	if err != nil {
		return nil, err
	}

	if params == nil {
		return tasks, nil
	}

	if params.Operation != nil {
		filtered := make([]task.Task, 0, len(tasks))
		for _, t := range tasks {
			if t.Operation() == *params.Operation {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	if params.Limit > 0 && len(tasks) > params.Limit {
		tasks = tasks[:params.Limit]
	}

	return tasks, nil
}

// Get retrieves a task by ID.
func (s *Queue) Get(ctx context.Context, id int64) (task.Task, error) {
	return s.store.Get(ctx, id)
}
