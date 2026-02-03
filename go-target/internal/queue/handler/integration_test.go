package handler_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/git"
	gitpostgres "github.com/helixml/kodit/internal/git/postgres"
	"github.com/helixml/kodit/internal/queue"
	"github.com/helixml/kodit/internal/queue/handler"
	queuepostgres "github.com/helixml/kodit/internal/queue/postgres"
	"github.com/helixml/kodit/internal/testutil"
	"github.com/helixml/kodit/internal/tracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for queue handlers with real database.
// These tests mirror the Python tests in tests/kodit/application/handlers/.

// fakeAdapter is a controllable git adapter for handler testing.
type fakeAdapter struct {
	cloneErr       error
	clonedPaths    map[string]bool
	fetchErr       error
	pullErr        error
	branches       []git.BranchInfo
	commits        map[string]git.CommitInfo
	files          map[string][]git.FileInfo
	tags           []git.TagInfo
	defaultBranch  string
}

func newFakeAdapter() *fakeAdapter {
	return &fakeAdapter{
		clonedPaths: make(map[string]bool),
		commits:     make(map[string]git.CommitInfo),
		files:       make(map[string][]git.FileInfo),
	}
}

func (f *fakeAdapter) CloneRepository(_ context.Context, _ string, path string) error {
	if f.cloneErr != nil {
		return f.cloneErr
	}
	f.clonedPaths[path] = true
	return nil
}

func (f *fakeAdapter) CheckoutCommit(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakeAdapter) CheckoutBranch(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakeAdapter) FetchRepository(_ context.Context, _ string) error {
	return f.fetchErr
}

func (f *fakeAdapter) PullRepository(_ context.Context, _ string) error {
	return f.pullErr
}

func (f *fakeAdapter) AllBranches(_ context.Context, _ string) ([]git.BranchInfo, error) {
	return f.branches, nil
}

func (f *fakeAdapter) BranchCommits(_ context.Context, _ string, _ string) ([]git.CommitInfo, error) {
	return nil, nil
}

func (f *fakeAdapter) AllCommitsBulk(_ context.Context, _ string, _ *time.Time) (map[string]git.CommitInfo, error) {
	return f.commits, nil
}

func (f *fakeAdapter) BranchCommitSHAs(_ context.Context, _ string, _ string) ([]string, error) {
	return nil, nil
}

func (f *fakeAdapter) AllBranchHeadSHAs(_ context.Context, _ string, _ []string) (map[string]string, error) {
	return nil, nil
}

func (f *fakeAdapter) CommitFiles(_ context.Context, _ string, sha string) ([]git.FileInfo, error) {
	if files, ok := f.files[sha]; ok {
		return files, nil
	}
	return nil, nil
}

func (f *fakeAdapter) RepositoryExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (f *fakeAdapter) CommitDetails(_ context.Context, _ string, sha string) (git.CommitInfo, error) {
	if info, ok := f.commits[sha]; ok {
		return info, nil
	}
	return git.CommitInfo{SHA: sha}, nil
}

func (f *fakeAdapter) EnsureRepository(_ context.Context, _ string, path string) error {
	f.clonedPaths[path] = true
	return f.cloneErr
}

func (f *fakeAdapter) FileContent(_ context.Context, _ string, _ string, _ string) ([]byte, error) {
	return nil, nil
}

func (f *fakeAdapter) DefaultBranch(_ context.Context, _ string) (string, error) {
	if f.defaultBranch != "" {
		return f.defaultBranch, nil
	}
	return "main", nil
}

func (f *fakeAdapter) LatestCommitSHA(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func (f *fakeAdapter) AllTags(_ context.Context, _ string) ([]git.TagInfo, error) {
	return f.tags, nil
}

func (f *fakeAdapter) CommitDiff(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

// fakeTrackerFactory creates no-op trackers for testing.
type fakeTrackerFactory struct {
	events []testutil.ProgressEvent
}

func newFakeTrackerFactory() *fakeTrackerFactory {
	return &fakeTrackerFactory{
		events: make([]testutil.ProgressEvent, 0),
	}
}

func (f *fakeTrackerFactory) ForOperation(
	operation queue.TaskOperation,
	trackableType domain.TrackableType,
	trackableID int64,
) *tracking.Tracker {
	return tracking.TrackerForOperation(operation, nil, trackableType, trackableID)
}

// TestCloneRepositoryHandler_SavesWorkingCopy_Integration tests that the clone handler
// updates the repository with the cloned path.
func TestCloneRepositoryHandler_SavesWorkingCopy_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	taskRepo := queuepostgres.NewTaskRepository(db)
	queueService := queue.NewService(taskRepo, logger)

	// Create a repository without working copy
	repo, err := git.NewRepo("https://github.com/example/clone-test.git")
	require.NoError(t, err)
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)
	assert.False(t, savedRepo.HasWorkingCopy())

	// Create cloner with fake adapter
	adapter := newFakeAdapter()
	cloneDir := t.TempDir()
	cloner := git.NewCloner(adapter, cloneDir, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewCloneRepository(repoRepo, cloner, queueService, trackerFactory, logger)

	// Execute handler
	payload := map[string]any{"repository_id": savedRepo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Verify working copy was saved
	updatedRepo, err := repoRepo.Get(ctx, savedRepo.ID())
	require.NoError(t, err)
	assert.True(t, updatedRepo.HasWorkingCopy())
	assert.NotEmpty(t, updatedRepo.WorkingCopy().Path())
}

// TestCloneRepositoryHandler_SkipsAlreadyCloned_Integration tests that the clone handler
// skips repositories that already have a working copy.
func TestCloneRepositoryHandler_SkipsAlreadyCloned_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	taskRepo := queuepostgres.NewTaskRepository(db)
	queueService := queue.NewService(taskRepo, logger)

	// Create a repository WITH working copy
	repo, err := git.NewRepo("https://github.com/example/already-cloned.git")
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(git.NewWorkingCopy("/tmp/already-cloned", repo.RemoteURL()))
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)

	// Create cloner with adapter that would fail if called - but it shouldn't be
	adapter := newFakeAdapter()
	adapter.cloneErr = assert.AnError
	cloneDir := t.TempDir()
	cloner := git.NewCloner(adapter, cloneDir, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewCloneRepository(repoRepo, cloner, queueService, trackerFactory, logger)

	// Execute handler - should skip without error
	payload := map[string]any{"repository_id": savedRepo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)
}

// TestCloneRepositoryHandler_EnqueuesFollowUp_Integration tests that the clone handler
// enqueues a sync task after successful clone.
func TestCloneRepositoryHandler_EnqueuesFollowUp_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	taskRepo := queuepostgres.NewTaskRepository(db)
	queueService := queue.NewService(taskRepo, logger)

	// Create a repository
	repo, err := git.NewRepo("https://github.com/example/follow-up.git")
	require.NoError(t, err)
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)

	// Clear any existing tasks
	tasks, _ := queueService.List(ctx, nil)
	for _, task := range tasks {
		_ = taskRepo.Delete(ctx, task)
	}

	adapter := newFakeAdapter()
	cloneDir := t.TempDir()
	cloner := git.NewCloner(adapter, cloneDir, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewCloneRepository(repoRepo, cloner, queueService, trackerFactory, logger)

	// Execute handler
	payload := map[string]any{"repository_id": savedRepo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Verify sync task was enqueued
	tasks, err = queueService.List(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, queue.OperationSyncRepository, tasks[0].Operation())
}

// TestSyncRepositoryHandler_FetchesAndScans_Integration tests that sync handler
// updates the repository and scans branches.
func TestSyncRepositoryHandler_FetchesAndScans_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	branchRepo := gitpostgres.NewBranchRepository(db)
	taskRepo := queuepostgres.NewTaskRepository(db)
	queueService := queue.NewService(taskRepo, logger)

	// Create a repository with working copy
	repo, err := git.NewRepo("https://github.com/example/sync-test.git")
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(git.NewWorkingCopy("/tmp/sync-test", repo.RemoteURL()))
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)

	// Clear existing tasks
	tasks, _ := queueService.List(ctx, nil)
	for _, task := range tasks {
		_ = taskRepo.Delete(ctx, task)
	}

	// Create cloner and scanner with fake adapter
	adapter := newFakeAdapter()
	adapter.branches = []git.BranchInfo{
		{Name: "main", HeadSHA: "abc123", IsDefault: true},
		{Name: "develop", HeadSHA: "def456", IsDefault: false},
	}
	cloneDir := t.TempDir()
	cloner := git.NewCloner(adapter, cloneDir, logger)
	scanner := git.NewScanner(adapter, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewSyncRepository(repoRepo, branchRepo, cloner, scanner, queueService, trackerFactory, logger)

	// Execute handler
	payload := map[string]any{"repository_id": savedRepo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Verify branches were saved
	branches, err := branchRepo.FindByRepoID(ctx, savedRepo.ID())
	require.NoError(t, err)
	assert.Len(t, branches, 2)
}

// TestSyncRepositoryHandler_EnqueuesCommitScans_Integration tests that sync handler
// enqueues tasks to scan commits on the default branch.
func TestSyncRepositoryHandler_EnqueuesCommitScans_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	branchRepo := gitpostgres.NewBranchRepository(db)
	taskRepo := queuepostgres.NewTaskRepository(db)
	queueService := queue.NewService(taskRepo, logger)

	// Create a repository with working copy
	repo, err := git.NewRepo("https://github.com/example/commit-scan.git")
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(git.NewWorkingCopy("/tmp/commit-scan", repo.RemoteURL()))
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)

	// Clear existing tasks
	tasks, _ := queueService.List(ctx, nil)
	for _, task := range tasks {
		_ = taskRepo.Delete(ctx, task)
	}

	adapter := newFakeAdapter()
	adapter.branches = []git.BranchInfo{
		{Name: "main", HeadSHA: "abc123def", IsDefault: true},
	}
	cloneDir := t.TempDir()
	cloner := git.NewCloner(adapter, cloneDir, logger)
	scanner := git.NewScanner(adapter, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewSyncRepository(repoRepo, branchRepo, cloner, scanner, queueService, trackerFactory, logger)

	// Execute handler
	payload := map[string]any{"repository_id": savedRepo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Verify scan tasks were enqueued
	tasks, err = queueService.List(ctx, nil)
	require.NoError(t, err)
	assert.Greater(t, len(tasks), 0)

	// First should be scan commit
	assert.Equal(t, queue.OperationScanCommit, tasks[0].Operation())
}

// TestScanCommitHandler_SavesCommitAndFiles_Integration tests that the scan commit handler
// persists commit and file information.
func TestScanCommitHandler_SavesCommitAndFiles_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	commitRepo := gitpostgres.NewCommitRepository(db)
	fileRepo := gitpostgres.NewFileRepository(db)

	// Create a repository with working copy
	repo, err := git.NewRepo("https://github.com/example/scan-commit.git")
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(git.NewWorkingCopy("/tmp/scan-commit", repo.RemoteURL()))
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)

	// Set up fake adapter with commit and file data
	adapter := newFakeAdapter()
	now := time.Now()
	adapter.commits["abc123def456"] = git.CommitInfo{
		SHA:            "abc123def456",
		Message:        "Test commit",
		AuthorName:     "Test Author",
		AuthorEmail:    "test@example.com",
		CommitterName:  "Test Author",
		CommitterEmail: "test@example.com",
		AuthoredAt:     now,
		CommittedAt:    now,
	}
	adapter.files["abc123def456"] = []git.FileInfo{
		{Path: "main.go", Size: 1234},
		{Path: "README.md", Size: 567},
	}

	scanner := git.NewScanner(adapter, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewScanCommit(repoRepo, commitRepo, fileRepo, scanner, trackerFactory, logger)

	// Execute handler
	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    "abc123def456",
	}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Verify commit was saved
	savedCommit, err := commitRepo.GetByRepoAndSHA(ctx, savedRepo.ID(), "abc123def456")
	require.NoError(t, err)
	assert.Equal(t, "Test commit", savedCommit.Message())

	// Verify files were saved
	savedFiles, err := fileRepo.FindByCommitSHA(ctx, "abc123def456")
	require.NoError(t, err)
	assert.Len(t, savedFiles, 2)
}

// TestScanCommitHandler_IsIdempotent_Integration tests that scanning the same commit twice
// doesn't create duplicates.
func TestScanCommitHandler_IsIdempotent_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	commitRepo := gitpostgres.NewCommitRepository(db)
	fileRepo := gitpostgres.NewFileRepository(db)

	// Create a repository with working copy
	repo, err := git.NewRepo("https://github.com/example/idempotent.git")
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(git.NewWorkingCopy("/tmp/idempotent", repo.RemoteURL()))
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)

	// Set up fake adapter with commit data
	adapter := newFakeAdapter()
	now := time.Now()
	adapter.commits["idempotent123"] = git.CommitInfo{
		SHA:            "idempotent123",
		Message:        "Idempotent test",
		AuthorName:     "Test Author",
		AuthorEmail:    "test@example.com",
		CommitterName:  "Test Author",
		CommitterEmail: "test@example.com",
		AuthoredAt:     now,
		CommittedAt:    now,
	}

	scanner := git.NewScanner(adapter, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewScanCommit(repoRepo, commitRepo, fileRepo, scanner, trackerFactory, logger)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    "idempotent123",
	}

	// Execute first time
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Execute second time - should skip
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Verify only one commit exists
	commits, err := commitRepo.FindByRepoID(ctx, savedRepo.ID())
	require.NoError(t, err)
	assert.Len(t, commits, 1)
}

// TestSyncRepositoryHandler_RequiresWorkingCopy_Integration tests that sync fails
// without a working copy.
func TestSyncRepositoryHandler_RequiresWorkingCopy_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	branchRepo := gitpostgres.NewBranchRepository(db)
	taskRepo := queuepostgres.NewTaskRepository(db)
	queueService := queue.NewService(taskRepo, logger)

	// Create a repository WITHOUT working copy
	repo, err := git.NewRepo("https://github.com/example/no-wc.git")
	require.NoError(t, err)
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)

	adapter := newFakeAdapter()
	cloneDir := t.TempDir()
	cloner := git.NewCloner(adapter, cloneDir, logger)
	scanner := git.NewScanner(adapter, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewSyncRepository(repoRepo, branchRepo, cloner, scanner, queueService, trackerFactory, logger)

	// Execute handler - should fail
	payload := map[string]any{"repository_id": savedRepo.ID()}
	err = h.Execute(ctx, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not been cloned")
}

// TestScanCommitHandler_RequiresWorkingCopy_Integration tests that scan fails
// without a working copy.
func TestScanCommitHandler_RequiresWorkingCopy_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	commitRepo := gitpostgres.NewCommitRepository(db)
	fileRepo := gitpostgres.NewFileRepository(db)

	// Create a repository WITHOUT working copy
	repo, err := git.NewRepo("https://github.com/example/scan-no-wc.git")
	require.NoError(t, err)
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)

	adapter := newFakeAdapter()
	scanner := git.NewScanner(adapter, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewScanCommit(repoRepo, commitRepo, fileRepo, scanner, trackerFactory, logger)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    "abc123",
	}
	err = h.Execute(ctx, payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not been cloned")
}

// TestSyncRepositoryHandler_WithTrackingConfig_Integration tests that sync respects
// branch tracking configuration.
func TestSyncRepositoryHandler_WithTrackingConfig_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	repoRepo := gitpostgres.NewRepoRepository(db)
	branchRepo := gitpostgres.NewBranchRepository(db)
	taskRepo := queuepostgres.NewTaskRepository(db)
	queueService := queue.NewService(taskRepo, logger)

	// Create a repository with working copy AND tracking config for develop branch
	repo, err := git.NewRepo("https://github.com/example/tracking.git")
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(git.NewWorkingCopy("/tmp/tracking", repo.RemoteURL()))
	repo = repo.WithTrackingConfig(git.NewTrackingConfigForBranch("develop"))
	savedRepo, err := repoRepo.Save(ctx, repo)
	require.NoError(t, err)

	// Clear existing tasks
	tasks, _ := queueService.List(ctx, nil)
	for _, task := range tasks {
		_ = taskRepo.Delete(ctx, task)
	}

	adapter := newFakeAdapter()
	adapter.branches = []git.BranchInfo{
		{Name: "main", HeadSHA: "main-sha", IsDefault: true},
		{Name: "develop", HeadSHA: "develop-sha", IsDefault: false},
	}
	cloneDir := t.TempDir()
	cloner := git.NewCloner(adapter, cloneDir, logger)
	scanner := git.NewScanner(adapter, logger)
	trackerFactory := newFakeTrackerFactory()

	h := handler.NewSyncRepository(repoRepo, branchRepo, cloner, scanner, queueService, trackerFactory, logger)

	// Execute handler
	payload := map[string]any{"repository_id": savedRepo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Verify tasks were enqueued for the develop branch commit
	tasks, err = queueService.List(ctx, nil)
	require.NoError(t, err)
	assert.Greater(t, len(tasks), 0)

	// Check that the payload has the develop branch's commit SHA
	scanTask := tasks[0]
	taskPayload := scanTask.Payload()
	commitSHA, ok := taskPayload["commit_sha"].(string)
	require.True(t, ok)
	assert.Equal(t, "develop-sha", commitSHA)
}
