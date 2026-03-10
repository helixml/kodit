package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

type handlerFunc func(ctx context.Context, payload map[string]any) error

func (f handlerFunc) Execute(ctx context.Context, payload map[string]any) error {
	return f(ctx, payload)
}

type recordingTracker struct {
	completed bool
	failMsg   string
}

func (r *recordingTracker) Fail(_ context.Context, message string) { r.failMsg = message }
func (r *recordingTracker) Complete(_ context.Context)             { r.completed = true }

type recordingTrackerFactory struct {
	trackers []*recordingTracker
}

func (f *recordingTrackerFactory) ForOperation(_ task.Operation, _ map[string]any) WorkerTracker {
	t := &recordingTracker{}
	f.trackers = append(f.trackers, t)
	return t
}

func TestWorker_ProcessOne(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	ctx := context.Background()

	tsk := task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(1)})
	_, err := store.Save(ctx, tsk)
	require.NoError(t, err)

	executed := false
	registry := NewRegistry()
	registry.Register(task.OperationSyncRepository, handlerFunc(func(_ context.Context, _ map[string]any) error {
		executed = true
		return nil
	}))

	factory := &recordingTrackerFactory{}
	statusStore := persistence.NewStatusStore(db)
	worker := NewWorker(store, statusStore, registry, factory, zerolog.Nop())

	found, err := worker.ProcessOne(ctx)
	require.NoError(t, err)
	assert.True(t, found)
	assert.True(t, executed)

	count, err := store.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	require.Len(t, factory.trackers, 1)
	assert.True(t, factory.trackers[0].completed)
}

func TestWorker_ProcessOne_FailedHandler(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	ctx := context.Background()

	tsk := task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(1)})
	_, err := store.Save(ctx, tsk)
	require.NoError(t, err)

	registry := NewRegistry()
	registry.Register(task.OperationSyncRepository, handlerFunc(func(_ context.Context, _ map[string]any) error {
		return errors.New("handler failed")
	}))

	factory := &recordingTrackerFactory{}
	statusStore := persistence.NewStatusStore(db)
	worker := NewWorker(store, statusStore, registry, factory, zerolog.Nop())

	found, err := worker.ProcessOne(ctx)
	require.NoError(t, err)
	assert.True(t, found)

	count, err := store.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	require.Len(t, factory.trackers, 1)
	assert.Equal(t, "handler failed", factory.trackers[0].failMsg)
}

func TestWorker_ProcessOne_EmptyQueue(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)

	registry := NewRegistry()
	statusStore := persistence.NewStatusStore(db)
	worker := NewWorker(store, statusStore, registry, nil, zerolog.Nop())

	found, err := worker.ProcessOne(context.Background())
	require.NoError(t, err)
	assert.False(t, found)
}

func TestWorker_ProcessOne_UnregisteredHandler(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	ctx := context.Background()

	tsk := task.NewTask(task.OperationSyncRepository, int(task.PriorityNormal), map[string]any{"repository_id": int64(1)})
	_, err := store.Save(ctx, tsk)
	require.NoError(t, err)

	registry := NewRegistry()
	statusStore := persistence.NewStatusStore(db)
	worker := NewWorker(store, statusStore, registry, nil, zerolog.Nop())

	found, err := worker.ProcessOne(ctx)
	require.NoError(t, err)
	assert.True(t, found)

	count, err := store.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestWorker_Start_RecoverStaleStatuses(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	statusStore := persistence.NewStatusStore(db)
	ctx := context.Background()

	started := task.NewStatus("op_a", nil, task.TrackableTypeRepository, 1)
	inProgress := task.NewStatus("op_b", nil, task.TrackableTypeRepository, 2).
		SetCurrent(5, "working")
	completed := task.NewStatus("op_c", nil, task.TrackableTypeRepository, 3).
		Complete()

	for _, s := range []task.Status{started, inProgress, completed} {
		_, err := statusStore.Save(ctx, s)
		require.NoError(t, err)
	}

	registry := NewRegistry()
	worker := NewWorker(store, statusStore, registry, nil, zerolog.Nop())

	err := worker.Start(ctx)
	require.NoError(t, err)
	worker.Stop()

	all, err := statusStore.Find(ctx)
	require.NoError(t, err)
	require.Len(t, all, 3)

	states := map[task.Operation]task.ReportingState{}
	for _, s := range all {
		states[s.Operation()] = s.State()
	}
	assert.Equal(t, task.ReportingStateFailed, states["op_a"])
	assert.Equal(t, task.ReportingStateFailed, states["op_b"])
	assert.Equal(t, task.ReportingStateCompleted, states["op_c"])
}

func TestWorker_Start_NoStaleStatuses(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	statusStore := persistence.NewStatusStore(db)
	ctx := context.Background()

	registry := NewRegistry()
	worker := NewWorker(store, statusStore, registry, nil, zerolog.Nop())

	err := worker.Start(ctx)
	require.NoError(t, err)
	worker.Stop()
}

func TestWorker_ProcessOne_HighestPriorityFirst(t *testing.T) {
	db := testdb.New(t)
	store := persistence.NewTaskStore(db)
	ctx := context.Background()

	low := task.NewTask(task.OperationSyncRepository, int(task.PriorityBackground), map[string]any{"repository_id": int64(1)})
	high := task.NewTask(task.OperationScanCommit, int(task.PriorityCritical), map[string]any{"commit_sha": "abc"})

	_, err := store.Save(ctx, low)
	require.NoError(t, err)
	_, err = store.Save(ctx, high)
	require.NoError(t, err)

	var processed []task.Operation
	registry := NewRegistry()
	handler := handlerFunc(func(_ context.Context, payload map[string]any) error {
		return nil
	})
	registry.Register(task.OperationSyncRepository, handler)
	registry.Register(task.OperationScanCommit, handler)

	// Dequeue both tasks and verify priority ordering
	for i := 0; i < 2; i++ {
		tsk, ok, err := store.Dequeue(ctx)
		require.NoError(t, err)
		require.True(t, ok)
		processed = append(processed, tsk.Operation())
	}

	require.Len(t, processed, 2)
	assert.Equal(t, task.OperationScanCommit, processed[0])
	assert.Equal(t, task.OperationSyncRepository, processed[1])
}
