package service

import (
	"context"
	"fmt"
)

// Conversion indexes a local directory and returns the kodit repository ID.
// The caller is responsible for persisting the returned repo ID for future
// search and delete operations.
type Conversion interface {
	Convert(ctx context.Context, localPath string) (int64, error)
}

// Indexer is a no-op for kodit: the background worker handles chunking and
// embedding as part of the Convert step.
type Indexer interface {
	Index(ctx context.Context, repoID int64) error
}

// RAG provides directory-based RAG indexing and search backed by kodit.
// It implements both Conversion and Indexer.
type RAG struct {
	repos  *Repository
	search *Search
}

// NewRAG creates a new RAG service wrapping the given repository and search services.
func NewRAG(repos *Repository, search *Search) *RAG {
	return &RAG{repos: repos, search: search}
}

// Convert indexes the given local directory into kodit and returns its repository ID.
// Uses a file:// URI so kodit skips git cloning and reads the directory directly.
// Idempotent: if the directory is already indexed, the existing repository ID is returned.
func (r *RAG) Convert(ctx context.Context, localPath string) (int64, error) {
	source, _, err := r.repos.Add(ctx, &RepositoryAddParams{
		URL: "file://" + localPath,
	})
	if err != nil {
		return 0, fmt.Errorf("index directory %s: %w", localPath, err)
	}
	return source.ID(), nil
}

// Index is a no-op. Kodit's background worker handles chunking and embedding
// automatically after Convert is called.
func (r *RAG) Index(_ context.Context, _ int64) error {
	return nil
}

// Search performs a semantic search scoped to the given kodit repository.
func (r *RAG) Search(ctx context.Context, repoID int64, query string, opts ...SearchOption) (SearchResult, error) {
	opts = append([]SearchOption{WithRepositories(repoID)}, opts...)
	return r.search.Query(ctx, query, opts...)
}
