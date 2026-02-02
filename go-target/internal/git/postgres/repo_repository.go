package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/git"
	"gorm.io/gorm"
)

// RepoRepository implements git.RepoRepository using PostgreSQL.
type RepoRepository struct {
	db     database.Database
	mapper RepoMapper
}

// NewRepoRepository creates a new RepoRepository.
func NewRepoRepository(db database.Database) *RepoRepository {
	return &RepoRepository{
		db:     db,
		mapper: RepoMapper{},
	}
}

// Get retrieves a repository by ID.
func (r *RepoRepository) Get(ctx context.Context, id int64) (git.Repo, error) {
	var entity RepoEntity
	result := r.db.Session(ctx).Where("id = ?", id).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return git.Repo{}, fmt.Errorf("%w: id %d", database.ErrNotFound, id)
		}
		return git.Repo{}, fmt.Errorf("get repo: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Find retrieves repositories matching a query.
func (r *RepoRepository) Find(ctx context.Context, query database.Query) ([]git.Repo, error) {
	var entities []RepoEntity
	result := query.Apply(r.db.Session(ctx).Model(&RepoEntity{})).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find repos: %w", result.Error)
	}

	repos := make([]git.Repo, len(entities))
	for i, e := range entities {
		repos[i] = r.mapper.ToDomain(e)
	}
	return repos, nil
}

// FindAll retrieves all repositories.
func (r *RepoRepository) FindAll(ctx context.Context) ([]git.Repo, error) {
	return r.Find(ctx, database.NewQuery())
}

// Save creates or updates a repository.
func (r *RepoRepository) Save(ctx context.Context, repo git.Repo) (git.Repo, error) {
	entity := r.mapper.ToDatabase(repo)

	var result *gorm.DB
	if repo.ID() == 0 {
		result = r.db.Session(ctx).Create(&entity)
	} else {
		result = r.db.Session(ctx).Save(&entity)
	}

	if result.Error != nil {
		return git.Repo{}, fmt.Errorf("save repo: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// Delete removes a repository.
func (r *RepoRepository) Delete(ctx context.Context, repo git.Repo) error {
	entity := r.mapper.ToDatabase(repo)
	result := r.db.Session(ctx).Delete(&entity)
	if result.Error != nil {
		return fmt.Errorf("delete repo: %w", result.Error)
	}
	return nil
}

// GetByRemoteURL retrieves a repository by its remote URL.
func (r *RepoRepository) GetByRemoteURL(ctx context.Context, url string) (git.Repo, error) {
	sanitized := sanitizeRemoteURI(url)
	var entity RepoEntity
	result := r.db.Session(ctx).Where("sanitized_remote_uri = ?", sanitized).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return git.Repo{}, fmt.Errorf("%w: url %s", database.ErrNotFound, url)
		}
		return git.Repo{}, fmt.Errorf("get repo by url: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// ExistsByRemoteURL checks if a repository with the given URL exists.
func (r *RepoRepository) ExistsByRemoteURL(ctx context.Context, url string) (bool, error) {
	sanitized := sanitizeRemoteURI(url)
	var count int64
	result := r.db.Session(ctx).Model(&RepoEntity{}).Where("sanitized_remote_uri = ?", sanitized).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check repo exists: %w", result.Error)
	}
	return count > 0, nil
}
