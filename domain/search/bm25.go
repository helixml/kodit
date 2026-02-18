package search

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// BM25Store defines operations for BM25 full-text search indexing.
type BM25Store interface {
	// Index adds documents to the BM25 index.
	Index(ctx context.Context, request IndexRequest) error

	// Find performs BM25 keyword search using options.
	// Query text must be passed via WithQuery.
	Find(ctx context.Context, options ...repository.Option) ([]Result, error)

	// DeleteBy removes documents matching the given options.
	DeleteBy(ctx context.Context, options ...repository.Option) error
}
