package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// ErrEmptyQuery indicates an empty search query.
var ErrEmptyQuery = errors.New("search query cannot be empty")

// ErrInvalidTopK indicates an invalid top-k value.
var ErrInvalidTopK = errors.New("top-k must be positive")

// BM25 provides domain logic for BM25 operations.
type BM25 struct {
	store search.Store
}

// NewBM25 creates a new BM25 service.
func NewBM25(store search.Store) (*BM25, error) {
	if store == nil {
		return nil, fmt.Errorf("NewBM25: nil store")
	}
	return &BM25{
		store: store,
	}, nil
}

// Index indexes documents using domain business rules.
func (s *BM25) Index(ctx context.Context, docs []search.Document) error {
	if len(docs) == 0 {
		return nil
	}

	valid := make([]search.Document, 0, len(docs))
	for _, doc := range docs {
		if doc.SnippetID() != "" && strings.TrimSpace(doc.Text()) != "" {
			valid = append(valid, doc)
		}
	}

	if len(valid) == 0 {
		return nil
	}

	return s.store.Index(ctx, valid)
}

// Find performs BM25 keyword search.
func (s *BM25) Find(ctx context.Context, query string, options ...repository.Option) ([]search.Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuery
	}

	combined := make([]repository.Option, 0, len(options)+1)
	combined = append(combined, search.WithQuery(query))
	combined = append(combined, options...)

	return s.store.Find(ctx, combined...)
}

// ExistingIDs returns the subset of ids whose snippet IDs already have
// BM25 entries in the underlying store.
func (s *BM25) ExistingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	return search.ExistingSnippetIDs(ctx, s.store, ids)
}

// DeleteBy removes documents matching the given options.
func (s *BM25) DeleteBy(ctx context.Context, options ...repository.Option) error {
	return s.store.DeleteBy(ctx, options...)
}
