package kodit_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/database"
)

func newTenancyClient(t *testing.T, opts ...kodit.Option) *kodit.Client {
	t.Helper()
	tmpDir := t.TempDir()
	client, err := kodit.New(append([]kodit.Option{
		kodit.WithSQLite(filepath.Join(tmpDir, "test.db")),
		kodit.WithDataDir(filepath.Join(tmpDir, "data")),
		kodit.WithCloneDir(filepath.Join(tmpDir, "repos")),
		kodit.WithEmbeddingProvider(stubEmbedder{}),
		kodit.WithWorkerPollPeriod(testPollPeriod),
	}, opts...)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
}

func latestSHA(t *testing.T, client *kodit.Client, repoID int64) string {
	t.Helper()
	commits, err := client.Commits.Find(context.Background(),
		repository.WithRepoID(repoID), repository.WithOrderDesc("date"), repository.WithLimit(1))
	require.NoError(t, err)
	require.NotEmpty(t, commits, "repository %d has no commits", repoID)
	return commits[0].SHA()
}

// keywordSearch runs kodit_keyword_search restricted to repo through the
// unscoped handler (source_repo) and through a handler scoped to repo.
func keywordSearch(t *testing.T, client *kodit.Client, repo repository.Source, keywords string) []string {
	t.Helper()
	var out []string
	unscoped := startMCP(t, kodit.NewMCPHandler(client))
	text, _ := callTool(t, unscoped, "kodit_keyword_search", map[string]any{
		"keywords": keywords, "source_repo": repo.Repo().UpstreamURL(),
	})
	out = append(out, text)
	scoped := startMCP(t, kodit.NewScopedMCPHandler(client, []int64{repo.ID()}))
	text, _ = callTool(t, scoped, "kodit_keyword_search", map[string]any{"keywords": keywords})
	out = append(out, text)
	return out
}

// TestIntegration_DirectorySnapshots_AreNotSharedAcrossRepositories covers two
// directories registered with identical first contents. A directory's
// snapshot SHA is salted with its repository ID, so after A adds a private
// file and is rescanned in place (as a knowledge folder is), B's searches
// never return A's private chunk, and B keeps its own results.
func TestIntegration_DirectorySnapshots_AreNotSharedAcrossRepositories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	const privateText = "zanzibarprivatememo"
	first := map[string]string{"guide.md": "# Shared guide\n\nquokkasharedcontent appears in both folders.\n"}
	dirA := filepath.Join(t.TempDir(), "folder-a")
	dirB := filepath.Join(t.TempDir(), "folder-b")
	writeFiles(t, dirA, first)
	writeFiles(t, dirB, first)

	client := newTenancyClient(t)
	ctx := context.Background()
	repoA, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(dirA)})
	require.NoError(t, err)
	repoB, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(dirB)})
	require.NoError(t, err)
	waitForTasks(ctx, t, client, 60*time.Second)

	shaA, shaB := latestSHA(t, client, repoA.ID()), latestSHA(t, client, repoB.ID())
	require.NotEqual(t, shaA, shaB, "identical directories must not share a snapshot SHA")

	writeFiles(t, dirA, map[string]string{"private.md": "# Private\n\n" + privateText + " is only in folder A.\n"})
	require.NoError(t, client.Repositories.Rescan(ctx, &service.RescanParams{RepositoryID: repoA.ID(), CommitSHA: shaA}))
	waitForTasks(ctx, t, client, 60*time.Second)

	for _, text := range keywordSearch(t, client, repoA, privateText) {
		assert.Contains(t, text, "private.md", "A must find its own private file")
	}
	for _, text := range keywordSearch(t, client, repoB, privateText) {
		assert.NotContains(t, text, privateText)
		assert.NotContains(t, text, "private.md")
	}
	for _, text := range keywordSearch(t, client, repoB, "quokkasharedcontent") {
		assert.Contains(t, text, "guide.md", "B keeps its own results")
	}
}

// TestIntegration_SharedGitCommit_VisibleToBothRepositories covers two git
// repositories with the same history. git_commits records the commit once,
// for the repository scanned first; the other repository's indexing shares
// the commit's chunks, so neither repository's results are emptied, and a
// rescan by the owner keeps the other repository's results.
func TestIntegration_SharedGitCommit_VisibleToBothRepositories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	pathA := createTestGitRepo(t)
	pathB := filepath.Join(t.TempDir(), "fork")
	out, err := exec.Command("git", "clone", "-q", pathA, pathB).CombinedOutput()
	require.NoError(t, err, "%s", out)

	client := newTenancyClient(t, kodit.WithLocalGitRepositories())
	ctx := context.Background()
	repoA, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(pathA), Branch: currentBranch(t, pathA)})
	require.NoError(t, err)
	waitForTasks(ctx, t, client, 60*time.Second)
	repoB, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(pathB), Branch: currentBranch(t, pathB)})
	require.NoError(t, err)
	waitForTasks(ctx, t, client, 60*time.Second)

	sha := latestSHA(t, client, repoA.ID())
	owner, err := client.Commits.Find(ctx, repository.WithSHA(sha))
	require.NoError(t, err)
	require.Len(t, owner, 1)
	assert.Equal(t, repoA.ID(), owner[0].RepoID(), "the second repository must not take over the commit")

	for _, repo := range []repository.Source{repoA, repoB} {
		for _, text := range keywordSearch(t, client, repo, "Subtract") {
			assert.Contains(t, text, "main.go", "repository %d lost its results for the shared commit", repo.ID())
		}
	}

	// The owner rescans the shared commit: the other repository keeps its
	// results, and the owner's are restored by indexing.
	require.NoError(t, client.Repositories.Rescan(ctx, &service.RescanParams{RepositoryID: repoA.ID(), CommitSHA: sha}))
	waitForTasks(ctx, t, client, 60*time.Second)
	// The other repository cannot rescan a commit recorded for the owner.
	require.ErrorIs(t, client.Repositories.Rescan(ctx, &service.RescanParams{RepositoryID: repoB.ID(), CommitSHA: sha}), database.ErrNotFound)
	waitForTasks(ctx, t, client, 60*time.Second)

	for _, repo := range []repository.Source{repoA, repoB} {
		for _, text := range keywordSearch(t, client, repo, "Subtract") {
			assert.Contains(t, text, "main.go", "repository %d lost its results after the rescans", repo.ID())
		}
	}
}

// TestIntegration_UntrustedDirectory_IsIndexedAsPlainFiles registers a
// directory laid out as a bare git repository whose object store borrows
// another repository's objects and whose config runs a command on diff. A
// client without WithLocalGitRepositories indexes it as plain files: the
// other repository's content never appears and no command runs.
func TestIntegration_UntrustedDirectory_IsIndexedAsPlainFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	base := t.TempDir()
	marker := filepath.Join(base, "command-ran")
	other := createTestGitRepo(t)
	const secret = "pelicanothersecret"
	writeFiles(t, other, map[string]string{"secret.txt": secret + " lives only in the other repository\n"})
	for _, args := range [][]string{{"add", "secret.txt"}, {"commit", "-q", "-m", "secret"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = other
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
	}
	headOut, err := exec.Command("git", "-C", other, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	head := strings.TrimSpace(string(headOut))

	touch := fmt.Sprintf("sh -c 'touch %s' --", marker)
	dir := filepath.Join(base, "uploaded")
	writeFiles(t, dir, map[string]string{
		"HEAD":                    "ref: refs/heads/main\n",
		"config":                  fmt.Sprintf("[core]\n\tbare = true\n\tfsmonitor = %s\n[diff \"x\"]\n\ttextconv = %s\n[diff]\n\texternal = %s\n", touch, touch, touch),
		"refs/heads/main":         head + "\n",
		"objects/info/alternates": filepath.Join(other, ".git", "objects") + "\n",
		"info/attributes":         "* diff=x\n",
		"notes.md":                "# Notes\n\nOrdinary uploaded notes.\n",
	})

	client := newTenancyClient(t)
	ctx := context.Background()
	repo, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(dir)})
	require.NoError(t, err)
	waitForTasks(ctx, t, client, 60*time.Second)

	sha := latestSHA(t, client, repo.ID())
	assert.NotEqual(t, head, sha, "the directory must be indexed as a snapshot, not as the crafted git history")
	files, err := client.Files.Find(ctx, repository.WithCommitSHA(sha))
	require.NoError(t, err)
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path())
	}
	assert.Contains(t, paths, "objects/info/alternates")
	assert.Contains(t, paths, "notes.md")
	assert.NotContains(t, paths, "secret.txt")

	enrichments, err := client.Enrichments.List(ctx, &service.EnrichmentListParams{CommitSHA: sha})
	require.NoError(t, err)
	for _, e := range enrichments {
		assert.NotContains(t, e.Content(), secret)
	}
	for _, text := range keywordSearch(t, client, repo, secret) {
		assert.NotContains(t, text, secret)
	}
	mcpClient := startMCP(t, kodit.NewMCPHandler(client))
	text, _ := callTool(t, mcpClient, "kodit_read_resource", map[string]any{
		"uri": fmt.Sprintf("file://%d/%s/secret.txt", repo.ID(), head),
	})
	assert.NotContains(t, text, secret)

	_, statErr := os.Stat(marker)
	assert.True(t, os.IsNotExist(statErr), "a command from the directory's git configuration ran")
}

// TestIntegration_ForeignCommitSHA_SharesNothing covers a repository naming a
// commit SHA it does not hold: by rescanning it, or by tracking it as a
// commit. Repositories only share indexed content for commits their own
// history holds, so the repository never gains the other repository's
// results.
func TestIntegration_ForeignCommitSHA_SharesNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	pathA := createTestGitRepo(t)
	dirB := filepath.Join(t.TempDir(), "folder-b")
	writeFiles(t, dirB, map[string]string{"guide.md": "# Unrelated\n\nnothing here\n"})
	dirC := filepath.Join(t.TempDir(), "folder-c")
	writeFiles(t, dirC, map[string]string{"guide.md": "# Also unrelated\n\nnothing here either\n"})
	pathD := createTestGitRepoWithDocuments(t) // a git repository without A's commit

	client := newTenancyClient(t, kodit.WithLocalGitRepositories())
	ctx := context.Background()
	repoA, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(pathA), Branch: currentBranch(t, pathA)})
	require.NoError(t, err)
	repoB, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(dirB)})
	require.NoError(t, err)
	repoC, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(dirC)})
	require.NoError(t, err)
	repoD, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(pathD), Branch: currentBranch(t, pathD)})
	require.NoError(t, err)
	waitForTasks(ctx, t, client, 60*time.Second)
	shaA := latestSHA(t, client, repoA.ID())

	// Rescanning a commit the repository does not hold is refused before
	// any task is enqueued.
	require.ErrorIs(t, client.Repositories.Rescan(ctx, &service.RescanParams{RepositoryID: repoB.ID(), CommitSHA: shaA}), database.ErrNotFound)
	waitForTasks(ctx, t, client, 60*time.Second)

	// Tracking the other repository's commit runs the pipeline for that SHA;
	// indexing must not share A's content with a repository that does not
	// hold the commit, whether it is a directory or a git repository.
	for _, repo := range []repository.Source{repoC, repoD} {
		_, err := client.Repositories.UpdateTrackingConfig(ctx, repo.ID(), &service.TrackingConfigParams{Commit: shaA})
		require.NoError(t, err)
		require.NoError(t, client.Repositories.Sync(ctx, repo.ID()))
	}
	waitForTasks(ctx, t, client, 60*time.Second)

	for _, repo := range []repository.Source{repoB, repoC, repoD} {
		for _, text := range keywordSearch(t, client, repo, "Subtract") {
			assert.NotContains(t, text, "main.go", "repository %d gained another repository's results", repo.ID())
		}
	}
	for _, text := range keywordSearch(t, client, repoA, "Subtract") {
		assert.Contains(t, text, "main.go", "the owner keeps its results")
	}
}
