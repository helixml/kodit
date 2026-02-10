package service

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
)

// CommitListParams configures commit listing.
type CommitListParams struct {
	RepositoryID int64
}

// CommitGetParams configures retrieving a single commit.
type CommitGetParams struct {
	RepositoryID int64
	SHA          string
}

// Commit provides commit query operations.
type Commit struct {
	commitStore repository.CommitStore
}

// NewCommit creates a new Commit service.
func NewCommit(commitStore repository.CommitStore) *Commit {
	return &Commit{
		commitStore: commitStore,
	}
}

// List returns commits for a repository.
func (s *Commit) List(ctx context.Context, params *CommitListParams) ([]repository.Commit, error) {
	commits, err := s.commitStore.FindByRepoID(ctx, params.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("find commits: %w", err)
	}
	return commits, nil
}

// Get retrieves a specific commit by repository ID and SHA.
func (s *Commit) Get(ctx context.Context, params *CommitGetParams) (repository.Commit, error) {
	commit, err := s.commitStore.GetByRepoAndSHA(ctx, params.RepositoryID, params.SHA)
	if err != nil {
		return repository.Commit{}, fmt.Errorf("get commit: %w", err)
	}
	return commit, nil
}
