package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/domain/repository"
	"gorm.io/gorm"
)

// RepositoryQuery provides read-only queries for repositories.
type RepositoryQuery struct {
	repoStore   repository.RepositoryStore
	commitStore repository.CommitStore
	branchStore repository.BranchStore
	tagStore    repository.TagStore
	fileStore   repository.FileStore
}

// NewRepositoryQuery creates a new RepositoryQuery.
func NewRepositoryQuery(
	repoStore repository.RepositoryStore,
	commitStore repository.CommitStore,
	branchStore repository.BranchStore,
	tagStore repository.TagStore,
) *RepositoryQuery {
	return &RepositoryQuery{
		repoStore:   repoStore,
		commitStore: commitStore,
		branchStore: branchStore,
		tagStore:    tagStore,
	}
}

// WithFileStore sets the file store (optional).
func (s *RepositoryQuery) WithFileStore(store repository.FileStore) *RepositoryQuery {
	s.fileStore = store
	return s
}

// RepositorySummary provides a summary view of a repository.
type RepositorySummary struct {
	source        Source
	branchCount   int
	tagCount      int
	commitCount   int
	defaultBranch string
}

// NewRepositorySummary creates a new RepositorySummary.
func NewRepositorySummary(
	source Source,
	branchCount, tagCount, commitCount int,
	defaultBranch string,
) RepositorySummary {
	return RepositorySummary{
		source:        source,
		branchCount:   branchCount,
		tagCount:      tagCount,
		commitCount:   commitCount,
		defaultBranch: defaultBranch,
	}
}

// Source returns the repository source.
func (s RepositorySummary) Source() Source { return s.source }

// BranchCount returns the number of branches.
func (s RepositorySummary) BranchCount() int { return s.branchCount }

// TagCount returns the number of tags.
func (s RepositorySummary) TagCount() int { return s.tagCount }

// CommitCount returns the number of indexed commits.
func (s RepositorySummary) CommitCount() int { return s.commitCount }

// DefaultBranch returns the default branch name.
func (s RepositorySummary) DefaultBranch() string { return s.defaultBranch }

// ByID returns a repository by ID.
func (s *RepositoryQuery) ByID(ctx context.Context, id int64) (Source, error) {
	repo, err := s.repoStore.Get(ctx, id)
	if err != nil {
		return Source{}, fmt.Errorf("get repository: %w", err)
	}
	return NewSource(repo), nil
}

// ByRemoteURL returns a repository by its remote URL.
func (s *RepositoryQuery) ByRemoteURL(ctx context.Context, url string) (Source, bool, error) {
	repo, err := s.repoStore.GetByRemoteURL(ctx, url)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Source{}, false, nil
		}
		return Source{}, false, fmt.Errorf("get repository by URL: %w", err)
	}
	return NewSource(repo), true, nil
}

// All returns all repositories.
func (s *RepositoryQuery) All(ctx context.Context) ([]Source, error) {
	repos, err := s.repoStore.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("find all repositories: %w", err)
	}

	sources := make([]Source, 0, len(repos))
	for _, repo := range repos {
		sources = append(sources, NewSource(repo))
	}

	return sources, nil
}

// SummaryByID returns a detailed summary for a repository.
func (s *RepositoryQuery) SummaryByID(ctx context.Context, id int64) (RepositorySummary, error) {
	repo, err := s.repoStore.Get(ctx, id)
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("get repository: %w", err)
	}

	branches, err := s.branchStore.FindByRepoID(ctx, id)
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("find branches: %w", err)
	}

	tags, err := s.tagStore.FindByRepoID(ctx, id)
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("find tags: %w", err)
	}

	commits, err := s.commitStore.FindByRepoID(ctx, id)
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("find commits: %w", err)
	}

	var defaultBranch string
	for _, branch := range branches {
		if branch.IsDefault() {
			defaultBranch = branch.Name()
			break
		}
	}

	return NewRepositorySummary(
		NewSource(repo),
		len(branches),
		len(tags),
		len(commits),
		defaultBranch,
	), nil
}

// Exists checks if a repository exists by remote URL.
func (s *RepositoryQuery) Exists(ctx context.Context, url string) (bool, error) {
	return s.repoStore.ExistsByRemoteURL(ctx, url)
}

// CommitsForRepository returns all indexed commits for a repository.
func (s *RepositoryQuery) CommitsForRepository(ctx context.Context, repoID int64) ([]repository.Commit, error) {
	commits, err := s.commitStore.FindByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("find commits: %w", err)
	}
	return commits, nil
}

// BranchesForRepository returns all branches for a repository.
func (s *RepositoryQuery) BranchesForRepository(ctx context.Context, repoID int64) ([]repository.Branch, error) {
	branches, err := s.branchStore.FindByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("find branches: %w", err)
	}
	return branches, nil
}

// TagsForRepository returns all tags for a repository.
func (s *RepositoryQuery) TagsForRepository(ctx context.Context, repoID int64) ([]repository.Tag, error) {
	tags, err := s.tagStore.FindByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("find tags: %w", err)
	}
	return tags, nil
}

// TagByID returns a specific tag by ID within a repository.
func (s *RepositoryQuery) TagByID(ctx context.Context, repoID, tagID int64) (repository.Tag, error) {
	tag, err := s.tagStore.Get(ctx, tagID)
	if err != nil {
		return repository.Tag{}, fmt.Errorf("get tag: %w", err)
	}
	// Verify tag belongs to the repository
	if tag.RepoID() != repoID {
		return repository.Tag{}, fmt.Errorf("tag %d not found in repository %d", tagID, repoID)
	}
	return tag, nil
}

// CommitBySHA returns a specific commit by SHA within a repository.
func (s *RepositoryQuery) CommitBySHA(ctx context.Context, repoID int64, sha string) (repository.Commit, error) {
	commit, err := s.commitStore.GetByRepoAndSHA(ctx, repoID, sha)
	if err != nil {
		return repository.Commit{}, fmt.Errorf("get commit: %w", err)
	}
	return commit, nil
}

// FilesForCommit returns all files for a commit.
func (s *RepositoryQuery) FilesForCommit(ctx context.Context, commitSHA string) ([]repository.File, error) {
	if s.fileStore == nil {
		return []repository.File{}, nil
	}
	files, err := s.fileStore.FindByCommitSHA(ctx, commitSHA)
	if err != nil {
		return nil, fmt.Errorf("find files: %w", err)
	}
	return files, nil
}

// FileByBlobSHA returns a file by commit SHA and blob SHA.
func (s *RepositoryQuery) FileByBlobSHA(ctx context.Context, commitSHA, blobSHA string) (repository.File, error) {
	if s.fileStore == nil {
		return repository.File{}, fmt.Errorf("file store not configured")
	}
	file, err := s.fileStore.GetByCommitAndBlobSHA(ctx, commitSHA, blobSHA)
	if err != nil {
		return repository.File{}, fmt.Errorf("get file: %w", err)
	}
	return file, nil
}
