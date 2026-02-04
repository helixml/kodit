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

// TagStore implements repository.TagStore using GORM.
type TagStore struct {
	db     Database
	mapper TagMapper
}

// NewTagStore creates a new TagStore.
func NewTagStore(db Database) TagStore {
	return TagStore{
		db:     db,
		mapper: TagMapper{},
	}
}

// Get retrieves a tag by ID.
// Note: Tags use composite key (repo_id, name), not integer ID.
func (s TagStore) Get(ctx context.Context, id int64) (repository.Tag, error) {
	return repository.Tag{}, fmt.Errorf("%w: tags use composite key", ErrNotFound)
}

// Save creates or updates a tag.
func (s TagStore) Save(ctx context.Context, tag repository.Tag) (repository.Tag, error) {
	model := s.mapper.ToModel(tag)
	model.UpdatedAt = time.Now()

	result := s.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_commit_sha", "message", "tagger_name", "tagger_email", "tagged_at", "updated_at"}),
	}).Create(&model)

	if result.Error != nil {
		return repository.Tag{}, fmt.Errorf("save tag: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// SaveAll creates or updates multiple tags.
func (s TagStore) SaveAll(ctx context.Context, tags []repository.Tag) ([]repository.Tag, error) {
	if len(tags) == 0 {
		return []repository.Tag{}, nil
	}

	models := make([]TagModel, len(tags))
	now := time.Now()
	for i, t := range tags {
		models[i] = s.mapper.ToModel(t)
		models[i].UpdatedAt = now
	}

	result := s.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_commit_sha", "message", "tagger_name", "tagger_email", "tagged_at", "updated_at"}),
	}).Create(&models)

	if result.Error != nil {
		return nil, fmt.Errorf("save tags: %w", result.Error)
	}

	saved := make([]repository.Tag, len(models))
	for i, m := range models {
		saved[i] = s.mapper.ToDomain(m)
	}
	return saved, nil
}

// Delete removes a tag.
func (s TagStore) Delete(ctx context.Context, tag repository.Tag) error {
	result := s.db.Session(ctx).Where("repo_id = ? AND name = ?", tag.RepoID(), tag.Name()).Delete(&TagModel{})
	if result.Error != nil {
		return fmt.Errorf("delete tag: %w", result.Error)
	}
	return nil
}

// GetByName retrieves a tag by repository ID and name.
func (s TagStore) GetByName(ctx context.Context, repoID int64, name string) (repository.Tag, error) {
	var model TagModel
	result := s.db.Session(ctx).Where("repo_id = ? AND name = ?", repoID, name).First(&model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return repository.Tag{}, fmt.Errorf("%w: tag %s in repo %d", ErrNotFound, name, repoID)
		}
		return repository.Tag{}, fmt.Errorf("get tag: %w", result.Error)
	}
	return s.mapper.ToDomain(model), nil
}

// FindByRepoID retrieves all tags for a repository.
func (s TagStore) FindByRepoID(ctx context.Context, repoID int64) ([]repository.Tag, error) {
	var models []TagModel
	result := s.db.Session(ctx).Where("repo_id = ?", repoID).Order("name ASC").Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find tags by repo: %w", result.Error)
	}

	tags := make([]repository.Tag, len(models))
	for i, m := range models {
		tags[i] = s.mapper.ToDomain(m)
	}
	return tags, nil
}

// Find retrieves tags matching a query.
func (s TagStore) Find(ctx context.Context, query Query) ([]repository.Tag, error) {
	var models []TagModel
	result := query.Apply(s.db.Session(ctx).Model(&TagModel{})).Find(&models)
	if result.Error != nil {
		return nil, fmt.Errorf("find tags: %w", result.Error)
	}

	tags := make([]repository.Tag, len(models))
	for i, m := range models {
		tags[i] = s.mapper.ToDomain(m)
	}
	return tags, nil
}
