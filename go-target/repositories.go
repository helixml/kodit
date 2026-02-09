package kodit

import (
	"context"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/repository"
)

// Repositories provides repository management operations.
type Repositories interface {
	// Clone clones a repository and queues it for indexing.
	Clone(ctx context.Context, url string) (repository.Repository, error)

	// Get retrieves a repository by ID.
	Get(ctx context.Context, id int64) (repository.Repository, error)

	// List returns all repositories.
	List(ctx context.Context) ([]repository.Repository, error)

	// Delete removes a repository and all associated data.
	Delete(ctx context.Context, id int64) error

	// Sync triggers re-indexing of a repository.
	Sync(ctx context.Context, id int64) error
}

// repositoriesImpl implements Repositories.
type repositoriesImpl struct {
	repoSync  *service.RepositorySync
	repoQuery *service.RepositoryQuery
}

func (r *repositoriesImpl) Clone(ctx context.Context, url string) (repository.Repository, error) {
	source, err := r.repoSync.AddRepository(ctx, url)
	if err != nil {
		return repository.Repository{}, err
	}
	return source.Repository(), nil
}

func (r *repositoriesImpl) Get(ctx context.Context, id int64) (repository.Repository, error) {
	source, err := r.repoQuery.ByID(ctx, id)
	if err != nil {
		return repository.Repository{}, err
	}
	return source.Repository(), nil
}

func (r *repositoriesImpl) List(ctx context.Context) ([]repository.Repository, error) {
	sources, err := r.repoQuery.All(ctx)
	if err != nil {
		return nil, err
	}
	repos := make([]repository.Repository, len(sources))
	for i, src := range sources {
		repos[i] = src.Repository()
	}
	return repos, nil
}

func (r *repositoriesImpl) Delete(ctx context.Context, id int64) error {
	return r.repoSync.RequestDelete(ctx, id)
}

func (r *repositoriesImpl) Sync(ctx context.Context, id int64) error {
	return r.repoSync.RequestSync(ctx, id)
}
