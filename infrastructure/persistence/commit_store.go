package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/database"
)

// CommitStore implements repository.CommitStore using GORM.
type CommitStore struct {
	database.Repository[repository.Commit, CommitModel]
}

// NewCommitStore creates a new CommitStore.
func NewCommitStore(db database.Database) CommitStore {
	return CommitStore{
		Repository: database.NewRepository[repository.Commit, CommitModel](db, CommitMapper{}, "commit"),
	}
}

// Save creates or updates a commit.
func (s CommitStore) Save(ctx context.Context, commit repository.Commit) (repository.Commit, error) {
	model := s.Mapper().ToModel(commit)
	model.UpdatedAt = time.Now()

	result := s.DB(ctx).Save(&model)
	if result.Error != nil {
		return repository.Commit{}, fmt.Errorf("save commit: %w", result.Error)
	}
	return s.Mapper().ToDomain(model), nil
}

// SaveAll creates or updates multiple commits.
func (s CommitStore) SaveAll(ctx context.Context, commits []repository.Commit) ([]repository.Commit, error) {
	if len(commits) == 0 {
		return []repository.Commit{}, nil
	}

	models := make([]CommitModel, len(commits))
	now := time.Now()
	for i, c := range commits {
		models[i] = s.Mapper().ToModel(c)
		models[i].UpdatedAt = now
	}

	result := s.DB(ctx).Save(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("save commits: %w", result.Error)
	}

	saved := make([]repository.Commit, len(models))
	for i, m := range models {
		saved[i] = s.Mapper().ToDomain(m)
	}
	return saved, nil
}

// Delete removes a commit.
func (s CommitStore) Delete(ctx context.Context, commit repository.Commit) error {
	result := s.DB(ctx).Where("commit_sha = ?", commit.SHA()).Delete(&CommitModel{})
	if result.Error != nil {
		return fmt.Errorf("delete commit: %w", result.Error)
	}
	return nil
}
