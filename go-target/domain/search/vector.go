package search

import (
	"context"

	"github.com/helixml/kodit/domain/snippet"
)

// VectorStore defines operations for vector similarity search.
type VectorStore interface {
	// Index adds documents to the vector index with embeddings.
	Index(ctx context.Context, request IndexRequest) error

	// Search performs vector similarity search.
	Search(ctx context.Context, request Request) ([]Result, error)

	// HasEmbedding checks if a snippet has an embedding of the given type.
	HasEmbedding(ctx context.Context, snippetID string, embeddingType snippet.EmbeddingType) (bool, error)

	// EmbeddingsForSnippets returns embedding info for the specified snippet IDs.
	EmbeddingsForSnippets(ctx context.Context, snippetIDs []string) ([]snippet.EmbeddingInfo, error)

	// Delete removes documents from the vector index.
	Delete(ctx context.Context, request DeleteRequest) error
}
