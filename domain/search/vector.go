package search

import (
	"context"
)

// VectorStore defines operations for vector similarity search.
type VectorStore interface {
	// Index adds documents to the vector index with embeddings.
	Index(ctx context.Context, request IndexRequest) error

	// Search performs vector similarity search.
	Search(ctx context.Context, request Request) ([]Result, error)

	// HasEmbedding checks if a document has an embedding of the given type.
	HasEmbedding(ctx context.Context, snippetID string, embeddingType EmbeddingType) (bool, error)

	// HasEmbeddings checks which snippet IDs already have embeddings of the given type.
	// Returns a set of snippet IDs that have embeddings.
	HasEmbeddings(ctx context.Context, snippetIDs []string, embeddingType EmbeddingType) (map[string]bool, error)

	// Delete removes documents from the vector index.
	Delete(ctx context.Context, request DeleteRequest) error
}
