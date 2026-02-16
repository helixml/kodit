package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/helixml/kodit/domain/task"
)

// WorkerTracker marks a task status as failed or complete.
type WorkerTracker interface {
	Fail(ctx context.Context, message string)
	Complete(ctx context.Context)
}

// WorkerTrackerFactory creates trackers for task status updates.
type WorkerTrackerFactory interface {
	ForOperation(operation task.Operation, trackableType task.TrackableType, trackableID int64) WorkerTracker
}

// Handler executes a specific task operation.
type Handler interface {
	Execute(ctx context.Context, payload map[string]any) error
}

// Registry manages task handlers for different operations.
type Registry struct {
	handlers map[task.Operation]Handler
	mu       sync.RWMutex
}

// NewRegistry creates a new handler registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[task.Operation]Handler),
	}
}

// Register registers a handler for an operation.
func (r *Registry) Register(operation task.Operation, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[operation] = handler
}

// Handler returns the handler for an operation.
func (r *Registry) Handler(operation task.Operation) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[operation]
	return handler, ok
}

// HasHandler reports whether a handler is registered for the operation.
func (r *Registry) HasHandler(operation task.Operation) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.handlers[operation]
	return ok
}

// Operations returns all registered operations.
func (r *Registry) Operations() []task.Operation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ops := make([]task.Operation, 0, len(r.handlers))
	for op := range r.handlers {
		ops = append(ops, op)
	}
	return ops
}

// Worker processes tasks from the queue.
type Worker struct {
	store          task.TaskStore
	registry       *Registry
	trackerFactory WorkerTrackerFactory
	logger         *slog.Logger
	pollPeriod     time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewWorker creates a new queue worker.
func NewWorker(store task.TaskStore, registry *Registry, trackerFactory WorkerTrackerFactory, logger *slog.Logger) *Worker {
	return &Worker{
		store:          store,
		registry:       registry,
		trackerFactory: trackerFactory,
		logger:         logger,
		pollPeriod:     time.Second,
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
	t, found, err := w.store.Dequeue(ctx)
	if err != nil {
		return err
	}
	if !found {
		return nil // No tasks to process
	}

	return w.processTask(ctx, t)
}

func (w *Worker) processTask(ctx context.Context, t task.Task) error {
	start := time.Now()

	w.logger.Info("processing task",
		slog.Int64("task_id", t.ID()),
		slog.String("operation", t.Operation().String()),
	)

	h, ok := w.registry.Handler(t.Operation())
	if !ok {
		w.logger.Error("no handler for operation",
			slog.Int64("task_id", t.ID()),
			slog.String("operation", t.Operation().String()),
		)
		// Delete the task anyway to prevent it from blocking the queue
		return w.store.Delete(ctx, t)
	}

	if err := w.executeWithRecovery(ctx, h, t); err != nil {
		w.logger.Error("task execution failed",
			slog.Int64("task_id", t.ID()),
			slog.String("operation", t.Operation().String()),
			slog.String("error", err.Error()),
		)
		w.markStatusFailed(ctx, t, err)
		// Delete the task - failed tasks are not retried
		return w.store.Delete(ctx, t)
	}

	w.markStatusComplete(ctx, t)

	duration := time.Since(start)
	w.logger.Info("task completed",
		slog.Int64("task_id", t.ID()),
		slog.String("operation", t.Operation().String()),
		slog.Duration("duration", duration),
	)

	return w.store.Delete(ctx, t)
}

func (w *Worker) executeWithRecovery(ctx context.Context, h Handler, t task.Task) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()
	return h.Execute(ctx, t.Payload())
}

// markStatusFailed updates the tracking status to failed for a task that errored.
func (w *Worker) markStatusFailed(ctx context.Context, t task.Task, err error) {
	if w.trackerFactory == nil {
		return
	}

	payload := t.Payload()
	repoID, _ := extractInt64(payload, "repository_id")
	if repoID == 0 {
		return
	}

	tracker := w.trackerFactory.ForOperation(t.Operation(), task.TrackableTypeRepository, repoID)
	tracker.Fail(ctx, err.Error())
}

// markStatusComplete updates the tracking status to complete for a task that succeeded.
func (w *Worker) markStatusComplete(ctx context.Context, t task.Task) {
	if w.trackerFactory == nil {
		return
	}

	payload := t.Payload()
	repoID, _ := extractInt64(payload, "repository_id")
	if repoID == 0 {
		return
	}

	tracker := w.trackerFactory.ForOperation(t.Operation(), task.TrackableTypeRepository, repoID)
	tracker.Complete(ctx)
}

func extractInt64(payload map[string]any, key string) (int64, bool) {
	val, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

// ProcessOne processes a single task synchronously (for testing).
func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	t, found, err := w.store.Dequeue(ctx)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	err = w.processTask(ctx, t)
	return true, err
}
