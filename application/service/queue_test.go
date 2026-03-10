package service

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

func TestQueue_Enqueue(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	queue := NewQueue(store, zerolog.Nop())
	ctx := context.Background()

	tsk := task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(1)})
	require.NoError(t, queue.Enqueue(ctx, tsk))

	tasks, err := queue.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, task.OperationSyncRepository, tasks[0].Operation())
}

func TestQueue_EnqueueDeduplicates(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	queue := NewQueue(store, zerolog.Nop())
	ctx := context.Background()

	tsk := task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(1)})
	require.NoError(t, queue.Enqueue(ctx, tsk))
	require.NoError(t, queue.Enqueue(ctx, tsk))

	count, err := queue.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestQueue_ListSortsByPriority(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	queue := NewQueue(store, zerolog.Nop())
	ctx := context.Background()

	low := task.NewTask(task.OperationSyncRepository, int(task.PriorityBackground), map[string]any{"repository_id": int64(1)})
	high := task.NewTask(task.OperationScanCommit, int(task.PriorityCritical), map[string]any{"commit_sha": "abc"})

	require.NoError(t, queue.Enqueue(ctx, low))
	require.NoError(t, queue.Enqueue(ctx, high))

	tasks, err := queue.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	assert.Equal(t, task.OperationScanCommit, tasks[0].Operation())
	assert.Equal(t, task.OperationSyncRepository, tasks[1].Operation())
}

func TestQueue_ListFiltersByOperation(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	queue := NewQueue(store, zerolog.Nop())
	ctx := context.Background()

	sync := task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(1)})
	scan := task.NewTask(task.OperationScanCommit, int(task.PriorityNormal), map[string]any{"commit_sha": "abc"})

	require.NoError(t, queue.Enqueue(ctx, sync))
	require.NoError(t, queue.Enqueue(ctx, scan))

	op := task.OperationSyncRepository
	tasks, err := queue.List(ctx, &TaskListParams{Operation: &op})
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, task.OperationSyncRepository, tasks[0].Operation())
}

func TestQueue_Remove(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	queue := NewQueue(store, zerolog.Nop())
	ctx := context.Background()

	tsk := task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(1)})
	require.NoError(t, queue.Enqueue(ctx, tsk))

	tasks, err := queue.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	require.NoError(t, queue.Remove(ctx, tasks[0].ID()))

	count, err := queue.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestQueue_Reprioritize(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	queue := NewQueue(store, zerolog.Nop())
	ctx := context.Background()

	tsk := task.NewTask(task.OperationSyncRepository, int(task.PriorityBackground), map[string]any{"repository_id": int64(1)})
	require.NoError(t, queue.Enqueue(ctx, tsk))

	tasks, err := queue.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	require.NoError(t, queue.Reprioritize(ctx, tasks[0].ID(), int(task.PriorityCritical)))

	updated, err := queue.Get(ctx, tasks[0].ID())
	require.NoError(t, err)
	assert.Equal(t, int(task.PriorityCritical), updated.Priority())
}

func TestQueue_DrainForRepository(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	queue := NewQueue(store, zerolog.Nop())
	ctx := context.Background()

	t1 := task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(1)})
	t2 := task.NewTask(task.OperationScanCommit, int(task.PriorityNormal), map[string]any{"repository_id": int64(2)})

	require.NoError(t, queue.Enqueue(ctx, t1))
	require.NoError(t, queue.Enqueue(ctx, t2))

	removed, err := queue.DrainForRepository(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	tasks, err := queue.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, int64(2), payloadRepoID(tasks[0].Payload()))
}
