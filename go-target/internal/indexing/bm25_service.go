package indexing

import (
	"context"
	"errors"
	"strings"

	"github.com/helixml/kodit/internal/domain"
)

// ErrEmptyQuery indicates an empty search query.
var ErrEmptyQuery = errors.New("search query cannot be empty")

// ErrInvalidTopK indicates an invalid top-k value.
var ErrInvalidTopK = errors.New("top-k must be positive")

// BM25Service provides domain logic for BM25 operations.
type BM25Service struct {
	repository BM25Repository
}

// NewBM25Service creates a new BM25Service.
func NewBM25Service(repository BM25Repository) *BM25Service {
	return &BM25Service{
		repository: repository,
	}
}

// Index indexes documents using domain business rules.
func (s *BM25Service) Index(ctx context.Context, request domain.IndexRequest) error {
	documents := request.Documents()

	// Skip if empty
	if len(documents) == 0 {
		return nil
	}

	// Filter out invalid documents
	valid := make([]domain.Document, 0, len(documents))
	for _, doc := range documents {
		if doc.SnippetID() != "" && strings.TrimSpace(doc.Text()) != "" {
			valid = append(valid, doc)
		}
	}

	if len(valid) == 0 {
		return nil
	}

	validRequest := domain.NewIndexRequest(valid)
	return s.repository.Index(ctx, validRequest)
}

// Search searches documents using domain business rules.
func (s *BM25Service) Search(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error) {
	query := strings.TrimSpace(request.Query())
	if query == "" {
		return nil, ErrEmptyQuery
	}

	if request.TopK() <= 0 {
		return nil, ErrInvalidTopK
	}

	// Create normalized request
	normalizedRequest := domain.NewSearchRequest(query, request.TopK(), request.SnippetIDs())
	return s.repository.Search(ctx, normalizedRequest)
}

// Delete deletes documents using domain business rules.
func (s *BM25Service) Delete(ctx context.Context, request domain.DeleteRequest) error {
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

	validRequest := domain.NewDeleteRequest(valid)
	return s.repository.Delete(ctx, validRequest)
}
