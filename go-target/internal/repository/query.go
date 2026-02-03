package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/helixml/kodit/internal/git"
	"gorm.io/gorm"
)

// QueryService provides read-only queries for repositories.
type QueryService struct {
	repoRepo   git.RepoRepository
	commitRepo git.CommitRepository
	branchRepo git.BranchRepository
	tagRepo    git.TagRepository
	fileRepo   git.FileRepository
}

// NewQueryService creates a new QueryService.
func NewQueryService(
	repoRepo git.RepoRepository,
	commitRepo git.CommitRepository,
	branchRepo git.BranchRepository,
	tagRepo git.TagRepository,
) *QueryService {
	return &QueryService{
		repoRepo:   repoRepo,
		commitRepo: commitRepo,
		branchRepo: branchRepo,
		tagRepo:    tagRepo,
	}
}

// WithFileRepository sets the file repository (optional).
func (s *QueryService) WithFileRepository(repo git.FileRepository) *QueryService {
	s.fileRepo = repo
	return s
}

// RepositorySummary provides a summary view of a repository.
type RepositorySummary struct {
	source       Source
	branchCount  int
	tagCount     int
	commitCount  int
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
func (s *QueryService) ByID(ctx context.Context, id int64) (Source, error) {
	repo, err := s.repoRepo.Get(ctx, id)
	if err != nil {
		return Source{}, fmt.Errorf("get repository: %w", err)
	}
	return NewSource(repo), nil
}

// ByRemoteURL returns a repository by its remote URL.
func (s *QueryService) ByRemoteURL(ctx context.Context, url string) (Source, bool, error) {
	repo, err := s.repoRepo.GetByRemoteURL(ctx, url)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Source{}, false, nil
		}
		return Source{}, false, fmt.Errorf("get repository by URL: %w", err)
	}
	return NewSource(repo), true, nil
}

// All returns all repositories.
func (s *QueryService) All(ctx context.Context) ([]Source, error) {
	repos, err := s.repoRepo.FindAll(ctx)
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
func (s *QueryService) SummaryByID(ctx context.Context, id int64) (RepositorySummary, error) {
	repo, err := s.repoRepo.Get(ctx, id)
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("get repository: %w", err)
	}

	branches, err := s.branchRepo.FindByRepoID(ctx, id)
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("find branches: %w", err)
	}

	tags, err := s.tagRepo.FindByRepoID(ctx, id)
	if err != nil {
		return RepositorySummary{}, fmt.Errorf("find tags: %w", err)
	}

	commits, err := s.commitRepo.FindByRepoID(ctx, id)
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
func (s *QueryService) Exists(ctx context.Context, url string) (bool, error) {
	return s.repoRepo.ExistsByRemoteURL(ctx, url)
}

// CommitsForRepository returns all indexed commits for a repository.
func (s *QueryService) CommitsForRepository(ctx context.Context, repoID int64) ([]git.Commit, error) {
	commits, err := s.commitRepo.FindByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("find commits: %w", err)
	}
	return commits, nil
}

// BranchesForRepository returns all branches for a repository.
func (s *QueryService) BranchesForRepository(ctx context.Context, repoID int64) ([]git.Branch, error) {
	branches, err := s.branchRepo.FindByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("find branches: %w", err)
	}
	return branches, nil
}

// TagsForRepository returns all tags for a repository.
func (s *QueryService) TagsForRepository(ctx context.Context, repoID int64) ([]git.Tag, error) {
	tags, err := s.tagRepo.FindByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("find tags: %w", err)
	}
	return tags, nil
}

// TagByID returns a specific tag by ID within a repository.
func (s *QueryService) TagByID(ctx context.Context, repoID, tagID int64) (git.Tag, error) {
	tag, err := s.tagRepo.Get(ctx, tagID)
	if err != nil {
		return git.Tag{}, fmt.Errorf("get tag: %w", err)
	}
	// Verify tag belongs to the repository
	if tag.RepoID() != repoID {
		return git.Tag{}, fmt.Errorf("tag %d not found in repository %d", tagID, repoID)
	}
	return tag, nil
}

// CommitBySHA returns a specific commit by SHA within a repository.
func (s *QueryService) CommitBySHA(ctx context.Context, repoID int64, sha string) (git.Commit, error) {
	commit, err := s.commitRepo.GetByRepoAndSHA(ctx, repoID, sha)
	if err != nil {
		return git.Commit{}, fmt.Errorf("get commit: %w", err)
	}
	return commit, nil
}

// FilesForCommit returns all files for a commit.
func (s *QueryService) FilesForCommit(ctx context.Context, commitSHA string) ([]git.File, error) {
	if s.fileRepo == nil {
		return []git.File{}, nil
	}
	files, err := s.fileRepo.FindByCommitSHA(ctx, commitSHA)
	if err != nil {
		return nil, fmt.Errorf("find files: %w", err)
	}
	return files, nil
}

// FileByBlobSHA returns a file by commit SHA and blob SHA.
func (s *QueryService) FileByBlobSHA(ctx context.Context, commitSHA, blobSHA string) (git.File, error) {
	if s.fileRepo == nil {
		return git.File{}, fmt.Errorf("file repository not configured")
	}
	file, err := s.fileRepo.GetByCommitAndBlobSHA(ctx, commitSHA, blobSHA)
	if err != nil {
		return git.File{}, fmt.Errorf("get file: %w", err)
	}
	return file, nil
}
