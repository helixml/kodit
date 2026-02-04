package service

import (
	"context"
	"strings"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
)

// Embedding provides domain logic for embedding operations.
type Embedding interface {
	// Index indexes documents using domain business rules.
	Index(ctx context.Context, request search.IndexRequest) error

	// Search searches documents using domain business rules.
	Search(ctx context.Context, request search.Request) ([]search.Result, error)

	// HasEmbedding checks if a snippet has an embedding using domain business rules.
	HasEmbedding(ctx context.Context, snippetID string, embeddingType snippet.EmbeddingType) (bool, error)
}

// EmbeddingService implements domain logic for embedding operations.
type EmbeddingService struct {
	store search.VectorStore
}

// NewEmbedding creates a new embedding service.
func NewEmbedding(store search.VectorStore) *EmbeddingService {
	return &EmbeddingService{
		store: store,
	}
}

// Index indexes documents using domain business rules.
func (s *EmbeddingService) Index(ctx context.Context, request search.IndexRequest) error {
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
func (s *EmbeddingService) Search(ctx context.Context, request search.Request) ([]search.Result, error) {
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

// HasEmbedding checks if a snippet has an embedding using domain business rules.
func (s *EmbeddingService) HasEmbedding(ctx context.Context, snippetID string, embeddingType snippet.EmbeddingType) (bool, error) {
	if snippetID == "" {
		return false, nil
	}
	return s.store.HasEmbedding(ctx, snippetID, embeddingType)
}
