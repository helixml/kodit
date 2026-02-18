package search

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// EmbeddingStore defines persistence operations for vector embeddings.
type EmbeddingStore interface {
	// SaveAll persists pre-computed embeddings.
	SaveAll(ctx context.Context, embeddings []Embedding) error

	// Find performs vector similarity search using options.
	// Embedding must be passed via WithEmbedding.
	Find(ctx context.Context, options ...repository.Option) ([]Result, error)

	// Exists checks whether any row matches the given options.
	Exists(ctx context.Context, options ...repository.Option) (bool, error)

	// SnippetIDs returns snippet IDs matching the given options.
	SnippetIDs(ctx context.Context, options ...repository.Option) ([]string, error)

	// DeleteBy removes documents matching the given options.
	DeleteBy(ctx context.Context, options ...repository.Option) error
}
