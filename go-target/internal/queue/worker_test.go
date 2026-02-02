package queue

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorker_ProcessOne(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	handler := NewFakeHandler()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	registry.Register(OperationCloneRepository, handler)

	worker := NewWorker(repo, registry, logger)

	// Add a task
	payload := map[string]any{"repo_id": int64(42)}
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, payload))

	// Process it
	processed, err := worker.ProcessOne(ctx)
	require.NoError(t, err)
	assert.True(t, processed)

	// Handler should have been called
	assert.Equal(t, 1, handler.CallCount())
	assert.Equal(t, int64(42), handler.LastCall()["repo_id"])

	// Task should be removed from queue
	tasks := repo.All()
	assert.Empty(t, tasks)
}

func TestWorker_ProcessOne_NoTasks(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	worker := NewWorker(repo, registry, logger)

	processed, err := worker.ProcessOne(ctx)
	require.NoError(t, err)
	assert.False(t, processed)
}

func TestWorker_ProcessOne_NoHandler(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	worker := NewWorker(repo, registry, logger)

	// Add a task with no registered handler
	payload := map[string]any{"repo_id": int64(42)}
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, payload))

	// Process it - should succeed but task removed
	processed, err := worker.ProcessOne(ctx)
	require.NoError(t, err)
	assert.True(t, processed)

	// Task should be removed (even without a handler)
	tasks := repo.All()
	assert.Empty(t, tasks)
}

func TestWorker_ProcessOne_HandlerError(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	handler := NewFakeHandler()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handlerErr := errors.New("handler failed")
	handler.ReturnFn = func(_ map[string]any) error {
		return handlerErr
	}

	registry.Register(OperationCloneRepository, handler)

	worker := NewWorker(repo, registry, logger)

	payload := map[string]any{"repo_id": int64(42)}
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, payload))

	// Process it
	processed, err := worker.ProcessOne(ctx)
	require.NoError(t, err) // Task deletion succeeds even if handler fails
	assert.True(t, processed)

	// Handler was called
	assert.Equal(t, 1, handler.CallCount())

	// Task should be removed (failed tasks are not retried)
	tasks := repo.All()
	assert.Empty(t, tasks)
}

func TestWorker_ProcessesByPriority(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	handler := NewFakeHandler()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	registry.Register(OperationCloneRepository, handler)

	worker := NewWorker(repo, registry, logger)

	// Add tasks with different priorities
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, map[string]any{"repo_id": int64(1)}))
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 300, map[string]any{"repo_id": int64(2)}))
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 200, map[string]any{"repo_id": int64(3)}))

	// Process all tasks
	for range 3 {
		processed, err := worker.ProcessOne(ctx)
		require.NoError(t, err)
		assert.True(t, processed)
	}

	// Highest priority should be processed first
	assert.Equal(t, 3, handler.CallCount())
	assert.Equal(t, int64(2), handler.Calls[0]["repo_id"]) // priority 300
	assert.Equal(t, int64(3), handler.Calls[1]["repo_id"]) // priority 200
	assert.Equal(t, int64(1), handler.Calls[2]["repo_id"]) // priority 100
}

func TestWorker_StartStop(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	handler := NewFakeHandler()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	registry.Register(OperationCloneRepository, handler)

	worker := NewWorker(repo, registry, logger).
		WithPollPeriod(10 * time.Millisecond)

	// Add a task
	payload := map[string]any{"repo_id": int64(42)}
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, payload))

	// Start worker
	worker.Start(ctx)

	// Wait for task to be processed
	require.Eventually(t, func() bool {
		return handler.CallCount() > 0
	}, time.Second, 10*time.Millisecond)

	// Stop worker
	worker.Stop()

	// Handler should have been called
	assert.GreaterOrEqual(t, handler.CallCount(), 1)
	assert.Equal(t, int64(42), handler.Calls[0]["repo_id"])
}

func TestWorker_GracefulShutdown(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	var mu sync.Mutex
	started := false
	finished := false

	handler := NewFakeHandler()
	handler.ReturnFn = func(_ map[string]any) error {
		mu.Lock()
		started = true
		mu.Unlock()

		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		finished = true
		mu.Unlock()
		return nil
	}

	registry.Register(OperationCloneRepository, handler)

	worker := NewWorker(repo, registry, logger).
		WithPollPeriod(10 * time.Millisecond)

	// Add a task
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, map[string]any{"repo_id": int64(1)}))

	worker.Start(ctx)

	// Wait for handler to start
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return started
	}, time.Second, 10*time.Millisecond)

	// Stop should wait for the handler to finish
	worker.Stop()

	mu.Lock()
	wasFinished := finished
	mu.Unlock()

	assert.True(t, wasFinished, "worker should wait for handler to complete")
}

func TestWorker_ProcessesMultipleTasks(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	handler := NewFakeHandler()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	registry.Register(OperationCloneRepository, handler)

	worker := NewWorker(repo, registry, logger).
		WithPollPeriod(10 * time.Millisecond)

	// Add multiple tasks
	for i := range 5 {
		_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100+i, map[string]any{"repo_id": int64(i)}))
	}

	worker.Start(ctx)

	// Wait for all tasks to be processed
	require.Eventually(t, func() bool {
		return handler.CallCount() == 5
	}, time.Second, 10*time.Millisecond)

	worker.Stop()

	assert.Equal(t, 5, handler.CallCount())
}

func TestWorker_WithPollPeriod(t *testing.T) {
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	worker := NewWorker(repo, registry, logger).
		WithPollPeriod(500 * time.Millisecond)

	assert.Equal(t, 500*time.Millisecond, worker.pollPeriod)
}

func TestWorker_MultipleDifferentOperations(t *testing.T) {
	ctx := context.Background()
	repo := NewFakeTaskRepository()
	registry := NewRegistry()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	cloneHandler := NewFakeHandler()
	syncHandler := NewFakeHandler()

	registry.Register(OperationCloneRepository, cloneHandler)
	registry.Register(OperationSyncRepository, syncHandler)

	worker := NewWorker(repo, registry, logger)

	// Add tasks of different types
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 100, map[string]any{"repo_id": int64(1)}))
	_, _ = repo.Save(ctx, NewTask(OperationSyncRepository, 200, map[string]any{"repo_id": int64(2)}))
	_, _ = repo.Save(ctx, NewTask(OperationCloneRepository, 150, map[string]any{"repo_id": int64(3)}))

	// Process all tasks
	for range 3 {
		processed, err := worker.ProcessOne(ctx)
		require.NoError(t, err)
		assert.True(t, processed)
	}

	// Both handlers should be called
	assert.Equal(t, 2, cloneHandler.CallCount())
	assert.Equal(t, 1, syncHandler.CallCount())
}
