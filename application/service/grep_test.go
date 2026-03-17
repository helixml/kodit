package service

import (
	"context"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/git"
)

func TestGroupByFile_MultipleFiles(t *testing.T) {
	matches := []git.GrepMatch{
		{Path: "a.go", Line: 1, Content: "line1"},
		{Path: "a.go", Line: 5, Content: "line5"},
		{Path: "b.go", Line: 3, Content: "line3"},
		{Path: "c.go", Line: 7, Content: "line7"},
	}

	results := groupByFile(matches, "abc123", 1, 10)

	if len(results) != 3 {
		t.Fatalf("expected 3 files, got %d", len(results))
	}

	if results[0].Path != "a.go" {
		t.Errorf("expected first file a.go, got %s", results[0].Path)
	}
	if len(results[0].Matches) != 2 {
		t.Errorf("expected 2 matches for a.go, got %d", len(results[0].Matches))
	}
	if results[0].Language != ".go" {
		t.Errorf("expected language .go, got %s", results[0].Language)
	}
	if results[0].CommitSHA != "abc123" {
		t.Errorf("expected commitSHA abc123, got %s", results[0].CommitSHA)
	}
	if results[0].RepoID != 1 {
		t.Errorf("expected repoID 1, got %d", results[0].RepoID)
	}

	if results[1].Path != "b.go" {
		t.Errorf("expected second file b.go, got %s", results[1].Path)
	}
	if len(results[1].Matches) != 1 {
		t.Errorf("expected 1 match for b.go, got %d", len(results[1].Matches))
	}

	if results[2].Path != "c.go" {
		t.Errorf("expected third file c.go, got %s", results[2].Path)
	}
}

func TestGroupByFile_MaxFilesCap(t *testing.T) {
	matches := []git.GrepMatch{
		{Path: "a.go", Line: 1, Content: "a"},
		{Path: "b.go", Line: 1, Content: "b"},
		{Path: "c.go", Line: 1, Content: "c"},
		{Path: "d.go", Line: 1, Content: "d"},
	}

	results := groupByFile(matches, "abc123", 1, 2)

	if len(results) != 2 {
		t.Fatalf("expected 2 files (capped), got %d", len(results))
	}
	if results[0].Path != "a.go" {
		t.Errorf("expected first file a.go, got %s", results[0].Path)
	}
	if results[1].Path != "b.go" {
		t.Errorf("expected second file b.go, got %s", results[1].Path)
	}
}

func TestGroupByFile_Empty(t *testing.T) {
	results := groupByFile(nil, "abc123", 1, 10)
	if results != nil {
		t.Errorf("expected nil for empty input, got %v", results)
	}
}

func TestGroupByFile_PreservesOrder(t *testing.T) {
	matches := []git.GrepMatch{
		{Path: "z.go", Line: 1, Content: "z"},
		{Path: "a.go", Line: 1, Content: "a"},
		{Path: "m.go", Line: 1, Content: "m"},
	}

	results := groupByFile(matches, "abc123", 1, 10)

	if len(results) != 3 {
		t.Fatalf("expected 3 files, got %d", len(results))
	}
	if results[0].Path != "z.go" {
		t.Errorf("expected first file z.go, got %s", results[0].Path)
	}
	if results[1].Path != "a.go" {
		t.Errorf("expected second file a.go, got %s", results[1].Path)
	}
	if results[2].Path != "m.go" {
		t.Errorf("expected third file m.go, got %s", results[2].Path)
	}
}

func TestGrep_Search_ReturnsGroupedResults(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	remoteURL := "https://github.com/test/repo"
	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy("/tmp/repo", remoteURL))
	saved, err := stores.repos.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repo: %v", err)
	}

	now := time.Now()
	commit := repository.NewCommit(
		"abc123", saved.ID(), "initial",
		repository.NewAuthor("Test", "test@test.com"),
		repository.NewAuthor("Test", "test@test.com"),
		now, now,
	)
	if _, err := stores.commits.Save(ctx, commit); err != nil {
		t.Fatalf("save commit: %v", err)
	}

	gitAdapter := &fakeGitAdapter{
		matches: []git.GrepMatch{
			{Path: "main.go", Line: 10, Content: "func main()"},
			{Path: "main.go", Line: 20, Content: "func init()"},
			{Path: "util.go", Line: 5, Content: "func helper()"},
		},
	}

	grep := NewGrep(stores.repos, stores.commits, gitAdapter)

	results, err := grep.Search(ctx, saved.ID(), "func", "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 grouped results, got %d", len(results))
	}

	if results[0].Path != "main.go" {
		t.Errorf("expected first group main.go, got %s", results[0].Path)
	}
	if len(results[0].Matches) != 2 {
		t.Errorf("expected 2 matches for main.go, got %d", len(results[0].Matches))
	}
	if results[0].CommitSHA != "abc123" {
		t.Errorf("expected commitSHA abc123, got %s", results[0].CommitSHA)
	}
	if results[0].RepoID != saved.ID() {
		t.Errorf("expected repoID %d, got %d", saved.ID(), results[0].RepoID)
	}

	if results[1].Path != "util.go" {
		t.Errorf("expected second group util.go, got %s", results[1].Path)
	}
	if len(results[1].Matches) != 1 {
		t.Errorf("expected 1 match for util.go, got %d", len(results[1].Matches))
	}
}

func TestGrep_Search_RepoNotFound(t *testing.T) {
	stores := newTestStores(t)

	grep := NewGrep(stores.repos, stores.commits, &fakeGitAdapter{})

	_, err := grep.Search(context.Background(), 999, "func", "", 10)
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
}

func TestGrep_Search_NoCommits(t *testing.T) {
	stores := newTestStores(t)
	ctx := context.Background()

	remoteURL := "https://github.com/test/repo"
	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy("/tmp/repo", remoteURL))
	saved, err := stores.repos.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repo: %v", err)
	}

	grep := NewGrep(stores.repos, stores.commits, &fakeGitAdapter{})

	_, err = grep.Search(ctx, saved.ID(), "func", "", 10)
	if err == nil {
		t.Fatal("expected error when no commits found")
	}
}
