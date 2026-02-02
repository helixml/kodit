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

// TagRepository implements git.TagRepository using PostgreSQL.
type TagRepository struct {
	db     database.Database
	mapper TagMapper
}

// NewTagRepository creates a new TagRepository.
func NewTagRepository(db database.Database) *TagRepository {
	return &TagRepository{
		db:     db,
		mapper: TagMapper{},
	}
}

// Get retrieves a tag by ID (not typically used since composite key).
func (r *TagRepository) Get(ctx context.Context, id int64) (git.Tag, error) {
	// Tags use composite key (repo_id, name), not integer ID
	return git.Tag{}, fmt.Errorf("%w: tags use composite key", database.ErrNotFound)
}

// Find retrieves tags matching a query.
func (r *TagRepository) Find(ctx context.Context, query database.Query) ([]git.Tag, error) {
	var entities []TagEntity
	result := query.Apply(r.db.Session(ctx).Model(&TagEntity{})).Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find tags: %w", result.Error)
	}

	tags := make([]git.Tag, len(entities))
	for i, e := range entities {
		tags[i] = r.mapper.ToDomain(e)
	}
	return tags, nil
}

// Save creates or updates a tag.
func (r *TagRepository) Save(ctx context.Context, tag git.Tag) (git.Tag, error) {
	entity := r.mapper.ToDatabase(tag)

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_commit_sha", "message", "tagger_name", "tagger_email", "tagged_at", "updated_at"}),
	}).Create(&entity)

	if result.Error != nil {
		return git.Tag{}, fmt.Errorf("save tag: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// SaveAll creates or updates multiple tags.
func (r *TagRepository) SaveAll(ctx context.Context, tags []git.Tag) ([]git.Tag, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	entities := make([]TagEntity, len(tags))
	for i, t := range tags {
		entities[i] = r.mapper.ToDatabase(t)
	}

	result := r.db.Session(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "repo_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_commit_sha", "message", "tagger_name", "tagger_email", "tagged_at", "updated_at"}),
	}).Create(&entities)

	if result.Error != nil {
		return nil, fmt.Errorf("save tags: %w", result.Error)
	}

	saved := make([]git.Tag, len(entities))
	for i, e := range entities {
		saved[i] = r.mapper.ToDomain(e)
	}
	return saved, nil
}

// Delete removes a tag.
func (r *TagRepository) Delete(ctx context.Context, tag git.Tag) error {
	result := r.db.Session(ctx).Where("repo_id = ? AND name = ?", tag.RepoID(), tag.Name()).Delete(&TagEntity{})
	if result.Error != nil {
		return fmt.Errorf("delete tag: %w", result.Error)
	}
	return nil
}

// GetByName retrieves a tag by repository ID and name.
func (r *TagRepository) GetByName(ctx context.Context, repoID int64, name string) (git.Tag, error) {
	var entity TagEntity
	result := r.db.Session(ctx).Where("repo_id = ? AND name = ?", repoID, name).First(&entity)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return git.Tag{}, fmt.Errorf("%w: tag %s", database.ErrNotFound, name)
		}
		return git.Tag{}, fmt.Errorf("get tag: %w", result.Error)
	}
	return r.mapper.ToDomain(entity), nil
}

// FindByRepoID retrieves all tags for a repository.
func (r *TagRepository) FindByRepoID(ctx context.Context, repoID int64) ([]git.Tag, error) {
	var entities []TagEntity
	result := r.db.Session(ctx).Where("repo_id = ?", repoID).Order("name ASC").Find(&entities)
	if result.Error != nil {
		return nil, fmt.Errorf("find tags by repo: %w", result.Error)
	}

	tags := make([]git.Tag, len(entities))
	for i, e := range entities {
		tags[i] = r.mapper.ToDomain(e)
	}
	return tags, nil
}
