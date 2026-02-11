package snippet

import (
	"context"
)

// SnippetStore defines operations for snippet persistence.
type SnippetStore interface {
	// Save persists snippets for a commit.
	Save(ctx context.Context, commitSHA string, snippets []Snippet) error

	// SnippetsForCommit returns all snippets for a specific commit.
	SnippetsForCommit(ctx context.Context, commitSHA string) ([]Snippet, error)

	// DeleteForCommit removes all snippet associations for a commit.
	DeleteForCommit(ctx context.Context, commitSHA string) error

	// ByIDs returns snippets by their SHA identifiers.
	ByIDs(ctx context.Context, ids []string) ([]Snippet, error)

	// BySHA returns a single snippet by its SHA identifier.
	BySHA(ctx context.Context, sha string) (Snippet, error)
}

// CommitIndexStore defines operations for commit index persistence.
type CommitIndexStore interface {
	// Get returns a commit index by SHA.
	Get(ctx context.Context, commitSHA string) (CommitIndex, error)

	// Save persists a commit index.
	Save(ctx context.Context, index CommitIndex) error

	// Delete removes a commit index.
	Delete(ctx context.Context, commitSHA string) error

	// Exists checks if a commit index exists.
	Exists(ctx context.Context, commitSHA string) (bool, error)
}

// EmbeddingType represents the type of embedding.
type EmbeddingType string

// EmbeddingType values.
const (
	EmbeddingTypeCode    EmbeddingType = "code"
	EmbeddingTypeSummary EmbeddingType = "summary"
)

