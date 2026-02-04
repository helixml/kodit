package service

import (
	"context"
	"log/slog"

	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/internal/database"
)

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
	query := database.NewQuery().
		OrderDesc("priority").
		OrderDesc("created_at")

	if operation != nil {
		query = query.Equal("type", operation.String())
	}

	return s.store.Find(ctx, query)
}

// TaskByDedupKey returns a task by its dedup_key.
func (s *Queue) TaskByDedupKey(ctx context.Context, dedupKey string) (task.Task, bool, error) {
	query := database.NewQuery().Equal("dedup_key", dedupKey)
	tasks, err := s.store.Find(ctx, query)
	if err != nil {
		return task.Task{}, false, err
	}
	if len(tasks) == 0 {
		return task.Task{}, false, nil
	}
	return tasks[0], true, nil
}

// PendingCount returns the count of pending tasks.
func (s *Queue) PendingCount(ctx context.Context) (int64, error) {
	return s.store.Count(ctx, database.NewQuery())
}

// PendingCountByOperation returns the count of pending tasks for an operation.
func (s *Queue) PendingCountByOperation(ctx context.Context, operation task.Operation) (int64, error) {
	query := database.NewQuery().Equal("type", operation.String())
	return s.store.Count(ctx, query)
}
