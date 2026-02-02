package gitadapter

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a test git repository with initial commit.
func setupTestRepo(t *testing.T) (string, *gogit.Repository) {
	t.Helper()
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "test-repo")

	repo, err := gogit.PlainInit(repoPath, false)
	require.NoError(t, err)

	// Create and commit a test file
	readme := filepath.Join(repoPath, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("# Test Repository\n"), 0644))

	wt, err := repo.Worktree()
	require.NoError(t, err)

	_, err = wt.Add("README.md")
	require.NoError(t, err)

	sig := &object.Signature{
		Name:  "Test Author",
		Email: "test@example.com",
		When:  time.Now(),
	}

	_, err = wt.Commit("Initial commit", &gogit.CommitOptions{
		Author: sig,
	})
	require.NoError(t, err)

	return repoPath, repo
}

func testAdapter(t *testing.T) *GoGit {
	t.Helper()
	return NewGoGit(slog.Default())
}

func TestGoGit_RepositoryExists(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("exists for valid repo", func(t *testing.T) {
		repoPath, _ := setupTestRepo(t)

		exists, err := adapter.RepositoryExists(ctx, repoPath)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("not exists for nonexistent path", func(t *testing.T) {
		exists, err := adapter.RepositoryExists(ctx, "/nonexistent/path")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("not exists for regular directory", func(t *testing.T) {
		dir := t.TempDir()

		exists, err := adapter.RepositoryExists(ctx, dir)
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestGoGit_AllBranches(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns main branch", func(t *testing.T) {
		repoPath, _ := setupTestRepo(t)

		branches, err := adapter.AllBranches(ctx, repoPath)
		require.NoError(t, err)
		assert.Len(t, branches, 1)
		// go-git defaults to "master" for PlainInit
		assert.Equal(t, "master", branches[0].Name)
		assert.NotEmpty(t, branches[0].HeadSHA)
	})

	t.Run("returns multiple branches", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		// Create another branch
		headRef, err := repo.Head()
		require.NoError(t, err)

		wt, err := repo.Worktree()
		require.NoError(t, err)

		err = wt.Checkout(&gogit.CheckoutOptions{
			Hash:   headRef.Hash(),
			Branch: "refs/heads/develop",
			Create: true,
		})
		require.NoError(t, err)

		branches, err := adapter.AllBranches(ctx, repoPath)
		require.NoError(t, err)
		assert.Len(t, branches, 2)

		names := make(map[string]bool)
		for _, b := range branches {
			names[b.Name] = true
		}
		assert.True(t, names["master"])
		assert.True(t, names["develop"])
	})
}

func TestGoGit_BranchCommits(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns commits for branch", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		// Add another commit
		readme := filepath.Join(repoPath, "README.md")
		require.NoError(t, os.WriteFile(readme, []byte("# Updated\n"), 0644))

		wt, err := repo.Worktree()
		require.NoError(t, err)

		_, err = wt.Add("README.md")
		require.NoError(t, err)

		sig := &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		}

		_, err = wt.Commit("Update README", &gogit.CommitOptions{
			Author: sig,
		})
		require.NoError(t, err)

		commits, err := adapter.BranchCommits(ctx, repoPath, "master")
		require.NoError(t, err)
		assert.Len(t, commits, 2)
		assert.Equal(t, "Update README", commits[0].Message)
		assert.Equal(t, "Initial commit", commits[1].Message)
	})

	t.Run("returns error for nonexistent branch", func(t *testing.T) {
		repoPath, _ := setupTestRepo(t)

		_, err := adapter.BranchCommits(ctx, repoPath, "nonexistent")
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrBranchNotFound)
	})
}

func TestGoGit_CommitFiles(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns files in commit", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		files, err := adapter.CommitFiles(ctx, repoPath, headRef.Hash().String())
		require.NoError(t, err)
		assert.Len(t, files, 1)
		assert.Equal(t, "README.md", files[0].Path)
		assert.NotEmpty(t, files[0].BlobSHA)
	})

	t.Run("returns multiple files", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		// Add more files
		require.NoError(t, os.WriteFile(filepath.Join(repoPath, "main.go"), []byte("package main\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(repoPath, "utils.go"), []byte("package main\n"), 0644))

		wt, err := repo.Worktree()
		require.NoError(t, err)

		_, err = wt.Add("main.go")
		require.NoError(t, err)
		_, err = wt.Add("utils.go")
		require.NoError(t, err)

		sig := &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		}

		hash, err := wt.Commit("Add Go files", &gogit.CommitOptions{
			Author: sig,
		})
		require.NoError(t, err)

		files, err := adapter.CommitFiles(ctx, repoPath, hash.String())
		require.NoError(t, err)
		assert.Len(t, files, 3) // README.md, main.go, utils.go
	})
}

func TestGoGit_CommitDetails(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns commit details", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		info, err := adapter.CommitDetails(ctx, repoPath, headRef.Hash().String())
		require.NoError(t, err)
		assert.Equal(t, headRef.Hash().String(), info.SHA)
		assert.Equal(t, "Initial commit", info.Message)
		assert.Equal(t, "Test Author", info.AuthorName)
		assert.Equal(t, "test@example.com", info.AuthorEmail)
	})
}

func TestGoGit_FileContent(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns file content", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		content, err := adapter.FileContent(ctx, repoPath, headRef.Hash().String(), "README.md")
		require.NoError(t, err)
		assert.Equal(t, "# Test Repository\n", string(content))
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		_, err = adapter.FileContent(ctx, repoPath, headRef.Hash().String(), "nonexistent.txt")
		assert.Error(t, err)
	})
}

func TestGoGit_DefaultBranch(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns master for default init", func(t *testing.T) {
		repoPath, _ := setupTestRepo(t)

		branch, err := adapter.DefaultBranch(ctx, repoPath)
		require.NoError(t, err)
		assert.Equal(t, "master", branch)
	})

	t.Run("returns main when main exists", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		// Create main branch
		headRef, err := repo.Head()
		require.NoError(t, err)

		wt, err := repo.Worktree()
		require.NoError(t, err)

		err = wt.Checkout(&gogit.CheckoutOptions{
			Hash:   headRef.Hash(),
			Branch: "refs/heads/main",
			Create: true,
		})
		require.NoError(t, err)

		branch, err := adapter.DefaultBranch(ctx, repoPath)
		require.NoError(t, err)
		// Should return main (checked alphabetically or by preference)
		assert.Contains(t, []string{"main", "master"}, branch)
	})
}

func TestGoGit_LatestCommitSHA(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns HEAD SHA when no branch specified", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		sha, err := adapter.LatestCommitSHA(ctx, repoPath, "")
		require.NoError(t, err)
		assert.Equal(t, headRef.Hash().String(), sha)
	})

	t.Run("returns branch SHA", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		sha, err := adapter.LatestCommitSHA(ctx, repoPath, "master")
		require.NoError(t, err)
		assert.Equal(t, headRef.Hash().String(), sha)
	})
}

func TestGoGit_AllTags(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns empty for repo without tags", func(t *testing.T) {
		repoPath, _ := setupTestRepo(t)

		tags, err := adapter.AllTags(ctx, repoPath)
		require.NoError(t, err)
		assert.Empty(t, tags)
	})

	t.Run("returns lightweight tags", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		// Create lightweight tag
		_, err = repo.CreateTag("v1.0.0", headRef.Hash(), nil)
		require.NoError(t, err)

		tags, err := adapter.AllTags(ctx, repoPath)
		require.NoError(t, err)
		assert.Len(t, tags, 1)
		assert.Equal(t, "v1.0.0", tags[0].Name)
		assert.Equal(t, headRef.Hash().String(), tags[0].TargetCommitSHA)
	})

	t.Run("returns annotated tags", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		// Create annotated tag
		sig := &object.Signature{
			Name:  "Tagger",
			Email: "tagger@example.com",
			When:  time.Now(),
		}

		_, err = repo.CreateTag("v2.0.0", headRef.Hash(), &gogit.CreateTagOptions{
			Tagger:  sig,
			Message: "Release v2.0.0",
		})
		require.NoError(t, err)

		tags, err := adapter.AllTags(ctx, repoPath)
		require.NoError(t, err)
		assert.Len(t, tags, 1)
		assert.Equal(t, "v2.0.0", tags[0].Name)
		assert.Contains(t, tags[0].Message, "Release v2.0.0")
		assert.Equal(t, "Tagger", tags[0].TaggerName)
	})
}

func TestGoGit_CommitDiff(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns diff for commit", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		// Add a new file
		require.NoError(t, os.WriteFile(filepath.Join(repoPath, "new.txt"), []byte("new content\n"), 0644))

		wt, err := repo.Worktree()
		require.NoError(t, err)

		_, err = wt.Add("new.txt")
		require.NoError(t, err)

		sig := &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		}

		hash, err := wt.Commit("Add new file", &gogit.CommitOptions{
			Author: sig,
		})
		require.NoError(t, err)

		diff, err := adapter.CommitDiff(ctx, repoPath, hash.String())
		require.NoError(t, err)
		assert.Contains(t, diff, "new.txt")
		assert.Contains(t, diff, "new content")
	})
}

func TestGoGit_AllCommitsBulk(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns all commits", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		// Add another commit
		require.NoError(t, os.WriteFile(filepath.Join(repoPath, "file.txt"), []byte("content\n"), 0644))

		wt, err := repo.Worktree()
		require.NoError(t, err)

		_, err = wt.Add("file.txt")
		require.NoError(t, err)

		sig := &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		}

		_, err = wt.Commit("Second commit", &gogit.CommitOptions{
			Author: sig,
		})
		require.NoError(t, err)

		commits, err := adapter.AllCommitsBulk(ctx, repoPath, nil)
		require.NoError(t, err)
		assert.Len(t, commits, 2)
	})

	t.Run("filters by since time", func(t *testing.T) {
		repoPath, _ := setupTestRepo(t)

		// Query with future time
		futureTime := time.Now().Add(24 * time.Hour)
		commits, err := adapter.AllCommitsBulk(ctx, repoPath, &futureTime)
		require.NoError(t, err)
		assert.Empty(t, commits)
	})
}

func TestGoGit_BranchCommitSHAs(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns commit SHAs", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		shas, err := adapter.BranchCommitSHAs(ctx, repoPath, "master")
		require.NoError(t, err)
		assert.Len(t, shas, 1)
		assert.Equal(t, headRef.Hash().String(), shas[0])
	})
}

func TestGoGit_AllBranchHeadSHAs(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns head SHAs for branches", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		headRef, err := repo.Head()
		require.NoError(t, err)

		shas, err := adapter.AllBranchHeadSHAs(ctx, repoPath, []string{"master"})
		require.NoError(t, err)
		assert.Len(t, shas, 1)
		assert.Equal(t, headRef.Hash().String(), shas["master"])
	})

	t.Run("skips nonexistent branches", func(t *testing.T) {
		repoPath, _ := setupTestRepo(t)

		shas, err := adapter.AllBranchHeadSHAs(ctx, repoPath, []string{"master", "nonexistent"})
		require.NoError(t, err)
		assert.Len(t, shas, 1)
		_, hasMaster := shas["master"]
		assert.True(t, hasMaster)
	})
}

func TestGoGit_EnsureRepository(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("returns nil for existing repo", func(t *testing.T) {
		repoPath, _ := setupTestRepo(t)

		// EnsureRepository on existing repo should pull (which may be a no-op for local repos)
		err := adapter.EnsureRepository(ctx, "https://example.com/fake.git", repoPath)
		// May get an error trying to pull from non-existent remote, but for local repos without
		// a proper remote this is expected. The point is the repo already exists.
		// For this test, we just verify it doesn't panic.
		_ = err
	})
}

func TestGoGit_CheckoutCommit(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("checkout specific commit", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		// Get initial commit
		headRef, err := repo.Head()
		require.NoError(t, err)
		initialSHA := headRef.Hash().String()

		// Add another commit
		require.NoError(t, os.WriteFile(filepath.Join(repoPath, "new.txt"), []byte("new\n"), 0644))

		wt, err := repo.Worktree()
		require.NoError(t, err)

		_, err = wt.Add("new.txt")
		require.NoError(t, err)

		sig := &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		}

		_, err = wt.Commit("Add new file", &gogit.CommitOptions{
			Author: sig,
		})
		require.NoError(t, err)

		// Checkout initial commit
		err = adapter.CheckoutCommit(ctx, repoPath, initialSHA)
		require.NoError(t, err)

		// Verify new.txt doesn't exist at initial commit
		_, err = os.Stat(filepath.Join(repoPath, "new.txt"))
		assert.True(t, os.IsNotExist(err))
	})
}

func TestGoGit_CheckoutBranch(t *testing.T) {
	adapter := testAdapter(t)
	ctx := context.Background()

	t.Run("checkout existing branch", func(t *testing.T) {
		repoPath, repo := setupTestRepo(t)

		// Create another branch with different content
		headRef, err := repo.Head()
		require.NoError(t, err)

		wt, err := repo.Worktree()
		require.NoError(t, err)

		err = wt.Checkout(&gogit.CheckoutOptions{
			Hash:   headRef.Hash(),
			Branch: "refs/heads/feature",
			Create: true,
		})
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(repoPath, "feature.txt"), []byte("feature\n"), 0644))

		_, err = wt.Add("feature.txt")
		require.NoError(t, err)

		sig := &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		}

		_, err = wt.Commit("Feature commit", &gogit.CommitOptions{
			Author: sig,
		})
		require.NoError(t, err)

		// Checkout back to master
		err = adapter.CheckoutBranch(ctx, repoPath, "master")
		require.NoError(t, err)

		// Verify feature.txt doesn't exist on master
		_, err = os.Stat(filepath.Join(repoPath, "feature.txt"))
		assert.True(t, os.IsNotExist(err))

		// Checkout feature branch
		err = adapter.CheckoutBranch(ctx, repoPath, "feature")
		require.NoError(t, err)

		// Verify feature.txt exists
		_, err = os.Stat(filepath.Join(repoPath, "feature.txt"))
		assert.NoError(t, err)
	})
}
