package snippet

import (
	"context"
)

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
