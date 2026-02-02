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

// VectorSearchRepository defines operations for vector similarity search.
type VectorSearchRepository interface {
	// Index adds documents to the vector index with embeddings.
	Index(ctx context.Context, request domain.IndexRequest) error

	// Search performs vector similarity search.
	Search(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error)

	// HasEmbedding checks if a snippet has an embedding of the given type.
	HasEmbedding(ctx context.Context, snippetID string, embeddingType EmbeddingType) (bool, error)

	// Delete removes documents from the vector index.
	Delete(ctx context.Context, request domain.DeleteRequest) error
}
