package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm/clause"
)

// TagStore implements repository.TagStore using GORM.
type TagStore struct {
	database.Repository[repository.Tag, TagModel]
}

// NewTagStore creates a new TagStore.
func NewTagStore(db database.Database) TagStore {
	return TagStore{
		Repository: database.NewRepository[repository.Tag, TagModel](db, TagMapper{}, "tag"),
	}
}

// Save creates or updates a tag.
func (s TagStore) Save(ctx context.Context, tag repository.Tag) (repository.Tag, error) {
	model := s.Mapper().ToModel(tag)
	model.UpdatedAt = time.Now()

	result := s.DB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_commit_sha", "message", "tagger_name", "tagger_email", "tagged_at", "updated_at"}),
	}).Create(&model)

	if result.Error != nil {
		return repository.Tag{}, fmt.Errorf("save tag: %w", result.Error)
	}
	return s.Mapper().ToDomain(model), nil
}

// SaveAll creates or updates multiple tags.
func (s TagStore) SaveAll(ctx context.Context, tags []repository.Tag) ([]repository.Tag, error) {
	if len(tags) == 0 {
		return []repository.Tag{}, nil
	}

	models := make([]TagModel, len(tags))
	now := time.Now()
	for i, t := range tags {
		models[i] = s.Mapper().ToModel(t)
		models[i].UpdatedAt = now
	}

	result := s.DB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_commit_sha", "message", "tagger_name", "tagger_email", "tagged_at", "updated_at"}),
	}).Create(&models)

	if result.Error != nil {
		return nil, fmt.Errorf("save tags: %w", result.Error)
	}

	saved := make([]repository.Tag, len(models))
	for i, m := range models {
		saved[i] = s.Mapper().ToDomain(m)
	}
	return saved, nil
}

// Delete removes a tag.
func (s TagStore) Delete(ctx context.Context, tag repository.Tag) error {
	result := s.DB(ctx).Where("repo_id = ? AND name = ?", tag.RepoID(), tag.Name()).Delete(&TagModel{})
	if result.Error != nil {
		return fmt.Errorf("delete tag: %w", result.Error)
	}
	return nil
}
