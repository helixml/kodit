package git

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
)

// fakeAdapter is a test double that records which methods were called.
type fakeAdapter struct {
	cloned  bool
	fetched bool
}

func (f *fakeAdapter) CloneRepository(_ context.Context, _ string, _ string) error {
	f.cloned = true
	return nil
}

func (f *fakeAdapter) FetchRepository(_ context.Context, _ string) error {
	f.fetched = true
	return nil
}

func (f *fakeAdapter) CheckoutCommit(_ context.Context, _ string, _ string) error { return nil }
func (f *fakeAdapter) CheckoutBranch(_ context.Context, _ string, _ string) error { return nil }
func (f *fakeAdapter) PullRepository(_ context.Context, _ string) error             { return nil }
func (f *fakeAdapter) AllBranches(_ context.Context, _ string) ([]BranchInfo, error) {
	return nil, nil
}
func (f *fakeAdapter) BranchCommits(_ context.Context, _ string, _ string) ([]CommitInfo, error) {
	return nil, nil
}
func (f *fakeAdapter) AllCommitsBulk(_ context.Context, _ string, _ *time.Time) (map[string]CommitInfo, error) {
	return nil, nil
}
func (f *fakeAdapter) BranchCommitSHAs(_ context.Context, _ string, _ string) ([]string, error) {
	return nil, nil
}
func (f *fakeAdapter) AllBranchHeadSHAs(_ context.Context, _ string, _ []string) (map[string]string, error) {
	return nil, nil
}
func (f *fakeAdapter) CommitFiles(_ context.Context, _ string, _ string) ([]FileInfo, error) {
	return nil, nil
}
func (f *fakeAdapter) RepositoryExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (f *fakeAdapter) CommitDetails(_ context.Context, _ string, _ string) (CommitInfo, error) {
	return CommitInfo{}, nil
}
func (f *fakeAdapter) EnsureRepository(_ context.Context, _ string, _ string) error { return nil }
func (f *fakeAdapter) FileContent(_ context.Context, _ string, _ string, _ string) ([]byte, error) {
	return nil, nil
}
func (f *fakeAdapter) DefaultBranch(_ context.Context, _ string) (string, error) {
	return "main", nil
}
func (f *fakeAdapter) LatestCommitSHA(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (f *fakeAdapter) AllTags(_ context.Context, _ string) ([]TagInfo, error) { return nil, nil }
func (f *fakeAdapter) CommitDiff(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func TestUpdate_MissingDirectory(t *testing.T) {
	fake := &fakeAdapter{}
	cloneDir := t.TempDir()
	cloner := NewRepositoryCloner(fake, cloneDir, slog.Default())

	missingPath := filepath.Join(t.TempDir(), "does-not-exist")
	remoteURI := "https://github.com/example/repo.git"
	repo := repository.ReconstructRepository(
		1,
		remoteURI,
		repository.NewWorkingCopy(missingPath, remoteURI),
		repository.NewTrackingConfigForBranch("main"),
		time.Now(), time.Now(),
	)

	newPath, err := cloner.Update(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fake.cloned {
		t.Fatal("expected CloneRepository to be called for missing directory")
	}
	if fake.fetched {
		t.Fatal("expected FetchRepository NOT to be called for missing directory")
	}

	// The returned path should be the correct clone directory, not the old stale one.
	expectedPath := cloner.ClonePathFromURI(remoteURI)
	if newPath != expectedPath {
		t.Fatalf("expected relocated path %q, got %q", expectedPath, newPath)
	}
}

func TestUpdate_InaccessibleDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based test not reliable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	fake := &fakeAdapter{}
	cloner := NewRepositoryCloner(fake, t.TempDir(), slog.Default())

	// Create parent/child, then remove execute on the parent so that
	// os.Stat on the child returns a permission error (not IsNotExist).
	parent := filepath.Join(t.TempDir(), "locked-parent")
	child := filepath.Join(parent, "repo")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatalf("setup chmod: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(parent, 0o755)
		_ = os.RemoveAll(parent)
	})

	repo := repository.ReconstructRepository(
		2,
		"https://github.com/example/repo.git",
		repository.NewWorkingCopy(child, "https://github.com/example/repo.git"),
		repository.NewTrackingConfigForBranch("main"),
		time.Now(), time.Now(),
	)

	newPath, err := cloner.Update(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fake.cloned {
		t.Fatal("expected CloneRepository to be called for inaccessible directory")
	}
	if fake.fetched {
		t.Fatal("expected FetchRepository NOT to be called for inaccessible directory")
	}

	expectedPath := cloner.ClonePathFromURI("https://github.com/example/repo.git")
	if newPath != expectedPath {
		t.Fatalf("expected relocated path %q, got %q", expectedPath, newPath)
	}
}
