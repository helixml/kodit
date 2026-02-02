package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/git"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BranchRepository implements git.BranchRepository using PostgreSQL.
type BranchRepository struct {
	db     database.Database
	mapper BranchMapper
}

// NewBranchRepository creates a new BranchRepository.
func NewBranchRepository(db database.Database) *BranchRepository {
	return &BranchRepository{
		db:     db,
		mapper: BranchMapper{},
	}
}

// Get retrieves a branch by ID (not typically used since composite key).
func (r *BranchRepository) Get(ctx context.Context, id int64) (git.Branch, error) {
	// Branches use composite key (repo_id, name), not integer ID
	return git.Branch{}, fmt.Errorf("%w: branches use composite key", database.ErrNotFound)
}

// Find retrieves branches matching a query.
func (r *BranchRepository) Find(ctx context.Context, query database.Query) ([]git.Branch, error) {
	var entities []BranchEntity
	result := query.Apply(r.db.Session(ctx).Model(&BranchEntity{})).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find branches: %w", result.Error)
	}

	branches := make([]git.Branch, len(entities))
	for i, e := range entities {
		branches[i] = r.mapper.ToDomain(e)
	}
	return branches, nil
}

// Save creates or updates a branch.
func (r *BranchRepository) Save(ctx context.Context, branch git.Branch) (git.Branch, error) {
	entity := r.mapper.ToDatabase(branch)

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"head_commit_sha", "is_default", "updated_at"}),
	}).Create(&entity)

	if result.Error != nil {
		return git.Branch{}, fmt.Errorf("save branch: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// SaveAll creates or updates multiple branches.
func (r *BranchRepository) SaveAll(ctx context.Context, branches []git.Branch) ([]git.Branch, error) {
	if len(branches) == 0 {
		return nil, nil
	}

	entities := make([]BranchEntity, len(branches))
	for i, b := range branches {
		entities[i] = r.mapper.ToDatabase(b)
	}

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"head_commit_sha", "is_default", "updated_at"}),
	}).Create(&entities)

	if result.Error != nil {
		return nil, fmt.Errorf("save branches: %w", result.Error)
	}

	saved := make([]git.Branch, len(entities))
	for i, e := range entities {
		saved[i] = r.mapper.ToDomain(e)
	}
	return saved, nil
}

// Delete removes a branch.
func (r *BranchRepository) Delete(ctx context.Context, branch git.Branch) error {
	result := r.db.Session(ctx).Where("repo_id = ? AND name = ?", branch.RepoID(), branch.Name()).Delete(&BranchEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete branch: %w", result.Error)
	}
	return nil
}

// GetByName retrieves a branch by repository ID and name.
func (r *BranchRepository) GetByName(ctx context.Context, repoID int64, name string) (git.Branch, error) {
	var entity BranchEntity
	result := r.db.Session(ctx).Where("repo_id = ? AND name = ?", repoID, name).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return git.Branch{}, fmt.Errorf("%w: branch %s", database.ErrNotFound, name)
		}
		return git.Branch{}, fmt.Errorf("get branch: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// FindByRepoID retrieves all branches for a repository.
func (r *BranchRepository) FindByRepoID(ctx context.Context, repoID int64) ([]git.Branch, error) {
	var entities []BranchEntity
	result := r.db.Session(ctx).Where("repo_id = ?", repoID).Order("name ASC").Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find branches by repo: %w", result.Error)
	}

	branches := make([]git.Branch, len(entities))
	for i, e := range entities {
		branches[i] = r.mapper.ToDomain(e)
	}
	return branches, nil
}

// GetDefaultBranch retrieves the default branch for a repository.
func (r *BranchRepository) GetDefaultBranch(ctx context.Context, repoID int64) (git.Branch, error) {
	var entity BranchEntity
	result := r.db.Session(ctx).Where("repo_id = ? AND is_default = ?", repoID, true).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return git.Branch{}, fmt.Errorf("%w: no default branch", database.ErrNotFound)
		}
		return git.Branch{}, fmt.Errorf("get default branch: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}
