package service

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/internal/config"
)

func TestPeriodicSync_Enabled(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()
	logger := zerolog.Nop()

	remoteA := "https://github.com/org/repo-a"
	repoA, err := repository.NewRepository(remoteA)
	require.NoError(t, err)
	repoA = repoA.WithWorkingCopy(repository.NewWorkingCopy("/tmp/a", remoteA))
	_, err = stores.repos.Save(ctx, repoA)
	require.NoError(t, err)

	remoteB := "https://github.com/org/repo-b"
	repoB, err := repository.NewRepository(remoteB)
	require.NoError(t, err)
	repoB = repoB.WithWorkingCopy(repository.NewWorkingCopy("/tmp/b", remoteB))
	_, err = stores.repos.Save(ctx, repoB)
	require.NoError(t, err)

	queue := NewQueue(stores.tasks, logger)

	cfg := config.NewPeriodicSyncConfig().
		WithEnabled(true).
		WithIntervalSeconds(0.01).     // 10ms
		WithCheckIntervalSeconds(0.01) // 10ms

	ps := NewPeriodicSync(cfg, stores.repos, queue, task.FullPrescribedOperations(), logger)
	ps.Start(ctx)

	require.Eventually(t, func() bool {
		tasks, _ := stores.tasks.Find(ctx)
		return len(tasks) >= 2
	}, time.Second, 5*time.Millisecond)

	ps.Stop()

	tasks, err := stores.tasks.Find(ctx)
	require.NoError(t, err)
	syncOps := 0
	for _, tsk := range tasks {
		if tsk.Operation() == task.OperationSyncRepository {
			syncOps++
		}
	}
	assert.GreaterOrEqual(t, syncOps, 2)
}

func TestPeriodicSync_Disabled(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()
	logger := zerolog.Nop()

	remote := "https://github.com/org/repo"
	repo, err := repository.NewRepository(remote)
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy("/tmp/r", remote))
	_, err = stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	queue := NewQueue(stores.tasks, logger)

	cfg := config.NewPeriodicSyncConfig().
		WithEnabled(false)

	ps := NewPeriodicSync(cfg, stores.repos, queue, task.FullPrescribedOperations(), logger)
	ps.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	ps.Stop()

	tasks, err := stores.tasks.Find(ctx)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestPeriodicSync_EmptyRepositories(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()
	logger := zerolog.Nop()

	queue := NewQueue(stores.tasks, logger)

	cfg := config.NewPeriodicSyncConfig().
		WithEnabled(true).
		WithIntervalSeconds(0.01).
		WithCheckIntervalSeconds(0.01)

	ps := NewPeriodicSync(cfg, stores.repos, queue, task.FullPrescribedOperations(), logger)
	ps.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	ps.Stop()

	tasks, err := stores.tasks.Find(ctx)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}
