package search

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// EmbeddingStore defines persistence operations for vector embeddings.
type EmbeddingStore interface {
	// SaveAll persists pre-computed embeddings.
	SaveAll(ctx context.Context, embeddings []Embedding) error

	// Find retrieves embeddings matching the given options.
	Find(ctx context.Context, options ...repository.Option) ([]Embedding, error)

	// FindAll retrieves all embeddings matching the given search filters,
	// including repository source filtering via enrichment association JOINs.
	FindAll(ctx context.Context, filters Filters) ([]Embedding, error)

	// Search performs vector similarity search using options.
	// Embedding must be passed via WithEmbedding.
	Search(ctx context.Context, options ...repository.Option) ([]Result, error)

	// Exists checks whether any row matches the given options.
	Exists(ctx context.Context, options ...repository.Option) (bool, error)

	// DeleteBy removes documents matching the given options.
	DeleteBy(ctx context.Context, options ...repository.Option) error
}
