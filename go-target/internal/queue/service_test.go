package queue

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/helixml/kodit/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Enqueue(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	task := NewTask(OperationCloneRepository, 100, map[string]any{"repo_id": int64(1)})

	err := service.Enqueue(ctx, task)
	require.NoError(t, err)

	// Verify task was saved
	tasks := repo.All()
	assert.Len(t, tasks, 1)
	assert.Equal(t, OperationCloneRepository, tasks[0].Operation())
	assert.Equal(t, 100, tasks[0].Priority())
}

func TestService_Enqueue_UpdatesPriorityForDuplicateKey(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	payload := map[string]any{"repo_id": int64(1)}

	// Enqueue first task
	task1 := NewTask(OperationCloneRepository, 100, payload)
	err := service.Enqueue(ctx, task1)
	require.NoError(t, err)

	// Enqueue same task with higher priority
	task2 := NewTask(OperationCloneRepository, 200, payload)
	err = service.Enqueue(ctx, task2)
	require.NoError(t, err)

	// Should still have only one task, but with updated priority
	tasks := repo.All()
	assert.Len(t, tasks, 1)
	assert.Equal(t, 200, tasks[0].Priority())
}

func TestService_EnqueueOperations(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	operations := []TaskOperation{
		OperationScanCommit,
		OperationExtractSnippetsForCommit,
		OperationCreateBM25IndexForCommit,
	}
	payload := map[string]any{"commit_sha": "abc123"}

	err := service.EnqueueOperations(ctx, operations, domain.QueuePriorityNormal, payload)
	require.NoError(t, err)

	// Should have 3 tasks
	tasks := repo.All()
	assert.Len(t, tasks, 3)

	// Find tasks by operation
	tasksByOp := make(map[TaskOperation]Task)
	for _, task := range tasks {
		tasksByOp[task.Operation()] = task
	}

	// First operation should have highest priority
	scanTask := tasksByOp[OperationScanCommit]
	extractTask := tasksByOp[OperationExtractSnippetsForCommit]
	bm25Task := tasksByOp[OperationCreateBM25IndexForCommit]

	assert.Greater(t, scanTask.Priority(), extractTask.Priority())
	assert.Greater(t, extractTask.Priority(), bm25Task.Priority())
}

func TestService_EnqueueOperations_PreservesPriorityOrder(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	// Use prescribed operations for a full workflow
	operations := PrescribedOperations{}.ScanAndIndexCommit()
	payload := map[string]any{"commit_sha": "abc123"}

	err := service.EnqueueOperations(ctx, operations, domain.QueuePriorityBackground, payload)
	require.NoError(t, err)

	tasks := repo.All()
	assert.Len(t, tasks, len(operations))

	// When dequeued, tasks should come out in order
	var lastPriority int
	for i := range len(operations) {
		task, found, err := repo.Dequeue(ctx)
		require.NoError(t, err)
		require.True(t, found)

		if i > 0 {
			assert.Less(t, task.Priority(), lastPriority, "task %d should have lower priority than previous", i)
		}
		lastPriority = task.Priority()
	}
}

func TestService_List(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	// Create tasks with different operations
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, map[string]any{"repo_id": int64(1)}))
	_, _ = repo.Save(ctx, NewTask(OperationSyncRepository, 200, map[string]any{"repo_id": int64(2)}))
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 150, map[string]any{"repo_id": int64(3)}))

	// List all tasks
	tasks, err := service.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, tasks, 3)

	// Tasks should be ordered by priority descending
	assert.Equal(t, 200, tasks[0].Priority())
	assert.Equal(t, 150, tasks[1].Priority())
	assert.Equal(t, 100, tasks[2].Priority())
}

func TestService_List_FiltersByOperation(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, map[string]any{"repo_id": int64(1)}))
	_, _ = repo.Save(ctx, NewTask(OperationSyncRepository, 200, map[string]any{"repo_id": int64(2)}))
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 150, map[string]any{"repo_id": int64(3)}))

	op := OperationCloneRepository
	tasks, err := service.List(ctx, &op)
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	for _, task := range tasks {
		assert.Equal(t, OperationCloneRepository, task.Operation())
	}
}

func TestService_TaskByDedupKey(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	payload := map[string]any{"repo_id": int64(42)}
	original := NewTask(OperationCloneRepository, 100, payload)
	saved, _ := repo.Save(ctx, original)

	// Find by dedup key
	task, found, err := service.TaskByDedupKey(ctx, saved.DedupKey())
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, saved.ID(), task.ID())
}

func TestService_TaskByDedupKey_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	_, found, err := service.TaskByDedupKey(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestService_PendingCount(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	// Initially empty
	count, err := service.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add some tasks
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, map[string]any{"repo_id": int64(1)}))
	_, _ = repo.Save(ctx, NewTask(OperationSyncRepository, 200, map[string]any{"repo_id": int64(2)}))

	count, err = service.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestService_PendingCountByOperation(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := NewService(repo, logger)

	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, map[string]any{"repo_id": int64(1)}))
	_, _ = repo.Save(ctx, NewTask(OperationSyncRepository, 200, map[string]any{"repo_id": int64(2)}))
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 150, map[string]any{"repo_id": int64(3)}))

	count, err := service.PendingCountByOperation(ctx, OperationCloneRepository)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = service.PendingCountByOperation(ctx, OperationSyncRepository)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
