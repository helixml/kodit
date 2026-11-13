package kodit_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/repository"
)

// TestIntegration_WorkingCopySymlinks_AreNotFollowed indexes a git repository
// and a plain directory whose files are symbolic links to another indexed
// repository's documents, to /etc/hostname and to /proc/self/environ. Neither
// indexing (chunks, which back search previews) nor raster and text reads
// through MCP may expose the link targets' content.
func TestIntegration_WorkingCopySymlinks_AreNotFollowed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	const docText = "Installation guide"
	const outsideText = "outside-working-copy-notes"
	// /proc/self/environ holds the environment the process started with, so
	// its PATH entry is the marker for that link.
	forbidden := []string{docText, outsideText, "PATH=" + os.Getenv("PATH")}
	if hostname, err := os.ReadFile("/etc/hostname"); err == nil && strings.TrimSpace(string(hostname)) != "" {
		forbidden = append(forbidden, strings.TrimSpace(string(hostname)))
	}

	docsPath := createTestGitRepoWithDocuments(t) // the other repository
	outside := filepath.Join(t.TempDir(), "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "notes.md"), []byte("# "+outsideText+"\n\n"+outsideText+" body text"), 0o644))

	links := map[string]string{
		"docs/leak.pdf":    filepath.Join(docsPath, "docs", "guide.pdf"),
		"docs/leak.docx":   filepath.Join(docsPath, "docs", "readme.docx"),
		"docs/environ.pdf": "/proc/self/environ",
		"docs/host.png":    "/etc/hostname",
		"notes.md":         filepath.Join(outside, "notes.md"),
		"environ.md":       "/proc/self/environ",
		"hostname.md":      "/etc/hostname",
		"README.md":        "/proc/self/environ",
		"linked":           filepath.Join(docsPath, "docs"),
	}
	writeTree := func(dir string, skip ...string) {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
		for name, target := range links {
			if slices.Contains(skip, name) {
				continue
			}
			require.NoError(t, os.Symlink(target, filepath.Join(dir, name)))
		}
	}

	gitPath := filepath.Join(t.TempDir(), "symlink-repo")
	writeTree(gitPath)
	for _, args := range [][]string{
		{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"},
		{"add", "-A"}, {"commit", "-m", "symlinks"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = gitPath
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	// A symlinked directory is covered by the git repository; leaving it out
	// here keeps the plain directory indexable by earlier releases too, so
	// this case shows its file links on their own.
	plainPath := filepath.Join(t.TempDir(), "symlink-dir")
	writeTree(plainPath, "linked")

	tmpDir := t.TempDir()
	client, err := kodit.New(
		kodit.WithSQLite(filepath.Join(tmpDir, "test.db")),
		kodit.WithDataDir(filepath.Join(tmpDir, "data")),
		kodit.WithCloneDir(filepath.Join(tmpDir, "repos")),
		kodit.WithModelDir("infrastructure/provider/models"),
		kodit.WithWorkerPollPeriod(testPollPeriod),
		kodit.WithLocalGitRepositories(),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	_, _, err = client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(docsPath), Branch: currentBranch(t, docsPath)})
	require.NoError(t, err)
	gitRepo, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(gitPath), Branch: currentBranch(t, gitPath)})
	require.NoError(t, err)
	plainRepo, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(plainPath)})
	require.NoError(t, err)
	waitForTasks(ctx, t, client, 120*time.Second)

	mcpClient := startMCP(t, kodit.NewMCPHandler(client))

	for _, repo := range []repository.Source{gitRepo, plainRepo} {
		commits, err := client.Commits.Find(ctx, repository.WithRepoID(repo.ID()))
		require.NoError(t, err)
		require.NotEmpty(t, commits, "repository %d was not indexed", repo.ID())
		sha := commits[0].SHA()

		t.Run(fmt.Sprintf("repo %d indexing", repo.ID()), func(t *testing.T) {
			enrichments, err := client.Enrichments.List(ctx, &service.EnrichmentListParams{CommitSHA: sha})
			require.NoError(t, err)
			for _, e := range enrichments {
				for _, f := range forbidden {
					assert.NotContains(t, e.Content(), f, "%s/%s enrichment followed a symbolic link", e.Type(), e.Subtype())
				}
			}
		})

		for _, path := range []string{"docs/leak.pdf", "docs/leak.docx", "docs/environ.pdf", "docs/host.png", "linked/guide.pdf"} {
			for _, query := range []string{"?mode=text", "?mode=text&page=1", "?mode=raster&page=1"} {
				uri := fmt.Sprintf("file://%d/%s/%s%s", repo.ID(), sha, path, query)
				t.Run(uri, func(t *testing.T) {
					text, isErr := callTool(t, mcpClient, "kodit_read_resource", map[string]any{"uri": uri})
					assert.True(t, isErr, "read through a symbolic link succeeded: %s", text)
					for _, f := range forbidden {
						assert.NotContains(t, text, f)
					}
				})
			}
		}

		for _, path := range []string{"notes.md", "environ.md", "hostname.md", "README.md"} {
			uri := fmt.Sprintf("file://%d/%s/%s", repo.ID(), sha, path)
			t.Run(uri, func(t *testing.T) {
				text, _ := callTool(t, mcpClient, "kodit_read_resource", map[string]any{"uri": uri})
				for _, f := range forbidden {
					assert.NotContains(t, text, f)
				}
			})
		}

		t.Run(fmt.Sprintf("repo %d keyword search", repo.ID()), func(t *testing.T) {
			for _, keywords := range []string{"Installation guide", outsideText} {
				text, _ := callTool(t, mcpClient, "kodit_keyword_search", map[string]any{
					"keywords": keywords, "source_repo": repo.Repo().UpstreamURL(),
				})
				for _, f := range forbidden {
					assert.NotContains(t, text, f)
				}
			}
		})
	}
}
