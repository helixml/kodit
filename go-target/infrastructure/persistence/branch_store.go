package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm/clause"
)

// BranchStore implements repository.BranchStore using GORM.
type BranchStore struct {
	database.Repository[repository.Branch, BranchModel]
}

// NewBranchStore creates a new BranchStore.
func NewBranchStore(db database.Database) BranchStore {
	return BranchStore{
		Repository: database.NewRepository[repository.Branch, BranchModel](db, BranchMapper{}, "branch"),
	}
}

// Save creates or updates a branch.
func (s BranchStore) Save(ctx context.Context, branch repository.Branch) (repository.Branch, error) {
	model := s.Mapper().ToModel(branch)
	model.UpdatedAt = time.Now()

	result := s.DB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"head_commit_sha", "is_default", "updated_at"}),
	}).Create(&model)

	if result.Error != nil {
		return repository.Branch{}, fmt.Errorf("save branch: %w", result.Error)
	}
	return s.Mapper().ToDomain(model), nil
}

// SaveAll creates or updates multiple branches.
func (s BranchStore) SaveAll(ctx context.Context, branches []repository.Branch) ([]repository.Branch, error) {
	if len(branches) == 0 {
		return []repository.Branch{}, nil
	}

	models := make([]BranchModel, len(branches))
	now := time.Now()
	for i, b := range branches {
		models[i] = s.Mapper().ToModel(b)
		models[i].UpdatedAt = now
	}

	result := s.DB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"head_commit_sha", "is_default", "updated_at"}),
	}).Create(&models)

	if result.Error != nil {
		return nil, fmt.Errorf("save branches: %w", result.Error)
	}

	saved := make([]repository.Branch, len(models))
	for i, m := range models {
		saved[i] = s.Mapper().ToDomain(m)
	}
	return saved, nil
}

// Delete removes a branch.
func (s BranchStore) Delete(ctx context.Context, branch repository.Branch) error {
	result := s.DB(ctx).Where("repo_id = ? AND name = ?", branch.RepoID(), branch.Name()).Delete(&BranchModel{})
	if result.Error != nil {
		return fmt.Errorf("delete branch: %w", result.Error)
	}
	return nil
}
