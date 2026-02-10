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

// List returns all tasks in the queue, optionally filtered by operation.
// Tasks are sorted by priority (highest first) then by created_at (oldest first).
func (s *Queue) List(ctx context.Context, operation *task.Operation) ([]task.Task, error) {
	tasks, err := s.store.FindPending(ctx)
	if err != nil {
		return nil, err
	}

	if operation == nil {
		return tasks, nil
	}

	// Filter by operation if specified
	filtered := make([]task.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Operation() == *operation {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}

// TaskByDedupKey returns a task by its dedup_key.
func (s *Queue) TaskByDedupKey(ctx context.Context, dedupKey string) (task.Task, bool, error) {
	tasks, err := s.store.FindPending(ctx)
	if err != nil {
		return task.Task{}, false, err
	}

	for _, t := range tasks {
		if t.DedupKey() == dedupKey {
			return t, true, nil
		}
	}
	return task.Task{}, false, nil
}

// Get retrieves a task by ID.
func (s *Queue) Get(ctx context.Context, id int64) (task.Task, error) {
	return s.store.Get(ctx, id)
}

// Cancel removes a pending task by ID.
func (s *Queue) Cancel(ctx context.Context, id int64) error {
	tsk, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.store.Delete(ctx, tsk)
}

// ListFiltered returns tasks matching the given filter.
func (s *Queue) ListFiltered(ctx context.Context, filter task.Filter) ([]task.Task, error) {
	tasks, err := s.List(ctx, filter.Operation())
	if err != nil {
		return nil, err
	}
	if filter.Limit() > 0 && len(tasks) > filter.Limit() {
		tasks = tasks[:filter.Limit()]
	}
	return tasks, nil
}

// ListByParams returns tasks matching the given params.
func (s *Queue) ListByParams(ctx context.Context, params *TaskListParams) ([]task.Task, error) {
	filter := task.NewFilter()
	if params != nil {
		if params.Operation != nil {
			filter = filter.WithOperation(*params.Operation)
		}
		if params.Limit > 0 {
			filter = filter.WithLimit(params.Limit)
		}
	}
	return s.ListFiltered(ctx, filter)
}

// PendingCount returns the count of pending tasks.
func (s *Queue) PendingCount(ctx context.Context) (int64, error) {
	return s.store.CountPending(ctx)
}

// PendingCountByOperation returns the count of pending tasks for an operation.
func (s *Queue) PendingCountByOperation(ctx context.Context, operation task.Operation) (int64, error) {
	tasks, err := s.store.FindPending(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	for _, t := range tasks {
		if t.Operation() == operation {
			count++
		}
	}
	return count, nil
}
