package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/internal/database"
)

type fakeBlobCommitStore struct {
	commits []repository.Commit
}

func (f *fakeBlobCommitStore) Find(_ context.Context, opts ...repository.Option) ([]repository.Commit, error) {
	return f.commits, nil
}

func (f *fakeBlobCommitStore) FindOne(_ context.Context, _ ...repository.Option) (repository.Commit, error) {
	if len(f.commits) > 0 {
		return f.commits[0], nil
	}
	return repository.Commit{}, fmt.Errorf("not found")
}

func (f *fakeBlobCommitStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	return int64(len(f.commits)), nil
}

func (f *fakeBlobCommitStore) Save(_ context.Context, c repository.Commit) (repository.Commit, error) {
	return c, nil
}

func (f *fakeBlobCommitStore) Delete(_ context.Context, _ repository.Commit) error { return nil }

func (f *fakeBlobCommitStore) Exists(_ context.Context, opts ...repository.Option) (bool, error) {
	q := repository.Build(opts...)
	for _, cond := range q.Conditions() {
		if cond.Field() == "commit_sha" {
			sha, ok := cond.Value().(string)
			if !ok {
				continue
			}
			for _, c := range f.commits {
				if c.SHA() == sha {
					return true, nil
				}
			}
			return false, nil
		}
	}
	return false, nil
}

func (f *fakeBlobCommitStore) SaveAll(_ context.Context, commits []repository.Commit) ([]repository.Commit, error) {
	return commits, nil
}

type fakeBlobTagStore struct {
	tags []repository.Tag
}

func (f *fakeBlobTagStore) Find(_ context.Context, opts ...repository.Option) ([]repository.Tag, error) {
	q := repository.Build(opts...)
	for _, cond := range q.Conditions() {
		if cond.Field() == "name" {
			name, ok := cond.Value().(string)
			if !ok {
				continue
			}
			for _, t := range f.tags {
				if t.Name() == name {
					return []repository.Tag{t}, nil
				}
			}
			return nil, nil
		}
	}
	return f.tags, nil
}

func (f *fakeBlobTagStore) FindOne(_ context.Context, _ ...repository.Option) (repository.Tag, error) {
	if len(f.tags) > 0 {
		return f.tags[0], nil
	}
	return repository.Tag{}, fmt.Errorf("not found")
}

func (f *fakeBlobTagStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	return int64(len(f.tags)), nil
}

func (f *fakeBlobTagStore) Save(_ context.Context, t repository.Tag) (repository.Tag, error) {
	return t, nil
}

func (f *fakeBlobTagStore) Delete(_ context.Context, _ repository.Tag) error { return nil }

func (f *fakeBlobTagStore) SaveAll(_ context.Context, tags []repository.Tag) ([]repository.Tag, error) {
	return tags, nil
}

type fakeBlobBranchStore struct {
	branches []repository.Branch
}

func (f *fakeBlobBranchStore) Find(_ context.Context, opts ...repository.Option) ([]repository.Branch, error) {
	q := repository.Build(opts...)
	for _, cond := range q.Conditions() {
		if cond.Field() == "name" {
			name, ok := cond.Value().(string)
			if !ok {
				continue
			}
			for _, b := range f.branches {
				if b.Name() == name {
					return []repository.Branch{b}, nil
				}
			}
			return nil, nil
		}
	}
	return f.branches, nil
}

func (f *fakeBlobBranchStore) FindOne(_ context.Context, _ ...repository.Option) (repository.Branch, error) {
	if len(f.branches) > 0 {
		return f.branches[0], nil
	}
	return repository.Branch{}, fmt.Errorf("not found")
}

func (f *fakeBlobBranchStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	return int64(len(f.branches)), nil
}

func (f *fakeBlobBranchStore) Save(_ context.Context, b repository.Branch) (repository.Branch, error) {
	return b, nil
}

func (f *fakeBlobBranchStore) Delete(_ context.Context, _ repository.Branch) error { return nil }

func (f *fakeBlobBranchStore) SaveAll(_ context.Context, branches []repository.Branch) ([]repository.Branch, error) {
	return branches, nil
}

type fakeBlobRepoStore struct {
	repo repository.Repository
}

func (f *fakeBlobRepoStore) Find(_ context.Context, _ ...repository.Option) ([]repository.Repository, error) {
	return []repository.Repository{f.repo}, nil
}

func (f *fakeBlobRepoStore) FindOne(_ context.Context, _ ...repository.Option) (repository.Repository, error) {
	return f.repo, nil
}

func (f *fakeBlobRepoStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	return 1, nil
}

func (f *fakeBlobRepoStore) Save(_ context.Context, r repository.Repository) (repository.Repository, error) {
	return r, nil
}

func (f *fakeBlobRepoStore) Delete(_ context.Context, _ repository.Repository) error { return nil }

func (f *fakeBlobRepoStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return true, nil
}

type fakeBlobGitAdapter struct {
	content     map[string][]byte
	cloneFn     func(remoteURI, localPath string) error
	cloneCalled bool
}

func (f *fakeBlobGitAdapter) FileContent(_ context.Context, _, commitSHA, filePath string) ([]byte, error) {
	key := commitSHA + ":" + filePath
	content, ok := f.content[key]
	if !ok {
		return nil, fmt.Errorf("get file: %w", git.ErrFileNotFound)
	}
	return content, nil
}

func (f *fakeBlobGitAdapter) CloneRepository(_ context.Context, remoteURI, localPath string) error {
	f.cloneCalled = true
	if f.cloneFn != nil {
		return f.cloneFn(remoteURI, localPath)
	}
	return nil
}
func (f *fakeBlobGitAdapter) CheckoutCommit(context.Context, string, string) error { return nil }
func (f *fakeBlobGitAdapter) CheckoutBranch(context.Context, string, string) error { return nil }
func (f *fakeBlobGitAdapter) FetchRepository(context.Context, string) error        { return nil }
func (f *fakeBlobGitAdapter) PullRepository(context.Context, string) error         { return nil }
func (f *fakeBlobGitAdapter) AllBranches(context.Context, string) ([]git.BranchInfo, error) {
	return nil, nil
}
func (f *fakeBlobGitAdapter) BranchCommits(context.Context, string, string) ([]git.CommitInfo, error) {
	return nil, nil
}
func (f *fakeBlobGitAdapter) AllCommitsBulk(context.Context, string, *time.Time) (map[string]git.CommitInfo, error) {
	return nil, nil
}
func (f *fakeBlobGitAdapter) BranchCommitSHAs(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (f *fakeBlobGitAdapter) AllBranchHeadSHAs(context.Context, string, []string) (map[string]string, error) {
	return nil, nil
}
func (f *fakeBlobGitAdapter) CommitFiles(context.Context, string, string) ([]git.FileInfo, error) {
	return nil, nil
}
func (f *fakeBlobGitAdapter) RepositoryExists(_ context.Context, localPath string) (bool, error) {
	_, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
func (f *fakeBlobGitAdapter) CommitDetails(context.Context, string, string) (git.CommitInfo, error) {
	return git.CommitInfo{}, nil
}
func (f *fakeBlobGitAdapter) EnsureRepository(context.Context, string, string) error { return nil }
func (f *fakeBlobGitAdapter) DefaultBranch(context.Context, string) (string, error) {
	return "main", nil
}
func (f *fakeBlobGitAdapter) LatestCommitSHA(context.Context, string, string) (string, error) {
	return "", nil
}
func (f *fakeBlobGitAdapter) AllTags(context.Context, string) ([]git.TagInfo, error) {
	return nil, nil
}
func (f *fakeBlobGitAdapter) CommitDiff(context.Context, string, string) (string, error) {
	return "", nil
}
func (f *fakeBlobGitAdapter) Grep(context.Context, string, string, string, string, int) ([]git.GrepMatch, error) {
	return nil, nil
}

func newTestBlob() (*Blob, *fakeBlobGitAdapter) {
	now := time.Now()
	repo := repository.ReconstructRepository(
		1, "https://github.com/example/repo", "https://github.com/example/repo", "",
		repository.NewWorkingCopy("/tmp/repo", "https://github.com/example/repo"),
		repository.NewTrackingConfigForBranch("main"),
		now, now, time.Time{},
	)

	commit := repository.ReconstructCommit(
		1, "abc1234567890def", 1, "initial commit",
		repository.NewAuthor("Test", "test@test.com"),
		repository.NewAuthor("Test", "test@test.com"),
		now, now, now, "",
	)

	tag := repository.ReconstructTag(
		1, 1, "v1.0.0", "def4567890abcdef", "", repository.Author{}, time.Time{}, now,
	)

	branch := repository.ReconstructBranch(
		1, 1, "main", "aaa1111222233334444", true, now, now,
	)

	gitAdapter := &fakeBlobGitAdapter{
		content: map[string][]byte{
			"abc1234567890def:README.md":    []byte("# Hello\nWorld"),
			"def4567890abcdef:README.md":    []byte("# Tagged\nContent"),
			"aaa1111222233334444:README.md": []byte("# Branch\nContent"),
		},
	}

	blob := NewBlob(
		&fakeBlobRepoStore{repo: repo},
		&fakeBlobCommitStore{commits: []repository.Commit{commit}},
		&fakeBlobTagStore{tags: []repository.Tag{tag}},
		&fakeBlobBranchStore{branches: []repository.Branch{branch}},
		gitAdapter,
	)

	return blob, gitAdapter
}

func TestBlob_ResolveCommitSHA(t *testing.T) {
	blob, _ := newTestBlob()

	sha, err := blob.Resolve(context.Background(), 1, "abc1234567890def")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "abc1234567890def" {
		t.Errorf("expected abc1234567890def, got %s", sha)
	}
}

func TestBlob_ResolveTag(t *testing.T) {
	blob, _ := newTestBlob()

	sha, err := blob.Resolve(context.Background(), 1, "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "def4567890abcdef" {
		t.Errorf("expected def4567890abcdef, got %s", sha)
	}
}

func TestBlob_ResolveBranch(t *testing.T) {
	blob, _ := newTestBlob()

	sha, err := blob.Resolve(context.Background(), 1, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "aaa1111222233334444" {
		t.Errorf("expected aaa1111222233334444, got %s", sha)
	}
}

func TestBlob_ResolveNotFound(t *testing.T) {
	blob, _ := newTestBlob()

	_, err := blob.Resolve(context.Background(), 1, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent blob")
	}
}

func TestBlob_ContentByCommitSHA(t *testing.T) {
	blob, _ := newTestBlob()

	result, err := blob.Content(context.Background(), 1, "abc1234567890def", "README.md")
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
	blob, _ := newTestBlob()

	result, err := blob.Content(context.Background(), 1, "v1.0.0", "README.md")
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
	blob, _ := newTestBlob()

	result, err := blob.Content(context.Background(), 1, "main", "README.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result.Content()) != "# Branch\nContent" {
		t.Errorf("unexpected content: %q", string(result.Content()))
	}
}

func TestBlob_ListFiles(t *testing.T) {
	// Create a temp directory with sample files to walk.
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

	now := time.Now()
	repo := repository.ReconstructRepository(
		1, "https://github.com/example/repo", "https://github.com/example/repo", "",
		repository.NewWorkingCopy(dir, "https://github.com/example/repo"),
		repository.NewTrackingConfigForBranch("main"),
		now, now, time.Time{},
	)

	blob := NewBlob(
		&fakeBlobRepoStore{repo: repo},
		&fakeBlobCommitStore{},
		&fakeBlobTagStore{},
		&fakeBlobBranchStore{},
		&fakeBlobGitAdapter{content: map[string][]byte{}},
	)

	t.Run("all files", func(t *testing.T) {
		entries, err := blob.ListFiles(context.Background(), 1, "**")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
	})

	t.Run("glob pattern", func(t *testing.T) {
		entries, err := blob.ListFiles(context.Background(), 1, "**/*.go")
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
		entries, err := blob.ListFiles(context.Background(), 1, "*")
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
	// The working copy directory does NOT exist on disk yet.
	// ListFiles should clone the repository before walking.
	dir := filepath.Join(t.TempDir(), "nonexistent")

	now := time.Now()
	remoteURL := "https://github.com/example/repo"
	repo := repository.ReconstructRepository(
		1, remoteURL, remoteURL, "",
		repository.NewWorkingCopy(dir, remoteURL),
		repository.NewTrackingConfigForBranch("main"),
		now, now, time.Time{},
	)

	// The fake adapter's CloneRepository creates sample files, simulating
	// what a real clone would do.
	gitAdapter := &fakeBlobGitAdapter{
		content: map[string][]byte{},
		cloneFn: func(_, localPath string) error {
			if err := os.MkdirAll(filepath.Join(localPath, "src"), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(localPath, "README.md"), []byte("# Hello"), 0o644); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(localPath, "src", "main.go"), []byte("package main"), 0o644)
		},
	}

	blob := NewBlob(
		&fakeBlobRepoStore{repo: repo},
		&fakeBlobCommitStore{},
		&fakeBlobTagStore{},
		&fakeBlobBranchStore{},
		gitAdapter,
	)

	entries, err := blob.ListFiles(context.Background(), 1, "**/*.go")
	if err != nil {
		t.Fatalf("expected ListFiles to succeed after cloning, got: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "src/main.go" {
		t.Errorf("expected src/main.go, got %s", entries[0].Path)
	}

	if !gitAdapter.cloneCalled {
		t.Error("expected CloneRepository to be called")
	}
}

func TestBlob_ContentFileNotFound(t *testing.T) {
	blob, _ := newTestBlob()

	_, err := blob.Content(context.Background(), 1, "main", "nonexistent.go")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected database.ErrNotFound, got: %v", err)
	}
}
