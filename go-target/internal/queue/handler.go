package queue

import "context"

// Handler defines the interface for task operation handlers.
type Handler interface {
	// Execute processes the task payload.
	Execute(ctx context.Context, payload map[string]any) error
}
