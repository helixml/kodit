package service

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// Cloner handles repository cloning and updating operations.
type Cloner interface {
	// ClonePathFromURI returns the local clone path for a given repository URI.
	ClonePathFromURI(uri string) string

	// Clone clones a repository and returns the local path.
	Clone(ctx context.Context, remoteURI string) (string, error)

	// CloneToPath clones a repository to a specific path.
	CloneToPath(ctx context.Context, remoteURI string, clonePath string) error

	// Update updates a repository based on its tracking configuration.
	// Returns the actual clone path used, which may differ from the stored
	// path if the repository was relocated (e.g. after migration).
	Update(ctx context.Context, repo repository.Repository) (string, error)

	// Ensure clones the repository if it doesn't exist, otherwise pulls latest changes.
	Ensure(ctx context.Context, remoteURI string) (string, error)
}
