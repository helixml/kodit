package kodit

import (
	"context"

	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
)

// Snippets provides snippet query operations.
type Snippets interface {
	// ForCommit returns snippets for a specific commit.
	ForCommit(ctx context.Context, commitSHA string) ([]snippet.Snippet, error)

	// BySHA retrieves a single snippet by its SHA.
	BySHA(ctx context.Context, sha string) (snippet.Snippet, error)

	// Embeddings returns embedding info for the given snippet IDs.
	Embeddings(ctx context.Context, snippetIDs []string) ([]snippet.EmbeddingInfo, error)
}

// snippetsImpl implements Snippets.
type snippetsImpl struct {
	snippetStore snippet.SnippetStore
	vectorStore  search.VectorStore
}

func (s *snippetsImpl) ForCommit(ctx context.Context, commitSHA string) ([]snippet.Snippet, error) {
	return s.snippetStore.SnippetsForCommit(ctx, commitSHA)
}

func (s *snippetsImpl) BySHA(ctx context.Context, sha string) (snippet.Snippet, error) {
	return s.snippetStore.BySHA(ctx, sha)
}

func (s *snippetsImpl) Embeddings(ctx context.Context, snippetIDs []string) ([]snippet.EmbeddingInfo, error) {
	if s.vectorStore == nil {
		return []snippet.EmbeddingInfo{}, nil
	}
	return s.vectorStore.EmbeddingsForSnippets(ctx, snippetIDs)
}
