package service

import "github.com/helixml/kodit/domain/repository"

// Commit provides read access to commits, extensible with bespoke methods later.
type Commit struct {
	repository.Collection[repository.Commit]
}

// NewCommit creates a new Commit service wrapping the given store.
func NewCommit(store repository.CommitStore) *Commit {
	return &Commit{Collection: repository.NewCollection[repository.Commit](store)}
}
