package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

// CommitIndexStore implements snippet.CommitIndexStore using GORM.
type CommitIndexStore struct {
	db     database.Database
	mapper CommitIndexMapper
}

// NewCommitIndexStore creates a new CommitIndexStore.
func NewCommitIndexStore(db database.Database) CommitIndexStore {
	return CommitIndexStore{
		db:     db,
		mapper: CommitIndexMapper{},
	}
}

// Get returns a commit index by SHA.
func (s CommitIndexStore) Get(ctx context.Context, commitSHA string) (snippet.CommitIndex, error) {
	var model CommitIndexModel
	err := s.db.Session(ctx).Where("commit_sha = ?", commitSHA).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return snippet.CommitIndex{}, fmt.Errorf("%w: commit index %s", database.ErrNotFound, commitSHA)
		}
		return snippet.CommitIndex{}, err
	}
	return s.mapper.ToDomain(model), nil
}

// Save persists a commit index.
func (s CommitIndexStore) Save(ctx context.Context, index snippet.CommitIndex) error {
	model := s.mapper.ToModel(index)
	return s.db.Session(ctx).Save(&model).Error
}

// Delete removes a commit index.
func (s CommitIndexStore) Delete(ctx context.Context, commitSHA string) error {
	return s.db.Session(ctx).Where("commit_sha = ?", commitSHA).Delete(&CommitIndexModel{}).Error
}

// Exists checks if a commit index exists.
func (s CommitIndexStore) Exists(ctx context.Context, commitSHA string) (bool, error) {
	var count int64
	err := s.db.Session(ctx).Model(&CommitIndexModel{}).Where("commit_sha = ?", commitSHA).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
