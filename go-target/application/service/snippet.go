package service

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
)

// SnippetListParams configures snippet listing.
type SnippetListParams struct {
	CommitSHA string
	Limit     int
	Offset    int
}

// Snippet provides snippet query operations.
type Snippet struct {
	snippetStore snippet.SnippetStore
}

// NewSnippet creates a new Snippet service.
func NewSnippet(snippetStore snippet.SnippetStore) *Snippet {
	return &Snippet{
		snippetStore: snippetStore,
	}
}

// List returns snippets for a commit.
func (s *Snippet) List(ctx context.Context, params *SnippetListParams) ([]snippet.Snippet, error) {
	var options []repository.Option
	if params.Limit > 0 {
		options = append(options, repository.WithPagination(params.Limit, params.Offset)...)
	}
	return s.snippetStore.SnippetsForCommit(ctx, params.CommitSHA, options...)
}

// Count returns the total number of snippets for a commit.
func (s *Snippet) Count(ctx context.Context, params *SnippetListParams) (int64, error) {
	return s.snippetStore.CountForCommit(ctx, params.CommitSHA)
}

// BySHA retrieves a single snippet by its SHA.
func (s *Snippet) BySHA(ctx context.Context, sha string) (snippet.Snippet, error) {
	return s.snippetStore.BySHA(ctx, sha)
}
