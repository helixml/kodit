// Package handler provides task handlers for processing queued operations.
package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/helixml/kodit/domain/task"
)

// ErrNoHandler indicates no handler is registered for the operation.
var ErrNoHandler = errors.New("no handler registered")

// Tracker provides progress tracking for task execution.
type Tracker interface {
	SetTotal(ctx context.Context, total int) error
	SetCurrent(ctx context.Context, current int, message string) error
	Skip(ctx context.Context, message string) error
	Fail(ctx context.Context, message string) error
	Complete(ctx context.Context) error
}

// TrackerFactory creates trackers for progress reporting.
type TrackerFactory interface {
	ForOperation(operation task.Operation, trackableType task.TrackableType, trackableID int64) Tracker
}

// Handler defines the interface for task operation handlers.
type Handler interface {
	Execute(ctx context.Context, payload map[string]any) error
}

// Registry maps task operations to their handlers.
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

// Register adds a handler for a task operation.
// Subsequent registrations for the same operation will overwrite the previous handler.
func (r *Registry) Register(operation task.Operation, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[operation] = handler
}

// Handler returns the handler for a task operation.
// Returns ErrNoHandler if no handler is registered.
func (r *Registry) Handler(operation task.Operation) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, ok := r.handlers[operation]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoHandler, operation)
	}
	return handler, nil
}

// HasHandler checks if a handler is registered for the operation.
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

// ExtractInt64 extracts an int64 value from the payload.
func ExtractInt64(payload map[string]any, key string) (int64, error) {
	val, ok := payload[key]
	if !ok {
		return 0, fmt.Errorf("missing required field: %s", key)
	}

	switch v := val.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("invalid type for %s: %T", key, val)
	}
}

// ExtractString extracts a string value from the payload.
func ExtractString(payload map[string]any, key string) (string, error) {
	val, ok := payload[key]
	if !ok {
		return "", fmt.Errorf("missing required field: %s", key)
	}

	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for %s: expected string, got %T", key, val)
	}

	return s, nil
}

// ShortSHA returns the first 8 characters of a SHA for display purposes.
func ShortSHA(sha string) string {
	if len(sha) >= 8 {
		return sha[:8]
	}
	return sha
}
