package git

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FakeAdapter implements Adapter for testing.
type FakeAdapter struct {
	commits map[string]CommitInfo
	files   map[string][]FileInfo
	branches []BranchInfo
	tags     []TagInfo
}

func NewFakeAdapter() *FakeAdapter {
	return &FakeAdapter{
		commits: make(map[string]CommitInfo),
		files:   make(map[string][]FileInfo),
	}
}

func (f *FakeAdapter) AddCommit(sha string, info CommitInfo) {
	f.commits[sha] = info
}

func (f *FakeAdapter) AddFilesForCommit(sha string, files []FileInfo) {
	f.files[sha] = files
}

func (f *FakeAdapter) SetBranches(branches []BranchInfo) {
	f.branches = branches
}

func (f *FakeAdapter) SetTags(tags []TagInfo) {
	f.tags = tags
}

func (f *FakeAdapter) CommitDetails(_ context.Context, _ string, commitSHA string) (CommitInfo, error) {
	if info, ok := f.commits[commitSHA]; ok {
		return info, nil
	}
	return CommitInfo{}, nil
}

func (f *FakeAdapter) CommitFiles(_ context.Context, _ string, commitSHA string) ([]FileInfo, error) {
	return f.files[commitSHA], nil
}

func (f *FakeAdapter) AllBranches(_ context.Context, _ string) ([]BranchInfo, error) {
	return f.branches, nil
}

func (f *FakeAdapter) AllTags(_ context.Context, _ string) ([]TagInfo, error) {
	return f.tags, nil
}

func (f *FakeAdapter) BranchCommits(_ context.Context, _ string, _ string) ([]CommitInfo, error) {
	infos := make([]CommitInfo, 0, len(f.commits))
	for _, info := range f.commits {
		infos = append(infos, info)
	}
	return infos, nil
}

// Unused adapter methods for test fake
func (f *FakeAdapter) CloneRepository(_ context.Context, _ string, _ string) error { return nil }
func (f *FakeAdapter) CheckoutCommit(_ context.Context, _ string, _ string) error { return nil }
func (f *FakeAdapter) CheckoutBranch(_ context.Context, _ string, _ string) error { return nil }
func (f *FakeAdapter) FetchRepository(_ context.Context, _ string) error { return nil }
func (f *FakeAdapter) PullRepository(_ context.Context, _ string) error { return nil }
func (f *FakeAdapter) AllCommitsBulk(_ context.Context, _ string, _ *time.Time) (map[string]CommitInfo, error) { return nil, nil }
func (f *FakeAdapter) BranchCommitSHAs(_ context.Context, _ string, _ string) ([]string, error) { return nil, nil }
func (f *FakeAdapter) AllBranchHeadSHAs(_ context.Context, _ string, _ []string) (map[string]string, error) { return nil, nil }
func (f *FakeAdapter) RepositoryExists(_ context.Context, _ string) (bool, error) { return false, nil }
func (f *FakeAdapter) EnsureRepository(_ context.Context, _ string, _ string) error { return nil }
func (f *FakeAdapter) FileContent(_ context.Context, _ string, _ string, _ string) ([]byte, error) { return nil, nil }
func (f *FakeAdapter) DefaultBranch(_ context.Context, _ string) (string, error) { return "main", nil }
func (f *FakeAdapter) LatestCommitSHA(_ context.Context, _ string, _ string) (string, error) { return "", nil }
func (f *FakeAdapter) CommitDiff(_ context.Context, _ string, _ string) (string, error) { return "", nil }

func TestScanner_ScanCommit(t *testing.T) {
	adapter := NewFakeAdapter()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewScanner(adapter, logger)

	sha := "abc123def456"
	repoID := int64(42)
	now := time.Now()

	adapter.AddCommit(sha, CommitInfo{
		SHA:            sha,
		Message:        "Add feature X",
		AuthorName:     "Alice",
		AuthorEmail:    "alice@example.com",
		CommitterName:  "Bob",
		CommitterEmail: "bob@example.com",
		AuthoredAt:     now.Add(-time.Hour),
		CommittedAt:    now,
	})

	adapter.AddFilesForCommit(sha, []FileInfo{
		{Path: "main.go", BlobSHA: "blob1", Size: 1024},
		{Path: "lib/util.py", BlobSHA: "blob2", Size: 512},
		{Path: "README.md", BlobSHA: "blob3", Size: 256},
	})

	result, err := scanner.ScanCommit(context.Background(), "/repo", sha, repoID)
	require.NoError(t, err)

	commit := result.Commit()
	assert.Equal(t, sha, commit.SHA())
	assert.Equal(t, repoID, commit.RepoID())
	assert.Equal(t, "Add feature X", commit.Message())
	assert.Equal(t, "Alice", commit.Author().Name())
	assert.Equal(t, "alice@example.com", commit.Author().Email())

	files := result.Files()
	assert.Len(t, files, 3)

	// Verify language detection
	assert.Equal(t, "go", files[0].Language())
	assert.Equal(t, "python", files[1].Language())
	assert.Equal(t, "markdown", files[2].Language())
}

func TestScanner_ScanAllBranches(t *testing.T) {
	adapter := NewFakeAdapter()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewScanner(adapter, logger)

	adapter.SetBranches([]BranchInfo{
		{Name: "main", HeadSHA: "sha1", IsDefault: true},
		{Name: "feature/x", HeadSHA: "sha2", IsDefault: false},
	})

	branches, err := scanner.ScanAllBranches(context.Background(), "/repo", 42)
	require.NoError(t, err)
	assert.Len(t, branches, 2)

	assert.Equal(t, "main", branches[0].Name())
	assert.True(t, branches[0].IsDefault())
	assert.Equal(t, "feature/x", branches[1].Name())
	assert.False(t, branches[1].IsDefault())
}

func TestScanner_ScanAllTags(t *testing.T) {
	adapter := NewFakeAdapter()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewScanner(adapter, logger)

	adapter.SetTags([]TagInfo{
		{Name: "v1.0.0", TargetCommitSHA: "sha1", Message: "Release 1.0"},
		{Name: "v1.1.0", TargetCommitSHA: "sha2", Message: "Release 1.1"},
	})

	tags, err := scanner.ScanAllTags(context.Background(), "/repo", 42)
	require.NoError(t, err)
	assert.Len(t, tags, 2)

	assert.Equal(t, "v1.0.0", tags[0].Name())
	assert.Equal(t, "sha1", tags[0].CommitSHA())
	assert.Equal(t, "v1.1.0", tags[1].Name())
}

func TestLanguageFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"component.tsx", "typescript"},
		{"lib.rb", "ruby"},
		{"main.rs", "rust"},
		{"App.java", "java"},
		{"core.c", "c"},
		{"util.cpp", "cpp"},
		{"Model.cs", "csharp"},
		{"index.php", "php"},
		{"AppDelegate.swift", "swift"},
		{"Main.kt", "kotlin"},
		{"build.sh", "shell"},
		{"query.sql", "sql"},
		{"config.yaml", "yaml"},
		{"data.json", "json"},
		{"index.html", "html"},
		{"style.css", "css"},
		{"styles.scss", "scss"},
		{"README.md", "markdown"},
		{"Dockerfile", ""},      // No extension
		{"unknown.xyz", "xyz"}, // Unknown extension
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := languageFromPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShortSHA(t *testing.T) {
	assert.Equal(t, "abc12345", shortSHA("abc12345def67890"))
	assert.Equal(t, "abc1234", shortSHA("abc1234"))
	assert.Equal(t, "abc", shortSHA("abc"))
	assert.Equal(t, "", shortSHA(""))
}
