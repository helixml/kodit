package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BranchStore implements repository.BranchStore using GORM.
type BranchStore struct {
	db     Database
	mapper BranchMapper
}

// NewBranchStore creates a new BranchStore.
func NewBranchStore(db Database) BranchStore {
	return BranchStore{
		db:     db,
		mapper: BranchMapper{},
	}
}

// Get retrieves a branch by ID.
// Note: Branches use composite key (repo_id, name), not integer ID.
func (s BranchStore) Get(ctx context.Context, id int64) (repository.Branch, error) {
	return repository.Branch{}, fmt.Errorf("%w: branches use composite key", ErrNotFound)
}

// Save creates or updates a branch.
func (s BranchStore) Save(ctx context.Context, branch repository.Branch) (repository.Branch, error) {
	model := s.mapper.ToModel(branch)
	model.UpdatedAt = time.Now()

	result := s.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"head_commit_sha", "is_default", "updated_at"}),
	}).Create(&model)

	if result.Error != nil {
		return repository.Branch{}, fmt.Errorf("save branch: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// SaveAll creates or updates multiple branches.
func (s BranchStore) SaveAll(ctx context.Context, branches []repository.Branch) ([]repository.Branch, error) {
	if len(branches) == 0 {
		return []repository.Branch{}, nil
	}

	models := make([]BranchModel, len(branches))
	now := time.Now()
	for i, b := range branches {
		models[i] = s.mapper.ToModel(b)
		models[i].UpdatedAt = now
	}

	result := s.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"head_commit_sha", "is_default", "updated_at"}),
	}).Create(&models)

	if result.Error != nil {
		return nil, fmt.Errorf("save branches: %w", result.Error)
	}

	saved := make([]repository.Branch, len(models))
	for i, m := range models {
		saved[i] = s.mapper.ToDomain(m)
	}
	return saved, nil
}

// Delete removes a branch.
func (s BranchStore) Delete(ctx context.Context, branch repository.Branch) error {
	result := s.db.Session(ctx).Where("repo_id = ? AND name = ?", branch.RepoID(), branch.Name()).Delete(&BranchModel{})
	if result.Error != nil {
		return fmt.Errorf("delete branch: %w", result.Error)
	}
	return nil
}

// GetByName retrieves a branch by repository ID and name.
func (s BranchStore) GetByName(ctx context.Context, repoID int64, name string) (repository.Branch, error) {
	var model BranchModel
	result := s.db.Session(ctx).Where("repo_id = ? AND name = ?", repoID, name).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return repository.Branch{}, fmt.Errorf("%w: branch %s in repo %d", ErrNotFound, name, repoID)
		}
		return repository.Branch{}, fmt.Errorf("get branch: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// FindByRepoID retrieves all branches for a repository.
func (s BranchStore) FindByRepoID(ctx context.Context, repoID int64) ([]repository.Branch, error) {
	var models []BranchModel
	result := s.db.Session(ctx).Where("repo_id = ?", repoID).Order("name ASC").Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find branches by repo: %w", result.Error)
	}

	branches := make([]repository.Branch, len(models))
	for i, m := range models {
		branches[i] = s.mapper.ToDomain(m)
	}
	return branches, nil
}

// GetDefaultBranch retrieves the default branch for a repository.
func (s BranchStore) GetDefaultBranch(ctx context.Context, repoID int64) (repository.Branch, error) {
	var model BranchModel
	result := s.db.Session(ctx).Where("repo_id = ? AND is_default = ?", repoID, true).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return repository.Branch{}, fmt.Errorf("%w: no default branch for repo %d", ErrNotFound, repoID)
		}
		return repository.Branch{}, fmt.Errorf("get default branch: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// Find retrieves branches matching a query.
func (s BranchStore) Find(ctx context.Context, query Query) ([]repository.Branch, error) {
	var models []BranchModel
	result := query.Apply(s.db.Session(ctx).Model(&BranchModel{})).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find branches: %w", result.Error)
	}

	branches := make([]repository.Branch, len(models))
	for i, m := range models {
		branches[i] = s.mapper.ToDomain(m)
	}
	return branches, nil
}
