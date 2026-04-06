package repository

import (
	"context"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/repository"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

type fakeSyncTracker struct{}

func (f *fakeSyncTracker) SetTotal(_ context.Context, _ int)             {}
func (f *fakeSyncTracker) SetCurrent(_ context.Context, _ int, _ string) {}
func (f *fakeSyncTracker) Skip(_ context.Context, _ string)              {}
func (f *fakeSyncTracker) Fail(_ context.Context, _ string)              {}
func (f *fakeSyncTracker) Complete(_ context.Context)                    {}

type fakeSyncTrackerFactory struct{}

func (f *fakeSyncTrackerFactory) ForOperation(_ task.Operation, _ map[string]any) handler.Tracker {
	return &fakeSyncTracker{}
}

type fakeCloner struct {
	path string
}

func (f *fakeCloner) ClonePathFromURI(_ string) string { return f.path }
func (f *fakeCloner) Clone(_ context.Context, _ string) (string, error) {
	return f.path, nil
}
func (f *fakeCloner) CloneToPath(_ context.Context, _ string, _ string) error { return nil }
func (f *fakeCloner) Update(_ context.Context, repo repository.Repository) (string, error) {
	return repo.WorkingCopy().Path(), nil
}
func (f *fakeCloner) Ensure(_ context.Context, _ string) (string, error) {
	return f.path, nil
}

type fakeScanner struct {
	branches []repository.Branch
}

func (f *fakeScanner) ScanCommit(_ context.Context, _ string, _ string, _ int64) (domainservice.ScanCommitResult, error) {
	return domainservice.ScanCommitResult{}, nil
}
func (f *fakeScanner) ScanBranch(_ context.Context, _ string, _ string, _ int64) ([]repository.Commit, error) {
	return nil, nil
}
func (f *fakeScanner) ScanAllBranches(_ context.Context, _ string, _ int64) ([]repository.Branch, error) {
	return f.branches, nil
}
func (f *fakeScanner) ScanAllTags(_ context.Context, _ string, _ int64) ([]repository.Tag, error) {
	return nil, nil
}
func (f *fakeScanner) FilesForCommitsBatch(_ context.Context, _ string, _ []string) ([]repository.File, error) {
	return nil, nil
}

type fakeResolver struct{}

func (f *fakeResolver) DefaultID(_ context.Context) (int64, error) { return 1, nil }
func (f *fakeResolver) Operations(_ context.Context, _ int64) ([]task.Operation, error) {
	return []task.Operation{task.OperationScanCommit}, nil
}

func newSyncHandler(t *testing.T, scanner *fakeScanner) (*Sync, persistence.RepositoryStore, persistence.BranchStore) {
	t.Helper()
	db := testdb.New(t)
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)

	repoStore := persistence.NewRepositoryStore(db)
	branchStore := persistence.NewBranchStore(db)
	taskStore := persistence.NewTaskStore(db)
	queue := service.NewQueue(taskStore, logger)

	h := NewSync(
		repoStore,
		branchStore,
		&fakeCloner{path: "/tmp/clone"},
		scanner,
		queue,
		&fakeResolver{},
		&fakeSyncTrackerFactory{},
		logger,
	)

	return h, repoStore, branchStore
}

func TestSync_SetsDefaultTrackingConfigWhenMissing(t *testing.T) {
	ctx := context.Background()

	defaultBranch := repository.NewBranch(1, "main", "abc123", true)
	otherBranch := repository.NewBranch(1, "feature", "def456", false)
	scanner := &fakeScanner{branches: []repository.Branch{defaultBranch, otherBranch}}

	h, repoStore, _ := newSyncHandler(t, scanner)

	// Create a repo with no tracking config.
	repo, err := repository.NewRepository("https://example.com/test/repo.git")
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy("/tmp/clone", "https://example.com/test/repo.git"))
	repo = repo.WithPipelineID(1)
	repo, err = repoStore.Save(ctx, repo)
	require.NoError(t, err)
	require.False(t, repo.HasTrackingConfig())

	payload := map[string]any{"repository_id": repo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Reload and verify a default tracking config was set.
	updated, err := repoStore.FindOne(ctx, repository.WithID(repo.ID()))
	require.NoError(t, err)
	assert.True(t, updated.HasTrackingConfig(), "expected tracking config to be set")
	assert.Equal(t, "main", updated.TrackingConfig().Branch())
}

func TestSync_PreservesExistingTrackingConfig(t *testing.T) {
	ctx := context.Background()

	defaultBranch := repository.NewBranch(1, "main", "abc123", true)
	scanner := &fakeScanner{branches: []repository.Branch{defaultBranch}}

	h, repoStore, _ := newSyncHandler(t, scanner)

	// Create a repo with an explicit tracking config.
	repo, err := repository.NewRepository("https://example.com/test/repo.git")
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy("/tmp/clone", "https://example.com/test/repo.git"))
	repo = repo.WithPipelineID(1)
	repo = repo.WithTrackingConfig(repository.NewTrackingConfigForBranch("develop"))
	repo, err = repoStore.Save(ctx, repo)
	require.NoError(t, err)

	payload := map[string]any{"repository_id": repo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Tracking config should remain unchanged.
	updated, err := repoStore.FindOne(ctx, repository.WithID(repo.ID()))
	require.NoError(t, err)
	assert.Equal(t, "develop", updated.TrackingConfig().Branch())
}
