package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
)

// TaskListParams configures task listing.
type TaskListParams struct {
	Operation *task.Operation
	Limit     int
	Offset    int
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

	s.logger.Debug("task enqueued",
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
	var options []repository.Option

	if params != nil && params.Limit > 0 {
		options = append(options, repository.WithPagination(params.Limit, params.Offset)...)
	}

	tasks, err := s.store.FindPending(ctx, options...)
	if err != nil {
		return nil, err
	}

	if params != nil && params.Operation != nil {
		filtered := make([]task.Task, 0, len(tasks))
		for _, t := range tasks {
			if t.Operation() == *params.Operation {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	return tasks, nil
}

// Count returns the total number of pending tasks.
func (s *Queue) Count(ctx context.Context) (int64, error) {
	return s.store.CountPending(ctx)
}

// Get retrieves a task by ID.
func (s *Queue) Get(ctx context.Context, id int64) (task.Task, error) {
	return s.store.Get(ctx, id)
}

// DrainForRepository removes all pending tasks whose payload contains
// the given repository_id. This prevents stale enrichment/indexing tasks
// from blocking a repository deletion.
func (s *Queue) DrainForRepository(ctx context.Context, repoID int64) (int, error) {
	tasks, err := s.store.FindAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("find pending tasks: %w", err)
	}

	removed := 0
	for _, t := range tasks {
		payload := t.Payload()
		if payloadRepoID(payload) != repoID {
			continue
		}
		if err := s.store.Delete(ctx, t); err != nil {
			return removed, fmt.Errorf("delete task %d: %w", t.ID(), err)
		}
		removed++
	}
	return removed, nil
}

func payloadRepoID(payload map[string]any) int64 {
	val, ok := payload["repository_id"]
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}
