package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/internal/database"
)

type blobTestDeps struct {
	blob   *Blob
	git    *fakeGitAdapter
	stores testStores
}

func newBlobTestDeps(t *testing.T) blobTestDeps {
	t.Helper()
	stores := newTestStores(t)
	gitAdapter := &fakeGitAdapter{content: map[string][]byte{}}
	blob := NewBlob(stores.repos, stores.commits, stores.tags, stores.branches, gitAdapter)
	return blobTestDeps{blob: blob, git: gitAdapter, stores: stores}
}

// seedBlobFixtures creates a repo with working copy, commit, tag, and branch,
// and configures the git adapter with file content for each.
func seedBlobFixtures(t *testing.T, deps blobTestDeps) int64 {
	t.Helper()
	ctx := context.Background()

	remoteURL := "https://github.com/example/repo"
	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy("/tmp/test-repo", remoteURL))
	saved, err := deps.stores.repos.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repo: %v", err)
	}

	now := time.Now()
	commit := repository.NewCommit(
		"abc1234567890def", saved.ID(), "initial commit",
		repository.NewAuthor("Test", "test@test.com"),
		repository.NewAuthor("Test", "test@test.com"),
		now, now,
	)
	if _, err := deps.stores.commits.Save(ctx, commit); err != nil {
		t.Fatalf("save commit: %v", err)
	}

	tag := repository.NewTag(saved.ID(), "v1.0.0", "def4567890abcdef")
	if _, err := deps.stores.tags.Save(ctx, tag); err != nil {
		t.Fatalf("save tag: %v", err)
	}

	branch := repository.NewBranch(saved.ID(), "main", "aaa1111222233334444", true)
	if _, err := deps.stores.branches.Save(ctx, branch); err != nil {
		t.Fatalf("save branch: %v", err)
	}

	deps.git.content = map[string][]byte{
		"abc1234567890def:README.md":    []byte("# Hello\nWorld"),
		"def4567890abcdef:README.md":    []byte("# Tagged\nContent"),
		"aaa1111222233334444:README.md": []byte("# Branch\nContent"),
	}

	return saved.ID()
}

func TestBlob_ResolveCommitSHA(t *testing.T) {
	deps := newBlobTestDeps(t)
	repoID := seedBlobFixtures(t, deps)

	sha, err := deps.blob.Resolve(context.Background(), repoID, "abc1234567890def")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "abc1234567890def" {
		t.Errorf("expected abc1234567890def, got %s", sha)
	}
}

func TestBlob_ResolveTag(t *testing.T) {
	deps := newBlobTestDeps(t)
	repoID := seedBlobFixtures(t, deps)

	sha, err := deps.blob.Resolve(context.Background(), repoID, "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "def4567890abcdef" {
		t.Errorf("expected def4567890abcdef, got %s", sha)
	}
}

func TestBlob_ResolveBranch(t *testing.T) {
	deps := newBlobTestDeps(t)
	repoID := seedBlobFixtures(t, deps)

	sha, err := deps.blob.Resolve(context.Background(), repoID, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "aaa1111222233334444" {
		t.Errorf("expected aaa1111222233334444, got %s", sha)
	}
}

func TestBlob_ResolveNotFound(t *testing.T) {
	deps := newBlobTestDeps(t)
	repoID := seedBlobFixtures(t, deps)

	_, err := deps.blob.Resolve(context.Background(), repoID, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent blob")
	}
}

func TestBlob_ContentByCommitSHA(t *testing.T) {
	deps := newBlobTestDeps(t)
	repoID := seedBlobFixtures(t, deps)

	result, err := deps.blob.Content(context.Background(), repoID, "abc1234567890def", "README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Content()) != "# Hello\nWorld" {
		t.Errorf("expected '# Hello\\nWorld', got %q", string(result.Content()))
	}
	if result.CommitSHA() != "abc1234567890def" {
		t.Errorf("expected commit SHA abc1234567890def, got %s", result.CommitSHA())
	}
}

func TestBlob_ContentByTag(t *testing.T) {
	deps := newBlobTestDeps(t)
	repoID := seedBlobFixtures(t, deps)

	result, err := deps.blob.Content(context.Background(), repoID, "v1.0.0", "README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Content()) != "# Tagged\nContent" {
		t.Errorf("unexpected content: %q", string(result.Content()))
	}
	if result.CommitSHA() != "def4567890abcdef" {
		t.Errorf("expected commit SHA def4567890abcdef, got %s", result.CommitSHA())
	}
}

func TestBlob_ContentByBranch(t *testing.T) {
	deps := newBlobTestDeps(t)
	repoID := seedBlobFixtures(t, deps)

	result, err := deps.blob.Content(context.Background(), repoID, "main", "README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Content()) != "# Branch\nContent" {
		t.Errorf("unexpected content: %q", string(result.Content()))
	}
}

func TestBlob_ContentFileNotFound(t *testing.T) {
	deps := newBlobTestDeps(t)
	repoID := seedBlobFixtures(t, deps)

	_, err := deps.blob.Content(context.Background(), repoID, "main", "nonexistent.go")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected database.ErrNotFound, got: %v", err)
	}
}

func TestBlob_ListFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[core]"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := newBlobTestDeps(t)
	ctx := context.Background()

	remoteURL := "https://github.com/example/repo"
	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy(dir, remoteURL))
	saved, err := deps.stores.repos.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repo: %v", err)
	}

	t.Run("all files", func(t *testing.T) {
		entries, err := deps.blob.ListFiles(ctx, saved.ID(), "**")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
	})

	t.Run("glob pattern", func(t *testing.T) {
		entries, err := deps.blob.ListFiles(ctx, saved.ID(), "**/*.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Path != "src/main.go" {
			t.Errorf("expected src/main.go, got %s", entries[0].Path)
		}
	})

	t.Run("skips .git", func(t *testing.T) {
		entries, err := deps.blob.ListFiles(ctx, saved.ID(), "*")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, e := range entries {
			if filepath.Base(e.Path) == "config" {
				t.Error("expected .git directory to be skipped")
			}
		}
	})
}

func TestBlob_ListFiles_ClonesWhenMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")

	deps := newBlobTestDeps(t)
	ctx := context.Background()

	remoteURL := "https://github.com/example/repo"
	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy(dir, remoteURL))
	saved, err := deps.stores.repos.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repo: %v", err)
	}

	deps.git.cloneFn = func(_, localPath string) error {
		if err := os.MkdirAll(filepath.Join(localPath, "src"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(localPath, "README.md"), []byte("# Hello"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(localPath, "src", "main.go"), []byte("package main"), 0o644)
	}

	entries, err := deps.blob.ListFiles(ctx, saved.ID(), "**/*.go")
	if err != nil {
		t.Fatalf("expected ListFiles to succeed after cloning, got: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "src/main.go" {
		t.Errorf("expected src/main.go, got %s", entries[0].Path)
	}
	if !deps.git.cloneCalled {
		t.Error("expected CloneRepository to be called")
	}
}

func TestBlob_ListFilesForCommit(t *testing.T) {
	dir := t.TempDir()

	deps := newBlobTestDeps(t)
	ctx := context.Background()

	remoteURL := "https://github.com/example/repo"
	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy(dir, remoteURL))
	saved, err := deps.stores.repos.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repo: %v", err)
	}

	deps.git.commitFiles = []git.FileInfo{
		{Path: "README.md", BlobSHA: "aaa", Size: 100},
		{Path: "src/main.go", BlobSHA: "bbb", Size: 200},
		{Path: "src/util.go", BlobSHA: "ccc", Size: 150},
		{Path: "docs/guide.txt", BlobSHA: "ddd", Size: 50},
	}

	t.Run("all files", func(t *testing.T) {
		entries, err := deps.blob.ListFilesForCommit(ctx, saved.ID(), "abc123", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 4 {
			t.Fatalf("expected 4 entries, got %d", len(entries))
		}
	})

	t.Run("glob filter", func(t *testing.T) {
		entries, err := deps.blob.ListFilesForCommit(ctx, saved.ID(), "abc123", "**/*.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 .go entries, got %d", len(entries))
		}
		for _, e := range entries {
			ext := filepath.Ext(e.Path)
			if ext != ".go" {
				t.Errorf("expected .go file, got %s", e.Path)
			}
		}
	})

	t.Run("preserves blob SHA", func(t *testing.T) {
		entries, err := deps.blob.ListFilesForCommit(ctx, saved.ID(), "abc123", "**")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entries[0].BlobSHA != "aaa" {
			t.Errorf("expected BlobSHA aaa, got %s", entries[0].BlobSHA)
		}
	})
}

func TestBlob_ListFilesForCommit_ClonesWhenMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")

	deps := newBlobTestDeps(t)
	ctx := context.Background()

	remoteURL := "https://github.com/example/repo"
	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy(dir, remoteURL))
	saved, err := deps.stores.repos.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repo: %v", err)
	}

	deps.git.commitFiles = []git.FileInfo{
		{Path: "main.go", BlobSHA: "abc", Size: 100},
	}
	deps.git.cloneFn = func(_, localPath string) error {
		return os.MkdirAll(localPath, 0o755)
	}

	entries, err := deps.blob.ListFilesForCommit(ctx, saved.ID(), "abc123", "")
	if err != nil {
		t.Fatalf("expected ListFilesForCommit to succeed after cloning, got: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "main.go" {
		t.Errorf("expected main.go, got %s", entries[0].Path)
	}
	if !deps.git.cloneCalled {
		t.Error("expected CloneRepository to be called")
	}
}
