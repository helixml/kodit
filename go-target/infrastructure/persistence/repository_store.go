package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
	"gorm.io/gorm"
)

// RepositoryStore implements repository.RepositoryStore using GORM.
type RepositoryStore struct {
	db     Database
	mapper RepositoryMapper
}

// NewRepositoryStore creates a new RepositoryStore.
func NewRepositoryStore(db Database) RepositoryStore {
	return RepositoryStore{
		db:     db,
		mapper: RepositoryMapper{},
	}
}

// Get retrieves a repository by ID.
func (s RepositoryStore) Get(ctx context.Context, id int64) (repository.Repository, error) {
	var model RepositoryModel
	result := s.db.Session(ctx).Where("id = ?", id).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return repository.Repository{}, fmt.Errorf("%w: id %d", ErrNotFound, id)
		}
		return repository.Repository{}, fmt.Errorf("get repository: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// Find retrieves repositories matching a query.
func (s RepositoryStore) Find(ctx context.Context, query Query) ([]repository.Repository, error) {
	var models []RepositoryModel
	result := query.Apply(s.db.Session(ctx).Model(&RepositoryModel{})).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find repositories: %w", result.Error)
	}

	repos := make([]repository.Repository, len(models))
	for i, m := range models {
		repos[i] = s.mapper.ToDomain(m)
	}
	return repos, nil
}

// FindAll retrieves all repositories.
func (s RepositoryStore) FindAll(ctx context.Context) ([]repository.Repository, error) {
	return s.Find(ctx, NewQuery())
}

// Save creates or updates a repository.
func (s RepositoryStore) Save(ctx context.Context, repo repository.Repository) (repository.Repository, error) {
	model := s.mapper.ToModel(repo)

	var result *gorm.DB
	if repo.ID() == 0 {
		result = s.db.Session(ctx).Create(&model)
	} else {
		result = s.db.Session(ctx).Save(&model)
	}

	if result.Error != nil {
		return repository.Repository{}, fmt.Errorf("save repository: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// Delete removes a repository.
func (s RepositoryStore) Delete(ctx context.Context, repo repository.Repository) error {
	model := s.mapper.ToModel(repo)
	result := s.db.Session(ctx).Delete(&model)
	if result.Error != nil {
		return fmt.Errorf("delete repository: %w", result.Error)
	}
	return nil
}

// GetByRemoteURL retrieves a repository by its remote URL.
func (s RepositoryStore) GetByRemoteURL(ctx context.Context, url string) (repository.Repository, error) {
	sanitized := sanitizeRemoteURI(url)
	var model RepositoryModel
	result := s.db.Session(ctx).Where("sanitized_remote_uri = ?", sanitized).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return repository.Repository{}, fmt.Errorf("%w: url %s", ErrNotFound, url)
		}
		return repository.Repository{}, fmt.Errorf("get repository by url: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// ExistsByRemoteURL checks if a repository with the given URL exists.
func (s RepositoryStore) ExistsByRemoteURL(ctx context.Context, url string) (bool, error) {
	sanitized := sanitizeRemoteURI(url)
	var count int64
	result := s.db.Session(ctx).Model(&RepositoryModel{}).Where("sanitized_remote_uri = ?", sanitized).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check repository exists: %w", result.Error)
	}
	return count > 0, nil
}
