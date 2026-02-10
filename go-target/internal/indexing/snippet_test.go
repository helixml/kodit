package indexing

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewSnippet(t *testing.T) {
	content := "func main() { fmt.Println(\"Hello\") }"
	extension := "go"
	file := repository.NewFile("abc123", "main.go", "go", 100)
	derivesFrom := []repository.File{file}

	snippet := NewSnippet(content, extension, derivesFrom)

	assert.NotEmpty(t, snippet.SHA())
	assert.Equal(t, content, snippet.Content())
	assert.Equal(t, extension, snippet.Extension())
	assert.Len(t, snippet.DerivesFrom(), 1)
	assert.Equal(t, "main.go", snippet.DerivesFrom()[0].Path())
	assert.Empty(t, snippet.Enrichments())
	assert.False(t, snippet.CreatedAt().IsZero())
	assert.False(t, snippet.UpdatedAt().IsZero())
}

func TestSnippetSHAIsContentAddressed(t *testing.T) {
	content1 := "func foo() {}"
	content2 := "func bar() {}"

	snippet1 := NewSnippet(content1, "go", nil)
	snippet2 := NewSnippet(content1, "go", nil) // Same content
	snippet3 := NewSnippet(content2, "go", nil) // Different content

	// Same content should produce same SHA
	assert.Equal(t, snippet1.SHA(), snippet2.SHA())

	// Different content should produce different SHA
	assert.NotEqual(t, snippet1.SHA(), snippet3.SHA())
}

func TestComputeSHA(t *testing.T) {
	content := "test content"

	sha := ComputeSHA(content)

	// Verify against direct computation
	hash := sha256.Sum256([]byte(content))
	expected := hex.EncodeToString(hash[:])
	assert.Equal(t, expected, sha)
}

func TestReconstructSnippet(t *testing.T) {
	sha := "abc123def456"
	content := "func test() {}"
	extension := "go"
	createdAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	file := repository.NewFile("commit1", "test.go", "go", 50)
	enrichment := domain.NewEnrichment("summary", "This is a test function")

	snippet := ReconstructSnippet(
		sha, content, extension,
		[]repository.File{file},
		[]domain.Enrichment{enrichment},
		createdAt, updatedAt,
	)

	assert.Equal(t, sha, snippet.SHA())
	assert.Equal(t, content, snippet.Content())
	assert.Equal(t, extension, snippet.Extension())
	assert.Len(t, snippet.DerivesFrom(), 1)
	assert.Len(t, snippet.Enrichments(), 1)
	assert.Equal(t, "summary", snippet.Enrichments()[0].Type())
	assert.Equal(t, createdAt, snippet.CreatedAt())
	assert.Equal(t, updatedAt, snippet.UpdatedAt())
}

func TestSnippetWithEnrichments(t *testing.T) {
	snippet := NewSnippet("func test() {}", "go", nil)
	assert.Empty(t, snippet.Enrichments())

	enrichments := []domain.Enrichment{
		domain.NewEnrichment("summary", "Test function"),
		domain.NewEnrichment("api_doc", "API documentation"),
	}

	updated := snippet.WithEnrichments(enrichments)

	// Original should be unchanged
	assert.Empty(t, snippet.Enrichments())

	// Updated should have enrichments
	assert.Len(t, updated.Enrichments(), 2)
	assert.Equal(t, "summary", updated.Enrichments()[0].Type())
	assert.Equal(t, "api_doc", updated.Enrichments()[1].Type())

	// SHA should remain the same (content-addressed)
	assert.Equal(t, snippet.SHA(), updated.SHA())
}

func TestSnippetDerivesFromIsCopied(t *testing.T) {
	file := repository.NewFile("abc123", "main.go", "go", 100)
	derivesFrom := []repository.File{file}

	snippet := NewSnippet("func main() {}", "go", derivesFrom)

	// Modifying the original slice should not affect the snippet
	derivesFrom[0] = repository.NewFile("xyz789", "other.go", "go", 200)

	// Snippet should still have the original file
	assert.Equal(t, "main.go", snippet.DerivesFrom()[0].Path())
}

func TestSnippetEnrichmentsIsCopied(t *testing.T) {
	enrichment := domain.NewEnrichment("summary", "Test")
	enrichments := []domain.Enrichment{enrichment}

	snippet := ReconstructSnippet(
		"sha", "content", "go",
		nil, enrichments,
		time.Now(), time.Now(),
	)

	// Get enrichments and verify it's a copy
	retrieved := snippet.Enrichments()
	assert.Len(t, retrieved, 1)

	// Modifying returned slice should not affect snippet
	retrieved[0] = domain.NewEnrichment("other", "Other")

	// Original should be unchanged
	assert.Equal(t, "summary", snippet.Enrichments()[0].Type())
}
