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

// CommitRepository implements git.CommitRepository using PostgreSQL.
type CommitRepository struct {
	db     database.Database
	mapper CommitMapper
}

// NewCommitRepository creates a new CommitRepository.
func NewCommitRepository(db database.Database) *CommitRepository {
	return &CommitRepository{
		db:     db,
		mapper: CommitMapper{},
	}
}

// Get retrieves a commit by ID (not typically used since SHA is primary key).
func (r *CommitRepository) Get(ctx context.Context, id int64) (git.Commit, error) {
	// Commits use SHA as primary key, not integer ID
	// This method exists for interface compatibility but isn't the typical lookup
	return git.Commit{}, fmt.Errorf("%w: commits use SHA, not integer ID", database.ErrNotFound)
}

// Find retrieves commits matching a query.
func (r *CommitRepository) Find(ctx context.Context, query database.Query) ([]git.Commit, error) {
	var entities []CommitEntity
	result := query.Apply(r.db.Session(ctx).Model(&CommitEntity{})).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find commits: %w", result.Error)
	}

	commits := make([]git.Commit, len(entities))
	for i, e := range entities {
		commits[i] = r.mapper.ToDomain(e)
	}
	return commits, nil
}

// Save creates or updates a commit.
func (r *CommitRepository) Save(ctx context.Context, commit git.Commit) (git.Commit, error) {
	entity := r.mapper.ToDatabase(commit)

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_sha"}},
		DoUpdates: clause.AssignmentColumns([]string{"message", "author", "updated_at"}),
	}).Create(&entity)

	if result.Error != nil {
		return git.Commit{}, fmt.Errorf("save commit: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// SaveAll creates or updates multiple commits.
func (r *CommitRepository) SaveAll(ctx context.Context, commits []git.Commit) ([]git.Commit, error) {
	if len(commits) == 0 {
		return nil, nil
	}

	entities := make([]CommitEntity, len(commits))
	for i, c := range commits {
		entities[i] = r.mapper.ToDatabase(c)
	}

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_sha"}},
		DoUpdates: clause.AssignmentColumns([]string{"message", "author", "updated_at"}),
	}).Create(&entities)

	if result.Error != nil {
		return nil, fmt.Errorf("save commits: %w", result.Error)
	}

	saved := make([]git.Commit, len(entities))
	for i, e := range entities {
		saved[i] = r.mapper.ToDomain(e)
	}
	return saved, nil
}

// Delete removes a commit.
func (r *CommitRepository) Delete(ctx context.Context, commit git.Commit) error {
	result := r.db.Session(ctx).Where("commit_sha = ?", commit.SHA()).Delete(&CommitEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete commit: %w", result.Error)
	}
	return nil
}

// GetByRepoAndSHA retrieves a commit by repository ID and SHA.
func (r *CommitRepository) GetByRepoAndSHA(ctx context.Context, repoID int64, sha string) (git.Commit, error) {
	var entity CommitEntity
	result := r.db.Session(ctx).Where("repo_id = ? AND commit_sha = ?", repoID, sha).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return git.Commit{}, fmt.Errorf("%w: sha %s", database.ErrNotFound, sha)
		}
		return git.Commit{}, fmt.Errorf("get commit: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// FindByRepoID retrieves all commits for a repository.
func (r *CommitRepository) FindByRepoID(ctx context.Context, repoID int64) ([]git.Commit, error) {
	var entities []CommitEntity
	result := r.db.Session(ctx).Where("repo_id = ?", repoID).Order("date DESC").Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find commits by repo: %w", result.Error)
	}

	commits := make([]git.Commit, len(entities))
	for i, e := range entities {
		commits[i] = r.mapper.ToDomain(e)
	}
	return commits, nil
}

// ExistsBySHA checks if a commit with the given SHA exists for a repository.
func (r *CommitRepository) ExistsBySHA(ctx context.Context, repoID int64, sha string) (bool, error) {
	var count int64
	result := r.db.Session(ctx).Model(&CommitEntity{}).Where("repo_id = ? AND commit_sha = ?", repoID, sha).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check commit exists: %w", result.Error)
	}
	return count > 0, nil
}
