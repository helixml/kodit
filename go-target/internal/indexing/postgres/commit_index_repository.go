package postgres

import (
	"context"
	"errors"

	"github.com/helixml/kodit/internal/indexing"
	"gorm.io/gorm"
)

// ErrCommitIndexNotFound indicates a commit index was not found.
var ErrCommitIndexNotFound = errors.New("commit index not found")

// CommitIndexRepository implements indexing.CommitIndexRepository using GORM.
type CommitIndexRepository struct {
	db     *gorm.DB
	mapper CommitIndexMapper
}

// NewCommitIndexRepository creates a new CommitIndexRepository.
func NewCommitIndexRepository(db *gorm.DB) *CommitIndexRepository {
	return &CommitIndexRepository{
		db:     db,
		mapper: CommitIndexMapper{},
	}
}

// Get returns a commit index by SHA.
func (r *CommitIndexRepository) Get(ctx context.Context, commitSHA string) (indexing.CommitIndex, error) {
	var entity CommitIndexEntity
	result := r.db.WithContext(ctx).Where("commit_sha = ?", commitSHA).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return indexing.CommitIndex{}, ErrCommitIndexNotFound
		}
		return indexing.CommitIndex{}, result.Error
	}
	return r.mapper.ToDomain(entity), nil
}

// Save persists a commit index.
func (r *CommitIndexRepository) Save(ctx context.Context, index indexing.CommitIndex) error {
	entity := r.mapper.ToEntity(index)

	// Use upsert pattern
	result := r.db.WithContext(ctx).Save(&entity)
	return result.Error
}

// Delete removes a commit index.
func (r *CommitIndexRepository) Delete(ctx context.Context, commitSHA string) error {
	result := r.db.WithContext(ctx).Where("commit_sha = ?", commitSHA).Delete(&CommitIndexEntity{})
	return result.Error
}

// Exists checks if a commit index exists.
func (r *CommitIndexRepository) Exists(ctx context.Context, commitSHA string) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&CommitIndexEntity{}).Where("commit_sha = ?", commitSHA).Count(&count)
	if result.Error != nil {
		return false, result.Error
	}
	return count > 0, nil
}
