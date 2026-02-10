package service

import (
	"context"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
)

// SnippetListParams configures snippet listing.
type SnippetListParams struct {
	CommitSHA string
}

// Snippet provides snippet query operations.
type Snippet struct {
	snippetStore snippet.SnippetStore
	vectorStore  search.VectorStore
}

// NewSnippet creates a new Snippet service.
func NewSnippet(snippetStore snippet.SnippetStore, vectorStore search.VectorStore) *Snippet {
	return &Snippet{
		snippetStore: snippetStore,
		vectorStore:  vectorStore,
	}
}

// List returns snippets for a commit.
func (s *Snippet) List(ctx context.Context, params *SnippetListParams) ([]snippet.Snippet, error) {
	return s.snippetStore.SnippetsForCommit(ctx, params.CommitSHA)
}

// BySHA retrieves a single snippet by its SHA.
func (s *Snippet) BySHA(ctx context.Context, sha string) (snippet.Snippet, error) {
	return s.snippetStore.BySHA(ctx, sha)
}

// Embeddings returns embedding info for the given snippet IDs.
func (s *Snippet) Embeddings(ctx context.Context, snippetIDs []string) ([]snippet.EmbeddingInfo, error) {
	if s.vectorStore == nil {
		return []snippet.EmbeddingInfo{}, nil
	}
	return s.vectorStore.EmbeddingsForSnippets(ctx, snippetIDs)
}
