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
	store search.BM25Store
}

// NewBM25 creates a new BM25 service.
func NewBM25(store search.BM25Store) (*BM25, error) {
	if store == nil {
		return nil, fmt.Errorf("NewBM25: nil store")
	}
	return &BM25{
		store: store,
	}, nil
}

// Index indexes documents using domain business rules.
func (s *BM25) Index(ctx context.Context, request search.IndexRequest) error {
	documents := request.Documents()

	// Skip if empty
	if len(documents) == 0 {
		return nil
	}

	// Filter out invalid documents
	valid := make([]search.Document, 0, len(documents))
	for _, doc := range documents {
		if doc.SnippetID() != "" && strings.TrimSpace(doc.Text()) != "" {
			valid = append(valid, doc)
		}
	}

	if len(valid) == 0 {
		return nil
	}

	validRequest := search.NewIndexRequest(valid)
	return s.store.Index(ctx, validRequest)
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

// DeleteBy removes documents matching the given options.
func (s *BM25) DeleteBy(ctx context.Context, options ...repository.Option) error {
	return s.store.DeleteBy(ctx, options...)
}
