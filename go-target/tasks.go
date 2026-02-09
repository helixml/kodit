package kodit

import (
	"context"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/task"
)

// Tasks provides task queue operations.
type Tasks interface {
	// List returns tasks matching the given filter.
	List(ctx context.Context, filter task.Filter) ([]task.Task, error)

	// Get retrieves a task by ID.
	Get(ctx context.Context, id int64) (task.Task, error)

	// Cancel cancels a pending task.
	Cancel(ctx context.Context, id int64) error
}

// tasksImpl implements Tasks as a thin forwarder to Queue.
type tasksImpl struct {
	queue *service.Queue
}

func (t *tasksImpl) List(ctx context.Context, filter task.Filter) ([]task.Task, error) {
	return t.queue.ListFiltered(ctx, filter)
}

func (t *tasksImpl) Get(ctx context.Context, id int64) (task.Task, error) {
	return t.queue.Get(ctx, id)
}

func (t *tasksImpl) Cancel(ctx context.Context, id int64) error {
	return t.queue.Cancel(ctx, id)
}
