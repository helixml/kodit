package service

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
)

// TagListParams configures tag listing.
type TagListParams struct {
	RepositoryID int64
}

// TagGetParams configures retrieving a single tag.
type TagGetParams struct {
	RepositoryID int64
	TagID        int64
}

// Tag provides tag query operations.
type Tag struct {
	tagStore repository.TagStore
}

// NewTag creates a new Tag service.
func NewTag(tagStore repository.TagStore) *Tag {
	return &Tag{
		tagStore: tagStore,
	}
}

// List returns tags for a repository.
func (s *Tag) List(ctx context.Context, params *TagListParams) ([]repository.Tag, error) {
	tags, err := s.tagStore.FindByRepoID(ctx, params.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("find tags: %w", err)
	}
	return tags, nil
}

// Get retrieves a specific tag by repository ID and tag ID.
func (s *Tag) Get(ctx context.Context, params *TagGetParams) (repository.Tag, error) {
	tag, err := s.tagStore.Get(ctx, params.TagID)
	if err != nil {
		return repository.Tag{}, fmt.Errorf("get tag: %w", err)
	}
	if tag.RepoID() != params.RepositoryID {
		return repository.Tag{}, fmt.Errorf("tag %d not found in repository %d", params.TagID, params.RepositoryID)
	}
	return tag, nil
}
