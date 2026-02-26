package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/git"
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
	content map[string][]byte
}

func (f *fakeBlobGitAdapter) FileContent(_ context.Context, _, commitSHA, filePath string) ([]byte, error) {
	key := commitSHA + ":" + filePath
	content, ok := f.content[key]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", key)
	}
	return content, nil
}

func (f *fakeBlobGitAdapter) CloneRepository(context.Context, string, string) error { return nil }
func (f *fakeBlobGitAdapter) CheckoutCommit(context.Context, string, string) error  { return nil }
func (f *fakeBlobGitAdapter) CheckoutBranch(context.Context, string, string) error  { return nil }
func (f *fakeBlobGitAdapter) FetchRepository(context.Context, string) error         { return nil }
func (f *fakeBlobGitAdapter) PullRepository(context.Context, string) error          { return nil }
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
func (f *fakeBlobGitAdapter) RepositoryExists(context.Context, string) (bool, error) {
	return false, nil
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

func newTestBlob() (*Blob, *fakeBlobGitAdapter) {
	now := time.Now()
	repo := repository.ReconstructRepository(
		1, "https://github.com/example/repo",
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
