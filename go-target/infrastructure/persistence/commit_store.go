package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"gorm.io/gorm"
)

// CommitStore implements repository.CommitStore using GORM.
type CommitStore struct {
	db     Database
	mapper CommitMapper
}

// NewCommitStore creates a new CommitStore.
func NewCommitStore(db Database) CommitStore {
	return CommitStore{
		db:     db,
		mapper: CommitMapper{},
	}
}

// Get retrieves a commit by ID.
// Note: Commits use commit_sha as primary key, not integer ID.
func (s CommitStore) Get(ctx context.Context, id int64) (repository.Commit, error) {
	return repository.Commit{}, fmt.Errorf("%w: commits use SHA as identifier", ErrNotFound)
}

// Save creates or updates a commit.
func (s CommitStore) Save(ctx context.Context, commit repository.Commit) (repository.Commit, error) {
	model := s.mapper.ToModel(commit)
	model.UpdatedAt = time.Now()

	result := s.db.Session(ctx).Save(&model)
	if result.Error != nil {
		return repository.Commit{}, fmt.Errorf("save commit: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// SaveAll creates or updates multiple commits.
func (s CommitStore) SaveAll(ctx context.Context, commits []repository.Commit) ([]repository.Commit, error) {
	if len(commits) == 0 {
		return []repository.Commit{}, nil
	}

	models := make([]CommitModel, len(commits))
	now := time.Now()
	for i, c := range commits {
		models[i] = s.mapper.ToModel(c)
		models[i].UpdatedAt = now
	}

	result := s.db.Session(ctx).Save(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("save commits: %w", result.Error)
	}

	saved := make([]repository.Commit, len(models))
	for i, m := range models {
		saved[i] = s.mapper.ToDomain(m)
	}
	return saved, nil
}

// Delete removes a commit.
func (s CommitStore) Delete(ctx context.Context, commit repository.Commit) error {
	result := s.db.Session(ctx).Where("commit_sha = ?", commit.SHA()).Delete(&CommitModel{})
	if result.Error != nil {
		return fmt.Errorf("delete commit: %w", result.Error)
	}
	return nil
}

// GetByRepoAndSHA retrieves a commit by repository ID and SHA.
func (s CommitStore) GetByRepoAndSHA(ctx context.Context, repoID int64, sha string) (repository.Commit, error) {
	var model CommitModel
	result := s.db.Session(ctx).Where("repo_id = ? AND commit_sha = ?", repoID, sha).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return repository.Commit{}, fmt.Errorf("%w: commit %s in repo %d", ErrNotFound, sha, repoID)
		}
		return repository.Commit{}, fmt.Errorf("get commit: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// FindByRepoID retrieves all commits for a repository.
func (s CommitStore) FindByRepoID(ctx context.Context, repoID int64) ([]repository.Commit, error) {
	var models []CommitModel
	result := s.db.Session(ctx).Where("repo_id = ?", repoID).Order("date DESC").Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find commits by repo: %w", result.Error)
	}

	commits := make([]repository.Commit, len(models))
	for i, m := range models {
		commits[i] = s.mapper.ToDomain(m)
	}
	return commits, nil
}

// ExistsBySHA checks if a commit with the given SHA exists in a repository.
func (s CommitStore) ExistsBySHA(ctx context.Context, repoID int64, sha string) (bool, error) {
	var count int64
	result := s.db.Session(ctx).Model(&CommitModel{}).Where("repo_id = ? AND commit_sha = ?", repoID, sha).Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("check commit exists: %w", result.Error)
	}
	return count > 0, nil
}

// Find retrieves commits matching a query.
func (s CommitStore) Find(ctx context.Context, query Query) ([]repository.Commit, error) {
	var models []CommitModel
	result := query.Apply(s.db.Session(ctx).Model(&CommitModel{})).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find commits: %w", result.Error)
	}

	commits := make([]repository.Commit, len(models))
	for i, m := range models {
		commits[i] = s.mapper.ToDomain(m)
	}
	return commits, nil
}
