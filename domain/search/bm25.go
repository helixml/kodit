package search

import "context"

// BM25Store defines operations for BM25 full-text search indexing.
type BM25Store interface {
	// Index adds documents to the BM25 index.
	Index(ctx context.Context, request IndexRequest) error

	// Search performs BM25 keyword search.
	Search(ctx context.Context, request Request) ([]Result, error)

	// Delete removes documents from the BM25 index.
	Delete(ctx context.Context, request DeleteRequest) error
}
