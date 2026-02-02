package queue

import (
	"errors"
	"fmt"
	"sync"
)

// ErrNoHandler indicates no handler is registered for the operation.
var ErrNoHandler = errors.New("no handler registered")

// Registry maps task operations to their handlers.
type Registry struct {
	handlers map[TaskOperation]Handler
	mu       sync.RWMutex
}

// NewRegistry creates a new handler registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[TaskOperation]Handler),
	}
}

// Register adds a handler for a task operation.
// Subsequent registrations for the same operation will overwrite the previous handler.
func (r *Registry) Register(operation TaskOperation, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[operation] = handler
}

// Handler returns the handler for a task operation.
// Returns ErrNoHandler if no handler is registered.
func (r *Registry) Handler(operation TaskOperation) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, ok := r.handlers[operation]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoHandler, operation)
	}
	return handler, nil
}

// HasHandler checks if a handler is registered for the operation.
func (r *Registry) HasHandler(operation TaskOperation) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.handlers[operation]
	return ok
}

// Operations returns all registered operations.
func (r *Registry) Operations() []TaskOperation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ops := make([]TaskOperation, 0, len(r.handlers))
	for op := range r.handlers {
		ops = append(ops, op)
	}
	return ops
}
