package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
)

// Embedding provides domain logic for embedding operations.
type Embedding interface {
	// Index indexes documents using domain business rules.
	Index(ctx context.Context, request search.IndexRequest, opts ...search.IndexOption) error

	// Find embeds the query text and performs vector similarity search.
	Find(ctx context.Context, query string, options ...repository.Option) ([]search.Result, error)

	// Exists checks whether any row matches the given options.
	Exists(ctx context.Context, options ...repository.Option) (bool, error)
}

// EmbeddingService implements domain logic for embedding operations.
type EmbeddingService struct {
	store    search.EmbeddingStore
	embedder search.Embedder
	budget   search.TokenBudget
}

// NewEmbedding creates a new embedding service.
// The budget controls text truncation and adaptive batching.
func NewEmbedding(store search.EmbeddingStore, embedder search.Embedder, budget search.TokenBudget) (*EmbeddingService, error) {
	if store == nil {
		return nil, fmt.Errorf("NewEmbedding: nil store")
	}
	return &EmbeddingService{
		store:    store,
		embedder: embedder,
		budget:   budget,
	}, nil
}

// Index indexes documents using domain business rules:
// validate → deduplicate against store → batch embed → batch save.
func (s *EmbeddingService) Index(ctx context.Context, request search.IndexRequest, opts ...search.IndexOption) error {
	cfg := search.NewIndexConfig(opts...)

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

	found, err := s.store.Find(ctx, search.WithSnippetIDs(ids))
	if err != nil {
		return fmt.Errorf("check existing: %w", err)
	}

	existing := make(map[string]struct{}, len(found))
	for _, emb := range found {
		existing[emb.SnippetID()] = struct{}{}
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

	batches := s.budget.Batches(toEmbed)
	total := len(toEmbed)
	completed := 0
	offset := 0
	var batchErrors []error

	for _, batch := range batches {
		if err := ctx.Err(); err != nil {
			return err
		}

		start := offset
		end := offset + len(batch)

		texts := make([]string, len(batch))
		for j, doc := range batch {
			texts[j] = s.budget.Truncate(doc.Text())
		}

		vectors, err := s.embedder.Embed(ctx, texts)
		if err != nil {
			batchErr := fmt.Errorf("embed batch [%d:%d]: %w", start, end, err)
			batchErrors = append(batchErrors, batchErr)
			if cfg.BatchError() != nil {
				cfg.BatchError()(start, end, err)
			}
			offset = end
			continue
		}

		if len(vectors) != len(batch) {
			batchErr := fmt.Errorf("embed batch [%d:%d]: count mismatch: got %d, expected %d", start, end, len(vectors), len(batch))
			batchErrors = append(batchErrors, batchErr)
			if cfg.BatchError() != nil {
				cfg.BatchError()(start, end, fmt.Errorf("count mismatch: got %d, expected %d", len(vectors), len(batch)))
			}
			offset = end
			continue
		}

		embeddings := make([]search.Embedding, len(batch))
		for j, doc := range batch {
			embeddings[j] = search.NewEmbedding(doc.SnippetID(), vectors[j])
		}

		if err := s.store.SaveAll(ctx, embeddings); err != nil {
			batchErr := fmt.Errorf("save batch [%d:%d]: %w", start, end, err)
			batchErrors = append(batchErrors, batchErr)
			if cfg.BatchError() != nil {
				cfg.BatchError()(start, end, err)
			}
			offset = end
			continue
		}

		completed += len(batch)
		if cfg.Progress() != nil {
			cfg.Progress()(completed, total)
		}
		offset = end
	}

	if len(batchErrors) > 0 {
		return fmt.Errorf("%d of %d embedding batches failed: %w", len(batchErrors), len(batches), errors.Join(batchErrors...))
	}

	return nil
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

	return s.store.Search(ctx, combined...)
}

// Exists checks whether any row matches the given options.
func (s *EmbeddingService) Exists(ctx context.Context, options ...repository.Option) (bool, error) {
	return s.store.Exists(ctx, options...)
}
