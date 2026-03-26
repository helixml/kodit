package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog"

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
func (f *fakeAdapter) PullRepository(_ context.Context, _ string) error           { return nil }
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
func (f *fakeAdapter) Grep(_ context.Context, _ string, _ string, _ string, _ string, _ int) ([]GrepMatch, error) {
	return nil, nil
}

// ---- file:// URI helpers ----

func TestIsFileURI(t *testing.T) {
	if !isFileURI("file:///home/user/project") {
		t.Fatal("expected true for file:// URI")
	}
	if isFileURI("https://github.com/example/repo") {
		t.Fatal("expected false for https:// URI")
	}
}

func TestLocalPathFromFileURI(t *testing.T) {
	got := localPathFromFileURI("file:///home/user/project")
	if got != "/home/user/project" {
		t.Fatalf("expected /home/user/project, got %q", got)
	}
}

func TestClonePathFromURI_FileURI(t *testing.T) {
	cloner := NewRepositoryCloner(&fakeAdapter{}, t.TempDir(), zerolog.Nop())
	localPath := "/home/user/project"
	uri := "file://" + localPath
	got := cloner.ClonePathFromURI(uri)
	if got != localPath {
		t.Fatalf("expected %q, got %q", localPath, got)
	}
}

func TestClone_FileURI_SkipsAdapter(t *testing.T) {
	fake := &fakeAdapter{}
	cloner := NewRepositoryCloner(fake, t.TempDir(), zerolog.Nop())

	localPath := t.TempDir()
	uri := "file://" + localPath

	got, err := cloner.Clone(context.Background(), uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != localPath {
		t.Fatalf("expected %q, got %q", localPath, got)
	}
	if fake.cloned {
		t.Fatal("expected CloneRepository NOT to be called for file:// URI")
	}
}

func TestUpdate_FileURI_NonGitDir_SkipsGitOps(t *testing.T) {
	fake := &fakeAdapter{}
	cloner := NewRepositoryCloner(fake, t.TempDir(), zerolog.Nop())

	// A plain temp dir — not a git repo.
	plainDir := t.TempDir()
	uri := "file://" + plainDir

	repo := repository.ReconstructRepository(
		3,
		0,
		uri, uri, "",
		repository.NewWorkingCopy(plainDir, uri),
		repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
	)

	gotPath, err := cloner.Update(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != plainDir {
		t.Fatalf("expected %q, got %q", plainDir, gotPath)
	}
	if fake.cloned {
		t.Fatal("expected CloneRepository NOT to be called for plain file:// dir")
	}
	if fake.fetched {
		t.Fatal("expected FetchRepository NOT to be called for plain file:// dir")
	}
}

func TestUpdate_FileURI_GitRepo_SkipsNetworkOps(t *testing.T) {
	fake := &fakeAdapter{}
	cloner := NewRepositoryCloner(fake, t.TempDir(), zerolog.Nop())

	// Create a real git repo in a temp dir so isGitRepo returns true.
	repoDir := t.TempDir()
	initCmd := exec.Command("git", "init", repoDir)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	uri := "file://" + repoDir

	repo := repository.ReconstructRepository(
		4,
		0,
		uri, uri, "",
		repository.NewWorkingCopy(repoDir, uri),
		repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
	)

	gotPath, err := cloner.Update(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != repoDir {
		t.Fatalf("expected %q, got %q", repoDir, gotPath)
	}
	if fake.cloned {
		t.Fatal("expected CloneRepository NOT to be called for file:// git repo")
	}
	if fake.fetched {
		t.Fatal("expected FetchRepository NOT to be called for file:// git repo (no remote)")
	}
}

func TestUpdate_FileURI_MissingDir_DoesNotReclone(t *testing.T) {
	fake := &fakeAdapter{}
	cloner := NewRepositoryCloner(fake, t.TempDir(), zerolog.Nop())

	// A path that doesn't exist.
	missingDir := filepath.Join(t.TempDir(), "gone")
	uri := "file://" + missingDir

	repo := repository.ReconstructRepository(
		5,
		0,
		uri, uri, "",
		repository.NewWorkingCopy(missingDir, uri),
		repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
	)

	// isGitRepo will return false for a missing dir, so we expect early return without cloning.
	_, err := cloner.Update(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.cloned {
		t.Fatal("expected CloneRepository NOT to be called for missing file:// dir")
	}
}

// ---- existing tests ----

func TestUpdate_MissingDirectory(t *testing.T) {
	fake := &fakeAdapter{}
	cloneDir := t.TempDir()
	cloner := NewRepositoryCloner(fake, cloneDir, zerolog.New(os.Stderr).With().Timestamp().Logger())

	missingPath := filepath.Join(t.TempDir(), "does-not-exist")
	remoteURI := "https://github.com/example/repo.git"
	repo := repository.ReconstructRepository(
		1,
		0,
		remoteURI,
		remoteURI,
		"",
		repository.NewWorkingCopy(missingPath, remoteURI),
		repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
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
	cloner := NewRepositoryCloner(fake, t.TempDir(), zerolog.New(os.Stderr).With().Timestamp().Logger())

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
		0,
		"https://github.com/example/repo.git",
		"https://github.com/example/repo.git",
		"",
		repository.NewWorkingCopy(child, "https://github.com/example/repo.git"),
		repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
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
