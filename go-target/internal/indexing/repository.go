package indexing

import (
	"context"

	"github.com/helixml/kodit/internal/domain"
)

// SnippetRepository defines operations for snippet persistence.
type SnippetRepository interface {
	// Save persists snippets for a commit.
	Save(ctx context.Context, commitSHA string, snippets []Snippet) error

	// SnippetsForCommit returns all snippets for a specific commit.
	SnippetsForCommit(ctx context.Context, commitSHA string) ([]Snippet, error)

	// DeleteForCommit removes all snippet associations for a commit.
	DeleteForCommit(ctx context.Context, commitSHA string) error

	// Search finds snippets matching the search request.
	Search(ctx context.Context, request domain.MultiSearchRequest) ([]Snippet, error)

	// ByIDs returns snippets by their SHA identifiers.
	ByIDs(ctx context.Context, ids []string) ([]Snippet, error)

	// BySHA returns a single snippet by its SHA identifier.
	BySHA(ctx context.Context, sha string) (Snippet, error)
}

// CommitIndexRepository defines operations for commit index persistence.
type CommitIndexRepository interface {
	// Get returns a commit index by SHA.
	Get(ctx context.Context, commitSHA string) (CommitIndex, error)

	// Save persists a commit index.
	Save(ctx context.Context, index CommitIndex) error

	// Delete removes a commit index.
	Delete(ctx context.Context, commitSHA string) error

	// Exists checks if a commit index exists.
	Exists(ctx context.Context, commitSHA string) (bool, error)
}

// BM25Repository defines operations for BM25 full-text search indexing.
type BM25Repository interface {
	// Index adds documents to the BM25 index.
	Index(ctx context.Context, request domain.IndexRequest) error

	// Search performs BM25 keyword search.
	Search(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error)

	// Delete removes documents from the BM25 index.
	Delete(ctx context.Context, request domain.DeleteRequest) error
}

// EmbeddingType represents the type of embedding.
type EmbeddingType string

// EmbeddingType values.
const (
	EmbeddingTypeCode    EmbeddingType = "code"
	EmbeddingTypeSummary EmbeddingType = "summary"
)

// EmbeddingInfo holds embedding data for a snippet.
type EmbeddingInfo struct {
	snippetID     string
	embeddingType EmbeddingType
	embedding     []float64
}

// NewEmbeddingInfo creates a new EmbeddingInfo.
func NewEmbeddingInfo(snippetID string, embeddingType EmbeddingType, embedding []float64) EmbeddingInfo {
	vec := make([]float64, len(embedding))
	copy(vec, embedding)
	return EmbeddingInfo{
		snippetID:     snippetID,
		embeddingType: embeddingType,
		embedding:     vec,
	}
}

// SnippetID returns the snippet identifier.
func (e EmbeddingInfo) SnippetID() string { return e.snippetID }

// Type returns the embedding type.
func (e EmbeddingInfo) Type() EmbeddingType { return e.embeddingType }

// Embedding returns the embedding vector (copy).
func (e EmbeddingInfo) Embedding() []float64 {
	result := make([]float64, len(e.embedding))
	copy(result, e.embedding)
	return result
}

// EmbeddingTruncated returns the first n values of the embedding.
func (e EmbeddingInfo) EmbeddingTruncated(n int) []float64 {
	if n >= len(e.embedding) {
		return e.Embedding()
	}
	result := make([]float64, n)
	copy(result, e.embedding[:n])
	return result
}

// VectorSearchRepository defines operations for vector similarity search.
type VectorSearchRepository interface {
	// Index adds documents to the vector index with embeddings.
	Index(ctx context.Context, request domain.IndexRequest) error

	// Search performs vector similarity search.
	Search(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error)

	// HasEmbedding checks if a snippet has an embedding of the given type.
	HasEmbedding(ctx context.Context, snippetID string, embeddingType EmbeddingType) (bool, error)

	// EmbeddingsForSnippets returns embedding info for the specified snippet IDs.
	EmbeddingsForSnippets(ctx context.Context, snippetIDs []string) ([]EmbeddingInfo, error)

	// Delete removes documents from the vector index.
	Delete(ctx context.Context, request domain.DeleteRequest) error
}
