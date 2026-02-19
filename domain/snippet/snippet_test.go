package snippet

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
)

func TestNewSnippet_ComputesSHA(t *testing.T) {
	s := NewSnippet("func main() {}", "go", nil)

	expected := sha256.Sum256([]byte("func main() {}"))
	want := hex.EncodeToString(expected[:])

	if s.SHA() != want {
		t.Errorf("SHA() = %q, want %q", s.SHA(), want)
	}
}

func TestNewSnippet_SameSHAForSameContent(t *testing.T) {
	s1 := NewSnippet("hello world", "txt", nil)
	s2 := NewSnippet("hello world", "txt", nil)

	if s1.SHA() != s2.SHA() {
		t.Error("same content should produce same SHA")
	}
}

func TestNewSnippet_DifferentSHAForDifferentContent(t *testing.T) {
	s1 := NewSnippet("hello", "txt", nil)
	s2 := NewSnippet("world", "txt", nil)

	if s1.SHA() == s2.SHA() {
		t.Error("different content should produce different SHA")
	}
}

func TestNewSnippet_EmptyContent(t *testing.T) {
	s := NewSnippet("", "go", nil)

	if s.SHA() == "" {
		t.Error("SHA should be computed even for empty content")
	}
	if s.Content() != "" {
		t.Errorf("Content() = %q, want empty", s.Content())
	}
}

func TestNewSnippet_Fields(t *testing.T) {
	files := []repository.File{
		repository.NewFile("abc123", "main.go", "go", 100),
	}
	s := NewSnippet("package main", "go", files)

	if s.Extension() != "go" {
		t.Errorf("Extension() = %q, want %q", s.Extension(), "go")
	}
	if s.Content() != "package main" {
		t.Errorf("Content() = %q, want %q", s.Content(), "package main")
	}
	if len(s.DerivesFrom()) != 1 {
		t.Fatalf("DerivesFrom() length = %d, want 1", len(s.DerivesFrom()))
	}
	if len(s.Enrichments()) != 0 {
		t.Errorf("Enrichments() length = %d, want 0", len(s.Enrichments()))
	}
	if s.CreatedAt().IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if s.UpdatedAt().IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestNewSnippet_CopiesDerivesFrom(t *testing.T) {
	files := []repository.File{
		repository.NewFile("abc123", "main.go", "go", 100),
	}
	s := NewSnippet("content", "go", files)

	// Mutate the original slice
	files[0] = repository.NewFile("def456", "MUTATED.go", "go", 200)

	derived := s.DerivesFrom()
	if derived[0].Path() == "MUTATED.go" {
		t.Error("NewSnippet should copy the derivesFrom slice")
	}
}

func TestSnippet_DerivesFrom_ReturnsCopy(t *testing.T) {
	files := []repository.File{
		repository.NewFile("abc123", "main.go", "go", 100),
	}
	s := NewSnippet("content", "go", files)

	derived := s.DerivesFrom()
	derived[0] = repository.NewFile("def456", "MUTATED.go", "go", 200)

	derivedAgain := s.DerivesFrom()
	if derivedAgain[0].Path() == "MUTATED.go" {
		t.Error("DerivesFrom() should return a copy")
	}
}

func TestSnippet_WithEnrichments(t *testing.T) {
	s := NewSnippet("content", "go", nil)
	enrichments := []Enrichment{
		NewEnrichment("summary", "This is a summary"),
	}

	updated := s.WithEnrichments(enrichments)

	if len(updated.Enrichments()) != 1 {
		t.Fatalf("Enrichments() length = %d, want 1", len(updated.Enrichments()))
	}
	if updated.Enrichments()[0].Type() != "summary" {
		t.Errorf("enrichment type = %q, want %q", updated.Enrichments()[0].Type(), "summary")
	}
	if updated.Enrichments()[0].Content() != "This is a summary" {
		t.Errorf("enrichment content = %q, want %q", updated.Enrichments()[0].Content(), "This is a summary")
	}

	// Original should be unchanged
	if len(s.Enrichments()) != 0 {
		t.Error("original snippet should not be modified")
	}
}

func TestSnippet_WithEnrichments_Appends(t *testing.T) {
	s := NewSnippet("content", "go", nil)
	s = s.WithEnrichments([]Enrichment{NewEnrichment("first", "A")})
	s = s.WithEnrichments([]Enrichment{NewEnrichment("second", "B")})

	if len(s.Enrichments()) != 2 {
		t.Errorf("Enrichments() length = %d, want 2", len(s.Enrichments()))
	}
}

func TestSnippet_WithEnrichments_SHA_Unchanged(t *testing.T) {
	s := NewSnippet("content", "go", nil)
	original := s.SHA()

	updated := s.WithEnrichments([]Enrichment{NewEnrichment("summary", "text")})

	if updated.SHA() != original {
		t.Error("SHA should not change when adding enrichments")
	}
}

func TestComputeSHA_Deterministic(t *testing.T) {
	sha1 := ComputeSHA("test input")
	sha2 := ComputeSHA("test input")

	if sha1 != sha2 {
		t.Error("ComputeSHA should be deterministic")
	}
}

func TestComputeSHA_EmptyString(t *testing.T) {
	sha := ComputeSHA("")
	if sha == "" {
		t.Error("ComputeSHA should return a hash even for empty string")
	}
	if len(sha) != 64 {
		t.Errorf("SHA256 hex should be 64 characters, got %d", len(sha))
	}
}

func TestSnippet_Enrichments_ReturnsCopy(t *testing.T) {
	s := NewSnippet("content", "go", nil).
		WithEnrichments([]Enrichment{NewEnrichment("summary", "text")})

	enrichments := s.Enrichments()
	enrichments[0] = NewEnrichment("MUTATED", "MUTATED")

	if s.Enrichments()[0].Type() == "MUTATED" {
		t.Error("Enrichments() should return a copy")
	}
}

func TestReconstructSnippet(t *testing.T) {
	now := time.Now()
	files := []repository.File{
		repository.NewFile("abc123", "main.go", "go", 100),
	}
	enrichments := []Enrichment{
		NewEnrichment("summary", "text"),
	}

	s := ReconstructSnippet("sha256hash", "content", "go", files, enrichments, now, now)
	if s.SHA() != "sha256hash" {
		t.Errorf("SHA() = %q, want %q", s.SHA(), "sha256hash")
	}
	if len(s.DerivesFrom()) != 1 {
		t.Errorf("DerivesFrom() length = %d, want 1", len(s.DerivesFrom()))
	}
	if len(s.Enrichments()) != 1 {
		t.Errorf("Enrichments() length = %d, want 1", len(s.Enrichments()))
	}
}
