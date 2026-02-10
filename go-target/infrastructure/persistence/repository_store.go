package persistence

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

// RepositoryStore implements repository.RepositoryStore using GORM.
type RepositoryStore struct {
	database.Repository[repository.Repository, RepositoryModel]
}

// NewRepositoryStore creates a new RepositoryStore.
func NewRepositoryStore(db database.Database) RepositoryStore {
	return RepositoryStore{
		Repository: database.NewRepository[repository.Repository, RepositoryModel](db, RepositoryMapper{}, "repository"),
	}
}

// Save creates or updates a repository.
func (s RepositoryStore) Save(ctx context.Context, repo repository.Repository) (repository.Repository, error) {
	model := s.Mapper().ToModel(repo)

	var result *gorm.DB
	if repo.ID() == 0 {
		result = s.DB(ctx).Create(&model)
	} else {
		result = s.DB(ctx).Save(&model)
	}

	if result.Error != nil {
		return repository.Repository{}, fmt.Errorf("save repository: %w", result.Error)
	}
	return s.Mapper().ToDomain(model), nil
}

// Delete removes a repository.
func (s RepositoryStore) Delete(ctx context.Context, repo repository.Repository) error {
	model := s.Mapper().ToModel(repo)
	result := s.DB(ctx).Delete(&model)
	if result.Error != nil {
		return fmt.Errorf("delete repository: %w", result.Error)
	}
	return nil
}
