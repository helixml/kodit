package service

import (
	"context"

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
