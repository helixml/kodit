package queue

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Worker processes tasks from the queue.
type Worker struct {
	repo       TaskRepository
	registry   *Registry
	logger     *slog.Logger
	pollPeriod time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewWorker creates a new queue worker.
func NewWorker(repo TaskRepository, registry *Registry, logger *slog.Logger) *Worker {
	return &Worker{
		repo:       repo,
		registry:   registry,
		logger:     logger,
		pollPeriod: time.Second,
	}
}

// WithPollPeriod sets the poll period for checking new tasks.
func (w *Worker) WithPollPeriod(d time.Duration) *Worker {
	w.pollPeriod = d
	return w
}

// Start begins processing tasks from the queue.
// The worker runs in a goroutine and can be stopped with Stop().
func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	ctx, w.cancel = context.WithCancel(ctx)
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		w.run(ctx)
	}()

	w.logger.Info("queue worker started")
}

// Stop gracefully shuts down the worker.
// It waits for the current task to complete before returning.
func (w *Worker) Stop() {
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	w.wg.Wait()
	w.logger.Info("queue worker stopped")
}

func (w *Worker) run(ctx context.Context) {
	w.logger.Debug("worker loop started")

	ticker := time.NewTicker(w.pollPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker loop stopping")
			return
		case <-ticker.C:
			if err := w.processNext(ctx); err != nil {
				if ctx.Err() != nil {
					return // Context cancelled, exit cleanly
				}
				w.logger.Error("error processing task",
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

func (w *Worker) processNext(ctx context.Context) error {
	task, found, err := w.repo.Dequeue(ctx)
	if err != nil {
		return err
	}
	if !found {
		return nil // No tasks to process
	}

	return w.processTask(ctx, task)
}

func (w *Worker) processTask(ctx context.Context, task Task) error {
	start := time.Now()

	w.logger.Info("processing task",
		slog.Int64("task_id", task.ID()),
		slog.String("operation", task.Operation().String()),
	)

	handler, err := w.registry.Handler(task.Operation())
	if err != nil {
		w.logger.Error("no handler for operation",
			slog.Int64("task_id", task.ID()),
			slog.String("operation", task.Operation().String()),
			slog.String("error", err.Error()),
		)
		// Delete the task anyway to prevent it from blocking the queue
		return w.repo.Delete(ctx, task)
	}

	if err := handler.Execute(ctx, task.Payload()); err != nil {
		w.logger.Error("task execution failed",
			slog.Int64("task_id", task.ID()),
			slog.String("operation", task.Operation().String()),
			slog.String("error", err.Error()),
		)
		// Delete the task - failed tasks are not retried
		return w.repo.Delete(ctx, task)
	}

	duration := time.Since(start)
	w.logger.Info("task completed",
		slog.Int64("task_id", task.ID()),
		slog.String("operation", task.Operation().String()),
		slog.Duration("duration", duration),
	)

	return w.repo.Delete(ctx, task)
}

// ProcessOne processes a single task synchronously (for testing).
func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	task, found, err := w.repo.Dequeue(ctx)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	err = w.processTask(ctx, task)
	return true, err
}
