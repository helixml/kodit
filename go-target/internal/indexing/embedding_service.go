package indexing

import (
	"context"
	"errors"
	"strings"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/provider"
)

// EmbeddingService provides domain logic for embedding operations.
type EmbeddingService struct {
	embeddingProvider provider.Provider
	vectorRepository  VectorSearchRepository
}

// NewEmbeddingService creates a new EmbeddingService.
func NewEmbeddingService(
	embeddingProvider provider.Provider,
	vectorRepository VectorSearchRepository,
) *EmbeddingService {
	return &EmbeddingService{
		embeddingProvider: embeddingProvider,
		vectorRepository:  vectorRepository,
	}
}

// Index indexes documents using domain business rules.
func (s *EmbeddingService) Index(ctx context.Context, request domain.IndexRequest) error {
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
	return s.vectorRepository.Index(ctx, validRequest)
}

// Search searches documents using domain business rules.
func (s *EmbeddingService) Search(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error) {
	query := strings.TrimSpace(request.Query())
	if query == "" {
		return nil, ErrEmptyQuery
	}

	if request.TopK() <= 0 {
		return nil, ErrInvalidTopK
	}

	// Create normalized request
	normalizedRequest := domain.NewSearchRequest(query, request.TopK(), request.SnippetIDs())
	results, err := s.vectorRepository.Search(ctx, normalizedRequest)
	if err != nil {
		return nil, err
	}

	// Deduplicate results while preserving order and scores
	seen := make(map[string]bool)
	unique := make([]domain.SearchResult, 0, len(results))
	for _, result := range results {
		if !seen[result.SnippetID()] {
			seen[result.SnippetID()] = true
			unique = append(unique, result)
		}
	}

	return unique, nil
}

// HasEmbedding checks if a snippet has an embedding using domain business rules.
func (s *EmbeddingService) HasEmbedding(ctx context.Context, snippetID string, embeddingType EmbeddingType) (bool, error) {
	if snippetID == "" {
		return false, ErrInvalidSnippetID
	}

	return s.vectorRepository.HasEmbedding(ctx, snippetID, embeddingType)
}

// ErrInvalidSnippetID indicates an invalid snippet ID.
var ErrInvalidSnippetID = errors.New("snippet ID must be non-empty")
