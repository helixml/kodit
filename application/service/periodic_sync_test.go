package service

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepositoryStore struct {
	mu    sync.Mutex
	repos []repository.Repository
}

func (f *fakeRepositoryStore) Find(_ context.Context, _ ...repository.Option) ([]repository.Repository, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]repository.Repository, len(f.repos))
	copy(result, f.repos)
	return result, nil
}

func (f *fakeRepositoryStore) FindOne(_ context.Context, _ ...repository.Option) (repository.Repository, error) {
	return repository.Repository{}, nil
}

func (f *fakeRepositoryStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return int64(len(f.repos)), nil
}

func (f *fakeRepositoryStore) Save(_ context.Context, r repository.Repository) (repository.Repository, error) {
	return r, nil
}

func (f *fakeRepositoryStore) Delete(_ context.Context, _ repository.Repository) error {
	return nil
}

func (f *fakeRepositoryStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return false, nil
}

type fakeTaskStore struct {
	mu    sync.Mutex
	tasks []task.Task
}

func (f *fakeTaskStore) Get(_ context.Context, _ int64) (task.Task, error) {
	return task.Task{}, nil
}

func (f *fakeTaskStore) FindAll(_ context.Context) ([]task.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tasks, nil
}

func (f *fakeTaskStore) FindPending(_ context.Context, _ ...repository.Option) ([]task.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tasks, nil
}

func (f *fakeTaskStore) Save(_ context.Context, t task.Task) (task.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tasks = append(f.tasks, t)
	return t, nil
}

func (f *fakeTaskStore) SaveBulk(_ context.Context, tasks []task.Task) ([]task.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tasks = append(f.tasks, tasks...)
	return tasks, nil
}

func (f *fakeTaskStore) Delete(_ context.Context, _ task.Task) error {
	return nil
}

func (f *fakeTaskStore) DeleteAll(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tasks = nil
	return nil
}

func (f *fakeTaskStore) CountPending(_ context.Context, _ ...repository.Option) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return int64(len(f.tasks)), nil
}

func (f *fakeTaskStore) Exists(_ context.Context, _ int64) (bool, error) {
	return false, nil
}

func (f *fakeTaskStore) Dequeue(_ context.Context) (task.Task, bool, error) {
	return task.Task{}, false, nil
}

func (f *fakeTaskStore) DequeueByOperation(_ context.Context, _ task.Operation) (task.Task, bool, error) {
	return task.Task{}, false, nil
}

func (f *fakeTaskStore) savedTasks() []task.Task {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]task.Task, len(f.tasks))
	copy(result, f.tasks)
	return result
}

func TestPeriodicSync_Enabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	repoStore := &fakeRepositoryStore{
		repos: []repository.Repository{
			repository.ReconstructRepository(1, "https://github.com/org/repo-a",
				repository.NewWorkingCopy("/tmp/a", "https://github.com/org/repo-a"),
				repository.NewTrackingConfig("main", "", ""),
				time.Now(), time.Now(), time.Time{}),
			repository.ReconstructRepository(2, "https://github.com/org/repo-b",
				repository.NewWorkingCopy("/tmp/b", "https://github.com/org/repo-b"),
				repository.NewTrackingConfig("main", "", ""),
				time.Now(), time.Now(), time.Time{}),
		},
	}

	taskStore := &fakeTaskStore{}
	queue := NewQueue(taskStore, logger)

	cfg := config.NewPeriodicSyncConfig().
		WithEnabled(true).
		WithIntervalSeconds(0.01).     // 10ms
		WithCheckIntervalSeconds(0.01) // 10ms

	ps := NewPeriodicSync(cfg, repoStore, queue, task.NewPrescribedOperations(true, true), logger)
	ps.Start(context.Background())

	require.Eventually(t, func() bool {
		return len(taskStore.savedTasks()) >= 2
	}, time.Second, 5*time.Millisecond)

	ps.Stop()

	tasks := taskStore.savedTasks()
	syncOps := 0
	for _, tsk := range tasks {
		if tsk.Operation() == task.OperationSyncRepository {
			syncOps++
		}
	}
	assert.GreaterOrEqual(t, syncOps, 2)
}

func TestPeriodicSync_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	repoStore := &fakeRepositoryStore{
		repos: []repository.Repository{
			repository.ReconstructRepository(1, "https://github.com/org/repo",
				repository.NewWorkingCopy("/tmp/r", "https://github.com/org/repo"),
				repository.NewTrackingConfig("main", "", ""),
				time.Now(), time.Now(), time.Time{}),
		},
	}

	taskStore := &fakeTaskStore{}
	queue := NewQueue(taskStore, logger)

	cfg := config.NewPeriodicSyncConfig().
		WithEnabled(false)

	ps := NewPeriodicSync(cfg, repoStore, queue, task.NewPrescribedOperations(true, true), logger)
	ps.Start(context.Background())

	time.Sleep(50 * time.Millisecond)

	ps.Stop()

	assert.Empty(t, taskStore.savedTasks())
}

func TestPeriodicSync_EmptyRepositories(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	repoStore := &fakeRepositoryStore{}
	taskStore := &fakeTaskStore{}
	queue := NewQueue(taskStore, logger)

	cfg := config.NewPeriodicSyncConfig().
		WithEnabled(true).
		WithIntervalSeconds(0.01).
		WithCheckIntervalSeconds(0.01)

	ps := NewPeriodicSync(cfg, repoStore, queue, task.NewPrescribedOperations(true, true), logger)
	ps.Start(context.Background())

	time.Sleep(50 * time.Millisecond)

	ps.Stop()

	assert.Empty(t, taskStore.savedTasks())
}
