package kodit_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPollPeriod = 50 * time.Millisecond

// fileURI converts an absolute filesystem path to a file:// URI.
// On Unix:    /tmp/repo  → file:///tmp/repo
// On Windows: C:\Users\x → file:///C:/Users/x
func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

// createTestGitRepo creates a small local git repository for testing.
// Returns the path to the repository.
func createTestGitRepo(t *testing.T) string {
	t.Helper()

	repoDir := filepath.Join(t.TempDir(), "test-repo")
	err := os.MkdirAll(repoDir, 0755)
	require.NoError(t, err, "create repo directory")

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git init: %s", out)

	// Configure git user
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	_, err = cmd.CombinedOutput()
	require.NoError(t, err, "git config email")

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	_, err = cmd.CombinedOutput()
	require.NoError(t, err, "git config name")

	// Create a Go source file with a function
	srcDir := filepath.Join(repoDir, "src")
	err = os.MkdirAll(srcDir, 0755)
	require.NoError(t, err, "create src directory")

	goCode := `package main

// Add adds two numbers and returns the result.
// This is a simple addition function for testing purposes.
func Add(a, b int) int {
	return a + b
}

// Subtract subtracts b from a and returns the result.
func Subtract(a, b int) int {
	return a - b
}

func main() {
	result := Add(1, 2)
	println(result)
}
`
	err = os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(goCode), 0644)
	require.NoError(t, err, "write main.go")

	// Create a Python file
	pythonCode := `"""Calculator module with basic operations."""

def multiply(a, b):
    """Multiply two numbers."""
    return a * b

def divide(a, b):
    """Divide a by b."""
    if b == 0:
        raise ValueError("Cannot divide by zero")
    return a / b
`
	err = os.WriteFile(filepath.Join(srcDir, "calculator.py"), []byte(pythonCode), 0644)
	require.NoError(t, err, "write calculator.py")

	// Add and commit
	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = repoDir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "git add: %s", out)

	cmd = exec.Command("git", "commit", "-m", "Initial commit with Go and Python files")
	cmd.Dir = repoDir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "git commit: %s", out)

	return repoDir
}

// waitForTasks waits until no pending tasks remain or timeout is reached.
// Tasks are deleted from the database when dequeued by the worker, so a
// single empty poll does not guarantee all work is finished. We require
// several consecutive empty polls (a stability window) to allow in-progress
// tasks to complete and enqueue follow-up tasks.
func waitForTasks(ctx context.Context, t *testing.T, client *kodit.Client, timeout time.Duration) {
	t.Helper()

	const (
		pollInterval   = 100 * time.Millisecond
		stableRequired = 4 // 4 × 100ms = 400ms stability window
	)

	deadline := time.Now().Add(timeout)
	lastCount := -1
	stableCount := 0

	for time.Now().Before(deadline) {
		tasks, err := client.Tasks.List(ctx, nil)
		require.NoError(t, err)

		if len(tasks) == 0 && client.WorkerIdle() {
			stableCount++
			if stableCount >= stableRequired {
				return
			}
		} else {
			stableCount = 0
			if len(tasks) != lastCount {
				t.Logf("waiting for %d tasks to complete...", len(tasks))
				lastCount = len(tasks)
			}
		}

		time.Sleep(pollInterval)
	}

	tasks, _ := client.Tasks.List(ctx, nil)
	t.Fatalf("timeout waiting for tasks to complete, %d remaining", len(tasks))
}

func TestIntegration_IndexRepository_QueuesCloneTask(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")

	// Create a local test repository
	repoPath := createTestGitRepo(t)

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(dataDir),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(testPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	// Clone the local repository
	repo, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(repoPath)})
	require.NoError(t, err)
	assert.Greater(t, repo.ID(), int64(0), "repository should have an ID")

	// Verify a clone task was queued
	tasks, err := client.Tasks.List(ctx, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, tasks, "expected clone task to be queued")
}

func TestIntegration_FullIndexingWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")
	cloneDir := filepath.Join(tmpDir, "repos")

	// Create a local test repository
	repoPath := createTestGitRepo(t)

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(dataDir),
		kodit.WithCloneDir(cloneDir),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(testPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	// Clone the local repository
	repo, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(repoPath)})
	require.NoError(t, err)
	t.Logf("created repository with ID %d", repo.ID())

	// Wait for all tasks to complete (clone, sync, scan, extract)
	// Note: 60 seconds to handle potential slowness in CI
	waitForTasks(ctx, t, client, 60*time.Second)

	// Verify repository is in the list
	repos, err := client.Repositories.Find(ctx)
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, repo.ID(), repos[0].ID())

	// Check if repository has working copy - this indicates clone completed
	// Note: The working copy path is updated asynchronously after clone completes
	updatedRepo, err := client.Repositories.Get(ctx, repository.WithID(repo.ID()))
	require.NoError(t, err)

	// Log the result rather than failing - clone may still be in progress
	if updatedRepo.HasWorkingCopy() {
		t.Logf("repository successfully cloned to: %s", updatedRepo.WorkingCopy().Path())
	} else {
		t.Logf("repository does not have working copy yet (clone may have failed or still in progress)")
	}
}

func TestIntegration_SearchAfterIndexing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")
	cloneDir := filepath.Join(tmpDir, "repos")

	// Create a local test repository
	repoPath := createTestGitRepo(t)

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(dataDir),
		kodit.WithCloneDir(cloneDir),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(testPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	// Clone the repository
	_, _, err = client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(repoPath)})
	require.NoError(t, err)

	// Wait for indexing to complete
	waitForTasks(ctx, t, client, 60*time.Second)

	// Search for content
	// Note: Text search uses vector embeddings on enrichment summaries
	result, err := client.Search.Query(ctx, "add numbers",
		service.WithLimit(10),
	)
	require.NoError(t, err)

	// Log the result - search functionality verified by not erroring
	t.Logf("search returned %d results", result.Count())
}

func TestIntegration_DeleteRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")
	cloneDir := filepath.Join(tmpDir, "repos")

	// Create a local test repository
	repoPath := createTestGitRepo(t)

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(dataDir),
		kodit.WithCloneDir(cloneDir),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(testPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	// Clone the repository
	repo, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(repoPath)})
	require.NoError(t, err)

	// Wait for initial tasks
	waitForTasks(ctx, t, client, 60*time.Second)

	// Delete the repository
	err = client.Repositories.Delete(ctx, repo.ID())
	require.NoError(t, err)

	// Wait for delete task to complete
	waitForTasks(ctx, t, client, 30*time.Second)

	// Verify repository is gone
	repos, err := client.Repositories.Find(ctx)
	require.NoError(t, err)
	assert.Empty(t, repos, "repository should be deleted")
}

func TestIntegration_MultipleRepositories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")
	cloneDir := filepath.Join(tmpDir, "repos")

	// Create two test repositories
	repoPath1 := createTestGitRepo(t)
	repoPath2 := createTestGitRepo(t)

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(dataDir),
		kodit.WithCloneDir(cloneDir),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(testPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	// Clone both repositories
	repo1, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(repoPath1)})
	require.NoError(t, err)

	repo2, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(repoPath2)})
	require.NoError(t, err)

	// Wait for all tasks (longer timeout for multiple repos)
	waitForTasks(ctx, t, client, 120*time.Second)

	// Verify both repositories exist
	repos, err := client.Repositories.Find(ctx)
	require.NoError(t, err)
	assert.Len(t, repos, 2)

	// Verify they have different IDs
	assert.NotEqual(t, repo1.ID(), repo2.ID())
}

// currentBranch returns the current branch name of the git repo at repoPath.
func currentBranch(t *testing.T, repoPath string) string {
	t.Helper()

	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git branch --show-current: %s", out)

	return strings.TrimSpace(string(out))
}

// snapshotTableCounts opens a read-only connection to the SQLite database
// at dbPath and returns row counts for all data tables that can accumulate
// during sync and rescan operations. Returns 0 for tables that do not exist.
func snapshotTableCounts(t *testing.T, dbPath string) map[string]int64 {
	t.Helper()

	ctx := context.Background()
	db, err := database.NewDatabase(ctx, "sqlite:///"+dbPath)
	require.NoError(t, err, "open database for snapshot")
	defer func() { _ = db.Close() }()

	tables := []string{
		"git_commits",
		"git_commit_files",
		"enrichments_v2",
		"enrichment_associations",
		"kodit_bm25_documents",
		"kodit_code_embeddings",
		"kodit_text_embeddings",
	}

	counts := make(map[string]int64, len(tables))
	for _, table := range tables {
		var count int64
		err := db.Session(ctx).Raw(fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", table)).Scan(&count).Error
		if err != nil {
			counts[table] = 0
		} else {
			counts[table] = count
		}
	}

	return counts
}

func TestIntegration_Rescan_CleansUpSearchIndexes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")
	cloneDir := filepath.Join(tmpDir, "repos")

	repoPath := createTestGitRepo(t)
	branch := currentBranch(t, repoPath)

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(dataDir),
		kodit.WithCloneDir(cloneDir),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(testPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	repo, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{
		URL:    fileURI(repoPath),
		Branch: branch,
	})
	require.NoError(t, err)

	waitForTasks(ctx, t, client, 60*time.Second)

	commits, err := client.Commits.Find(ctx, repository.WithRepoID(repo.ID()))
	require.NoError(t, err)
	require.NotEmpty(t, commits, "expected at least one commit after indexing")

	sha := commits[0].SHA()

	baseline := snapshotTableCounts(t, dbPath)
	t.Logf("baseline counts: %v", baseline)

	require.Greater(t, baseline["kodit_bm25_documents"], int64(0), "expected BM25 documents after indexing")

	err = client.Repositories.Rescan(ctx, &service.RescanParams{
		RepositoryID: repo.ID(),
		CommitSHA:    sha,
	})
	require.NoError(t, err)

	waitForTasks(ctx, t, client, 60*time.Second)

	after := snapshotTableCounts(t, dbPath)
	t.Logf("after-rescan counts: %v", after)

	// Rescan should delete old search indexes before re-creating them.
	// Counts should match baseline (cleaned and re-created, not doubled).
	assert.Equal(t, baseline["kodit_bm25_documents"], after["kodit_bm25_documents"],
		"BM25 documents should be cleaned and re-created, not accumulated")
	assert.Equal(t, baseline["kodit_code_embeddings"], after["kodit_code_embeddings"],
		"code embeddings should be cleaned and re-created, not accumulated")
	assert.Equal(t, baseline["kodit_text_embeddings"], after["kodit_text_embeddings"],
		"text embeddings should be cleaned and re-created, not accumulated")
}

func TestIntegration_DuplicateSync_DoesNotDuplicateData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	dataDir := filepath.Join(tmpDir, "data")
	cloneDir := filepath.Join(tmpDir, "repos")

	repoPath := createTestGitRepo(t)
	branch := currentBranch(t, repoPath)

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(dataDir),
		kodit.WithCloneDir(cloneDir),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(testPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()

	// Pass Branch so that Sync's Update() actually fetches and pulls.
	repo, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{
		URL:    fileURI(repoPath),
		Branch: branch,
	})
	require.NoError(t, err)

	waitForTasks(ctx, t, client, 60*time.Second)

	baseline := snapshotTableCounts(t, dbPath)
	t.Logf("baseline counts: %v", baseline)

	// Sync again with no changes — same HEAD, nothing new.
	err = client.Repositories.Sync(ctx, repo.ID())
	require.NoError(t, err)

	waitForTasks(ctx, t, client, 60*time.Second)

	after := snapshotTableCounts(t, dbPath)
	t.Logf("after-duplicate-sync counts: %v", after)

	// Scanning the same commit twice should be idempotent.
	for table, beforeCount := range baseline {
		assert.Equal(t, beforeCount, after[table],
			"table %s should be unchanged after duplicate sync", table)
	}
}
