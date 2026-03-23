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
)

type repositoryTestDeps struct {
	service *Repository
	stores  testStores
}

func newRepositoryTestDeps(t *testing.T) repositoryTestDeps {
	t.Helper()
	stores := newTestStores(t)
	queue := NewQueue(stores.tasks, zerolog.Nop())
	prescribedOps := task.FullPrescribedOperations()
	svc := NewRepository(stores.repos, stores.commits, stores.branches, stores.tags, queue, prescribedOps, zerolog.Nop())
	return repositoryTestDeps{service: svc, stores: stores}
}

func savedTasks(t *testing.T, deps repositoryTestDeps) []task.Task {
	t.Helper()
	tasks, err := deps.stores.tasks.Find(context.Background())
	require.NoError(t, err)
	return tasks
}

func TestRepository_Add_NewRepository(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	source, created, err := deps.service.Add(ctx, &RepositoryAddParams{
		URL: "https://github.com/test/repo",
	})

	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "https://github.com/test/repo", source.RemoteURL())

	tasks := savedTasks(t, deps)
	require.NotEmpty(t, tasks)

	operations := make([]task.Operation, len(tasks))
	for i, tsk := range tasks {
		operations[i] = tsk.Operation()
	}
	assert.Contains(t, operations, task.OperationCloneRepository)
}

func TestRepository_Add_ExistingByURL(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	source, created, err := deps.service.Add(ctx, &RepositoryAddParams{
		URL: "https://github.com/test/repo",
	})

	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, saved.ID(), source.ID())
	assert.Empty(t, savedTasks(t, deps))
}

func TestRepository_Add_ExistingByUpstreamURL(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.WithUpstreamURL("https://github.com/upstream/repo")
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	source, created, err := deps.service.Add(ctx, &RepositoryAddParams{
		URL:         "https://github.com/different/url",
		UpstreamURL: "https://github.com/upstream/repo",
	})

	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, saved.ID(), source.ID())
	assert.Empty(t, savedTasks(t, deps))
}

func TestRepository_Add_WithTrackingConfig(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	// Only one tracking field is stored per the persistence mapper (branch > tag > commit).
	source, created, err := deps.service.Add(ctx, &RepositoryAddParams{
		URL:    "https://github.com/test/repo",
		Branch: "develop",
	})

	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "develop", source.TrackingConfig().Branch())
}

func TestRepository_Delete_EnqueuesTask(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	err = deps.service.Delete(ctx, saved.ID())
	require.NoError(t, err)

	tasks := savedTasks(t, deps)
	require.Len(t, tasks, 1)
	assert.Equal(t, task.OperationDeleteRepository, tasks[0].Operation())
	assert.Equal(t, int(task.PriorityCritical), tasks[0].Priority())
}

func TestRepository_Delete_NotFound(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	err := deps.service.Delete(ctx, 999)
	assert.Error(t, err)
}

func TestRepository_Sync_EnqueuesOperations(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	err = deps.service.Sync(ctx, saved.ID())
	require.NoError(t, err)

	tasks := savedTasks(t, deps)
	require.NotEmpty(t, tasks)

	operations := make([]task.Operation, len(tasks))
	for i, tsk := range tasks {
		operations[i] = tsk.Operation()
	}
	assert.Contains(t, operations, task.OperationCloneRepository)
	assert.Contains(t, operations, task.OperationSyncRepository)
}

func TestRepository_Sync_NotFound(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	err := deps.service.Sync(ctx, 999)
	assert.Error(t, err)
}

func TestRepository_Rescan_EnqueuesOperations(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	err = deps.service.Rescan(ctx, &RescanParams{
		RepositoryID: saved.ID(),
		CommitSHA:    "abc123",
	})
	require.NoError(t, err)

	tasks := savedTasks(t, deps)
	require.NotEmpty(t, tasks)

	operations := make([]task.Operation, len(tasks))
	for i, tsk := range tasks {
		operations[i] = tsk.Operation()
	}
	assert.Contains(t, operations, task.OperationRescanCommit)
}

func TestRepository_Rescan_NotFound(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	err := deps.service.Rescan(ctx, &RescanParams{
		RepositoryID: 999,
		CommitSHA:    "abc123",
	})
	assert.Error(t, err)
}

func TestRepository_RescanAll(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo1, err := repository.NewRepository("https://github.com/test/repo1")
	require.NoError(t, err)
	_, err = deps.stores.repos.Save(ctx, repo1)
	require.NoError(t, err)

	repo2, err := repository.NewRepository("https://github.com/test/repo2")
	require.NoError(t, err)
	_, err = deps.stores.repos.Save(ctx, repo2)
	require.NoError(t, err)

	err = deps.service.RescanAll(ctx)
	require.NoError(t, err)

	tasks := savedTasks(t, deps)
	rescanCount := 0
	for _, tsk := range tasks {
		if tsk.Operation() == task.OperationRescanCommit {
			rescanCount++
		}
	}
	assert.GreaterOrEqual(t, rescanCount, 2)
}

func TestRepository_UpdateTrackingConfig(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	// Only one tracking field is stored per the persistence mapper (branch > tag > commit).
	source, err := deps.service.UpdateTrackingConfig(ctx, saved.ID(), &TrackingConfigParams{
		Tag: "v2.0",
	})

	require.NoError(t, err)
	assert.Equal(t, "v2.0", source.TrackingConfig().Tag())
}

func TestRepository_UpdateTrackingConfig_NotFound(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	_, err := deps.service.UpdateTrackingConfig(ctx, 999, &TrackingConfigParams{
		Branch: "develop",
	})
	assert.Error(t, err)
}

func TestRepository_SummaryByID(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	branch := repository.NewBranch(saved.ID(), "main", "abc123", true)
	_, err = deps.stores.branches.Save(ctx, branch)
	require.NoError(t, err)

	featureBranch := repository.NewBranch(saved.ID(), "feature", "def456", false)
	_, err = deps.stores.branches.Save(ctx, featureBranch)
	require.NoError(t, err)

	now := time.Now()
	commit := repository.NewCommit(
		"abc123", saved.ID(), "initial commit",
		repository.NewAuthor("Test", "test@test.com"),
		repository.NewAuthor("Test", "test@test.com"),
		now, now,
	)
	_, err = deps.stores.commits.Save(ctx, commit)
	require.NoError(t, err)

	tag := repository.NewTag(saved.ID(), "v1.0", "abc123")
	_, err = deps.stores.tags.Save(ctx, tag)
	require.NoError(t, err)

	summary, err := deps.service.SummaryByID(ctx, saved.ID())
	require.NoError(t, err)

	assert.Equal(t, saved.ID(), summary.Source().ID())
	assert.Equal(t, 2, summary.BranchCount())
	assert.Equal(t, 1, summary.TagCount())
	assert.Equal(t, 1, summary.CommitCount())
	assert.Equal(t, "main", summary.DefaultBranch())
}

func TestRepository_SummaryByID_NotFound(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	_, err := deps.service.SummaryByID(ctx, 999)
	assert.Error(t, err)
}

func TestRepository_UpdateChunkingConfig(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	// Verify defaults
	assert.Equal(t, 1500, saved.ChunkingConfig().Size())
	assert.Equal(t, 200, saved.ChunkingConfig().Overlap())
	assert.Equal(t, 50, saved.ChunkingConfig().MinSize())

	updated, err := deps.service.UpdateChunkingConfig(ctx, saved.ID(), &ChunkingConfigParams{
		Size:    2000,
		Overlap: 300,
		MinSize: 100,
	})

	require.NoError(t, err)
	assert.Equal(t, 2000, updated.ChunkingConfig().Size())
	assert.Equal(t, 300, updated.ChunkingConfig().Overlap())
	assert.Equal(t, 100, updated.ChunkingConfig().MinSize())

	// Verify persistence
	fetched, err := deps.stores.repos.FindOne(ctx, repository.WithID(saved.ID()))
	require.NoError(t, err)
	assert.Equal(t, 2000, fetched.ChunkingConfig().Size())
	assert.Equal(t, 300, fetched.ChunkingConfig().Overlap())
	assert.Equal(t, 100, fetched.ChunkingConfig().MinSize())
}

func TestRepository_UpdateChunkingConfig_NotFound(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	_, err := deps.service.UpdateChunkingConfig(ctx, 999, &ChunkingConfigParams{
		Size:    2000,
		Overlap: 300,
		MinSize: 100,
	})
	assert.Error(t, err)
}

func TestRepository_UpdateChunkingConfig_InvalidParams(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	_, err = deps.service.UpdateChunkingConfig(ctx, saved.ID(), &ChunkingConfigParams{
		Size:    100,
		Overlap: 200, // overlap >= size
		MinSize: 50,
	})
	assert.Error(t, err)
}

func TestRepository_BranchesForRepository(t *testing.T) {
	deps := newRepositoryTestDeps(t)
	ctx := context.Background()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	saved, err := deps.stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	branch1 := repository.NewBranch(saved.ID(), "main", "abc123", true)
	_, err = deps.stores.branches.Save(ctx, branch1)
	require.NoError(t, err)

	branch2 := repository.NewBranch(saved.ID(), "develop", "def456", false)
	_, err = deps.stores.branches.Save(ctx, branch2)
	require.NoError(t, err)

	branches, err := deps.service.BranchesForRepository(ctx, saved.ID())
	require.NoError(t, err)
	assert.Len(t, branches, 2)

	names := make([]string, len(branches))
	for i, b := range branches {
		names[i] = b.Name()
	}
	assert.Contains(t, names, "main")
	assert.Contains(t, names, "develop")
}
