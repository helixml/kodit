package queue_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/queue"
	queuepostgres "github.com/helixml/kodit/internal/queue/postgres"
	"github.com/helixml/kodit/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for queue.Service with real database.
// These tests mirror the Python tests in tests/kodit/application/services/queue_service_test.py.

func TestService_Enqueue_NewTask_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	task := queue.NewTask(
		queue.OperationRefreshWorkingCopy,
		int(domain.QueuePriorityUserInitiated),
		map[string]any{"repository_id": int64(1)},
	)

	err := service.Enqueue(ctx, task)
	require.NoError(t, err)

	// Verify task was persisted
	tasks, err := service.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	assert.Equal(t, task.DedupKey(), tasks[0].DedupKey())
	assert.Equal(t, queue.OperationRefreshWorkingCopy, tasks[0].Operation())
	assert.Equal(t, int(domain.QueuePriorityUserInitiated), tasks[0].Priority())
}

func TestService_Enqueue_ExistingTaskUpdatesPriority_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	payload := map[string]any{"repository_id": int64(1)}

	// Enqueue first task with normal priority
	task1 := queue.NewTask(queue.OperationCloneRepository, int(domain.QueuePriorityNormal), payload)
	err := service.Enqueue(ctx, task1)
	require.NoError(t, err)

	// Enqueue same task (same dedup key) with higher priority
	task2 := queue.NewTask(queue.OperationCloneRepository, int(domain.QueuePriorityUserInitiated), payload)
	err = service.Enqueue(ctx, task2)
	require.NoError(t, err)

	// Should still have only one task, but with updated priority
	tasks, err := service.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	assert.Equal(t, int(domain.QueuePriorityUserInitiated), tasks[0].Priority())
}

func TestService_Enqueue_MultipleTasks_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	// Enqueue multiple different tasks
	for i := 1; i <= 5; i++ {
		task := queue.NewTask(
			queue.OperationCloneRepository,
			int(domain.QueuePriorityNormal),
			map[string]any{"repository_id": int64(i)},
		)
		err := service.Enqueue(ctx, task)
		require.NoError(t, err)
	}

	tasks, err := service.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, tasks, 5)
}

func TestService_List_FiltersByType_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	// Create tasks of different types
	cloneTask := queue.NewTask(queue.OperationCloneRepository, int(domain.QueuePriorityNormal), map[string]any{"repository_id": int64(1)})
	syncTask := queue.NewTask(queue.OperationSyncRepository, int(domain.QueuePriorityNormal), map[string]any{"repository_id": int64(2)})
	scanTask := queue.NewTask(queue.OperationScanCommit, int(domain.QueuePriorityNormal), map[string]any{"commit_sha": "abc123"})

	err := service.Enqueue(ctx, cloneTask)
	require.NoError(t, err)
	err = service.Enqueue(ctx, syncTask)
	require.NoError(t, err)
	err = service.Enqueue(ctx, scanTask)
	require.NoError(t, err)

	// Filter by clone operation
	cloneOp := queue.OperationCloneRepository
	cloneTasks, err := service.List(ctx, &cloneOp)
	require.NoError(t, err)
	assert.Len(t, cloneTasks, 1)
	assert.Equal(t, queue.OperationCloneRepository, cloneTasks[0].Operation())

	// Filter by sync operation
	syncOp := queue.OperationSyncRepository
	syncTasks, err := service.List(ctx, &syncOp)
	require.NoError(t, err)
	assert.Len(t, syncTasks, 1)
	assert.Equal(t, queue.OperationSyncRepository, syncTasks[0].Operation())
}

func TestService_List_EmptyQueue_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	tasks, err := service.List(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestService_TaskPriorityOrdering_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	// Create tasks with different priorities
	backgroundTask := queue.NewTask(queue.OperationSyncRepository, int(domain.QueuePriorityBackground), map[string]any{"repository_id": int64(1)})
	normalTask := queue.NewTask(queue.OperationSyncRepository, int(domain.QueuePriorityNormal), map[string]any{"repository_id": int64(2)})
	urgentTask := queue.NewTask(queue.OperationSyncRepository, int(domain.QueuePriorityUserInitiated), map[string]any{"repository_id": int64(3)})

	// Enqueue in random order
	err := service.Enqueue(ctx, normalTask)
	require.NoError(t, err)
	err = service.Enqueue(ctx, backgroundTask)
	require.NoError(t, err)
	err = service.Enqueue(ctx, urgentTask)
	require.NoError(t, err)

	// List should return in priority order (highest first)
	tasks, err := service.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tasks, 3)

	// Highest priority first
	assert.Equal(t, int(domain.QueuePriorityUserInitiated), tasks[0].Priority())
	assert.Equal(t, int(domain.QueuePriorityNormal), tasks[1].Priority())
	assert.Equal(t, int(domain.QueuePriorityBackground), tasks[2].Priority())
}

func TestService_UserInitiatedPreemptsBackgroundBatch_Integration(t *testing.T) {
	// This is a critical scenario: when a user triggers an operation,
	// it should preempt any background tasks that are already queued.
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	// First, queue a batch of background operations (simulating automated sync)
	backgroundOps := queue.PrescribedOperations{}.ScanAndIndexCommit()
	backgroundPayload := map[string]any{"commit_sha": "background123", "repository_id": int64(1)}
	err := service.EnqueueOperations(ctx, backgroundOps, domain.QueuePriorityBackground, backgroundPayload)
	require.NoError(t, err)

	// Now user initiates a new operation (should be processed first)
	userOps := queue.PrescribedOperations{}.ScanAndIndexCommit()
	userPayload := map[string]any{"commit_sha": "user456", "repository_id": int64(2)}
	err = service.EnqueueOperations(ctx, userOps, domain.QueuePriorityUserInitiated, userPayload)
	require.NoError(t, err)

	// Verify all tasks are queued
	tasks, err := service.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, tasks, len(backgroundOps)+len(userOps))

	// The first tasks in the list should be user-initiated (higher priority)
	for i := 0; i < len(userOps); i++ {
		assert.Greater(t, tasks[i].Priority(), int(domain.QueuePriorityNormal),
			"User-initiated task at position %d should have higher priority", i)
	}

	// Verify dequeue order respects priority
	dequeuedTask, found, err := taskRepo.Dequeue(ctx)
	require.NoError(t, err)
	require.True(t, found)

	// First dequeued should be user-initiated (highest priority in that batch)
	payload := dequeuedTask.Payload()
	commitSHA, ok := payload["commit_sha"].(string)
	require.True(t, ok)
	assert.Equal(t, "user456", commitSHA, "First dequeued task should be user-initiated")
}

func TestService_EnqueueOperations_PreservesPriorityOrder_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	// Use prescribed operations for a full workflow
	operations := queue.PrescribedOperations{}.ScanAndIndexCommit()
	payload := map[string]any{"commit_sha": "abc123", "repository_id": int64(1)}

	err := service.EnqueueOperations(ctx, operations, domain.QueuePriorityNormal, payload)
	require.NoError(t, err)

	tasks, err := service.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, tasks, len(operations))

	// Verify dequeue order matches operation sequence
	expectedOps := operations
	for i := range len(expectedOps) {
		task, found, err := taskRepo.Dequeue(ctx)
		require.NoError(t, err)
		require.True(t, found, "Should find task at position %d", i)

		assert.Equal(t, expectedOps[i], task.Operation(),
			"Task %d should be operation %s", i, expectedOps[i])
	}
}

func TestService_PendingCount_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	// Initially empty
	count, err := service.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add some tasks
	for i := 1; i <= 3; i++ {
		task := queue.NewTask(queue.OperationSyncRepository, int(domain.QueuePriorityNormal), map[string]any{"repository_id": int64(i)})
		err := service.Enqueue(ctx, task)
		require.NoError(t, err)
	}

	count, err = service.PendingCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestService_PendingCountByOperation_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	// Add tasks of different types
	err := service.Enqueue(ctx, queue.NewTask(queue.OperationCloneRepository, 100, map[string]any{"repository_id": int64(1)}))
	require.NoError(t, err)
	err = service.Enqueue(ctx, queue.NewTask(queue.OperationCloneRepository, 100, map[string]any{"repository_id": int64(2)}))
	require.NoError(t, err)
	err = service.Enqueue(ctx, queue.NewTask(queue.OperationSyncRepository, 100, map[string]any{"repository_id": int64(3)}))
	require.NoError(t, err)

	cloneCount, err := service.PendingCountByOperation(ctx, queue.OperationCloneRepository)
	require.NoError(t, err)
	assert.Equal(t, int64(2), cloneCount)

	syncCount, err := service.PendingCountByOperation(ctx, queue.OperationSyncRepository)
	require.NoError(t, err)
	assert.Equal(t, int64(1), syncCount)
}

func TestService_TaskByDedupKey_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	// Create and enqueue a task
	task := queue.NewTask(queue.OperationCloneRepository, int(domain.QueuePriorityNormal), map[string]any{"repository_id": int64(42)})
	err := service.Enqueue(ctx, task)
	require.NoError(t, err)

	// Find by dedup key
	found, exists, err := service.TaskByDedupKey(ctx, task.DedupKey())
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, task.DedupKey(), found.DedupKey())
	assert.Equal(t, queue.OperationCloneRepository, found.Operation())
}

func TestService_TaskByDedupKey_NotFound_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	_, exists, err := service.TaskByDedupKey(ctx, "nonexistent:123")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestService_Dequeue_ReturnsHighestPriority_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	taskRepo := queuepostgres.NewTaskRepository(db)
	logger := slog.Default()
	service := queue.NewService(taskRepo, logger)

	// Enqueue in reverse priority order
	err := service.Enqueue(ctx, queue.NewTask(queue.OperationSyncRepository, int(domain.QueuePriorityBackground), map[string]any{"repository_id": int64(1)}))
	require.NoError(t, err)
	err = service.Enqueue(ctx, queue.NewTask(queue.OperationSyncRepository, int(domain.QueuePriorityUserInitiated), map[string]any{"repository_id": int64(2)}))
	require.NoError(t, err)
	err = service.Enqueue(ctx, queue.NewTask(queue.OperationSyncRepository, int(domain.QueuePriorityNormal), map[string]any{"repository_id": int64(3)}))
	require.NoError(t, err)

	// Dequeue should return highest priority first
	task1, found, err := taskRepo.Dequeue(ctx)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int(domain.QueuePriorityUserInitiated), task1.Priority())

	task2, found, err := taskRepo.Dequeue(ctx)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int(domain.QueuePriorityNormal), task2.Priority())

	task3, found, err := taskRepo.Dequeue(ctx)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int(domain.QueuePriorityBackground), task3.Priority())

	// Queue should be empty now
	_, found, err = taskRepo.Dequeue(ctx)
	require.NoError(t, err)
	assert.False(t, found)
}
