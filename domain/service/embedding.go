package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// Embedding provides domain logic for embedding operations.
type Embedding interface {
	// Index indexes documents using domain business rules.
	Index(ctx context.Context, request search.IndexRequest) error

	// Find embeds the query text and performs vector similarity search.
	Find(ctx context.Context, query string, options ...repository.Option) ([]search.Result, error)

	// Exists checks whether any row matches the given options.
	Exists(ctx context.Context, options ...repository.Option) (bool, error)

	// SnippetIDs returns snippet IDs matching the given options.
	SnippetIDs(ctx context.Context, options ...repository.Option) ([]string, error)
}

// EmbeddingService implements domain logic for embedding operations.
type EmbeddingService struct {
	store    search.EmbeddingStore
	embedder search.Embedder
}

// NewEmbedding creates a new embedding service.
func NewEmbedding(store search.EmbeddingStore, embedder search.Embedder) (*EmbeddingService, error) {
	if store == nil {
		return nil, fmt.Errorf("NewEmbedding: nil store")
	}
	return &EmbeddingService{
		store:    store,
		embedder: embedder,
	}, nil
}

// Index indexes documents using domain business rules:
// validate → deduplicate against store → embed → save.
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

	// Deduplicate: find which snippet IDs already exist
	ids := make([]string, len(valid))
	for i, doc := range valid {
		ids[i] = doc.SnippetID()
	}

	existingIDs, err := s.store.SnippetIDs(ctx, search.WithSnippetIDs(ids))
	if err != nil {
		return fmt.Errorf("check existing: %w", err)
	}

	existing := make(map[string]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		existing[id] = struct{}{}
	}

	var toEmbed []search.Document
	for _, doc := range valid {
		if _, ok := existing[doc.SnippetID()]; !ok {
			toEmbed = append(toEmbed, doc)
		}
	}

	if len(toEmbed) == 0 {
		return nil
	}

	// Embed
	if s.embedder == nil {
		return fmt.Errorf("Index: nil embedder")
	}

	texts := make([]string, len(toEmbed))
	for i, doc := range toEmbed {
		texts[i] = doc.Text()
	}

	vectors, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("generate embeddings: %w", err)
	}

	if len(vectors) != len(toEmbed) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(vectors), len(toEmbed))
	}

	// Build domain embeddings
	embeddings := make([]search.Embedding, len(toEmbed))
	for i, doc := range toEmbed {
		embeddings[i] = search.NewEmbedding(doc.SnippetID(), vectors[i])
	}

	return s.store.SaveAll(ctx, embeddings)
}

// Find embeds the query text and performs vector similarity search.
func (s *EmbeddingService) Find(ctx context.Context, query string, options ...repository.Option) ([]search.Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuery
	}

	if s.embedder == nil {
		return nil, fmt.Errorf("Find: nil embedder")
	}

	embeddings, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return []search.Result{}, nil
	}

	combined := make([]repository.Option, 0, len(options)+1)
	combined = append(combined, search.WithEmbedding(embeddings[0]))
	combined = append(combined, options...)

	return s.store.Find(ctx, combined...)
}

// Exists checks whether any row matches the given options.
func (s *EmbeddingService) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	return s.store.Exists(ctx, options...)
}

// SnippetIDs returns snippet IDs matching the given options.
func (s *EmbeddingService) SnippetIDs(ctx context.Context, options ...repository.Option) ([]string, error) {
	return s.store.SnippetIDs(ctx, options...)
}
