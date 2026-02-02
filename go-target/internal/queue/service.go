package queue

import (
	"context"
	"log/slog"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/domain"
)

// Service provides the main interface for enqueuing and managing tasks.
type Service struct {
	repo   TaskRepository
	logger *slog.Logger
}

// NewService creates a new queue service.
func NewService(repo TaskRepository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// Enqueue adds a task to the queue.
// If a task with the same dedup_key exists, it updates the priority instead.
func (s *Service) Enqueue(ctx context.Context, task Task) error {
	_, err := s.repo.Save(ctx, task)
	if err != nil {
		return err
	}

	s.logger.Info("task enqueued",
		slog.String("dedup_key", task.DedupKey()),
		slog.String("operation", task.Operation().String()),
	)
	return nil
}

// EnqueueOperations queues multiple operations with decreasing priority.
// The first operation in the list has the highest priority, ensuring
// operations are processed in order.
func (s *Service) EnqueueOperations(
	ctx context.Context,
	operations []TaskOperation,
	basePriority domain.QueuePriority,
	payload map[string]any,
) error {
	// Calculate priority offsets so first operation has highest priority
	priorityOffset := len(operations) * 10
	for _, op := range operations {
		task := NewTask(op, int(basePriority)+priorityOffset, payload)
		if err := s.Enqueue(ctx, task); err != nil {
			return err
		}
		priorityOffset -= 10
	}
	return nil
}

// List returns all tasks in the queue, optionally filtered by operation.
// Tasks are sorted by priority (highest first) then by created_at (oldest first).
func (s *Service) List(ctx context.Context, operation *TaskOperation) ([]Task, error) {
	query := database.NewQuery().
		OrderDesc("priority").
		OrderDesc("created_at")

	if operation != nil {
		query = query.Equal("type", operation.String())
	}

	return s.repo.Find(ctx, query)
}

// TaskByDedupKey returns a task by its dedup_key.
func (s *Service) TaskByDedupKey(ctx context.Context, dedupKey string) (Task, bool, error) {
	query := database.NewQuery().Equal("dedup_key", dedupKey)
	tasks, err := s.repo.Find(ctx, query)
	if err != nil {
		return Task{}, false, err
	}
	if len(tasks) == 0 {
		return Task{}, false, nil
	}
	return tasks[0], true, nil
}

// PendingCount returns the count of pending tasks.
func (s *Service) PendingCount(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx, database.NewQuery())
}

// PendingCountByOperation returns the count of pending tasks for an operation.
func (s *Service) PendingCountByOperation(ctx context.Context, operation TaskOperation) (int64, error) {
	query := database.NewQuery().Equal("type", operation.String())
	return s.repo.Count(ctx, query)
}
