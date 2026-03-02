package service

import (
	"testing"

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
