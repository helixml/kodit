package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// Search searches documents using domain business rules.
func (s *BM25) Search(ctx context.Context, request search.Request) ([]search.Result, error) {
	query := strings.TrimSpace(request.Query())
	if query == "" {
		return nil, ErrEmptyQuery
	}

	if request.TopK() <= 0 {
		return nil, ErrInvalidTopK
	}

	// Create normalized request
	normalizedRequest := search.NewRequest(query, request.TopK(), request.SnippetIDs())
	return s.store.Search(ctx, normalizedRequest)
}

// Delete deletes documents using domain business rules.
func (s *BM25) Delete(ctx context.Context, request search.DeleteRequest) error {
	ids := request.SnippetIDs()

	// Skip if empty
	if len(ids) == 0 {
		return nil
	}

	// Filter out invalid IDs
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && id != "0" && !strings.HasPrefix(id, "-") {
			valid = append(valid, id)
		}
	}

	if len(valid) == 0 {
		return nil
	}

	validRequest := search.NewDeleteRequest(valid)
	return s.store.Delete(ctx, validRequest)
}
