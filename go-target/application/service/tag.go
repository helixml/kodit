package service

import "github.com/helixml/kodit/domain/repository"

// Tag provides read access to tags, extensible with bespoke methods later.
type Tag struct {
	repository.Collection[repository.Tag]
}

// NewTag creates a new Tag service wrapping the given store.
func NewTag(store repository.TagStore) *Tag {
	return &Tag{Collection: repository.NewCollection[repository.Tag](store)}
}
