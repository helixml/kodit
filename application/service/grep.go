package service

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/git"
)

// GrepResult holds grouped grep matches for a single file.
type GrepResult struct {
	Path      string
	Language  string
	Matches   []git.GrepMatch
	CommitSHA string
	RepoID    int64
}

// Grep searches repository file contents using git grep.
type Grep struct {
	repositories repository.RepositoryStore
	commits      repository.CommitStore
	git          git.Adapter
}

// NewGrep creates a new Grep service.
func NewGrep(
	repositories repository.RepositoryStore,
	commits repository.CommitStore,
	gitAdapter git.Adapter,
) *Grep {
	return &Grep{
		repositories: repositories,
		commits:      commits,
		git:          gitAdapter,
	}
}

// Search runs git grep against a repository and returns results grouped by file.
func (g *Grep) Search(ctx context.Context, repoID int64, pattern string, pathspec string, maxFiles int) ([]GrepResult, error) {
	repo, err := g.repositories.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return nil, fmt.Errorf("find repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return nil, fmt.Errorf("repository %d has no working copy", repoID)
	}

	commits, err := g.commits.Find(ctx,
		repository.WithRepoID(repoID),
		repository.WithOrderDesc("date"),
		repository.WithLimit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("find latest commit: %w", err)
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("no commits found for repository %d", repoID)
	}
	commitSHA := commits[0].SHA()

	matches, err := g.git.Grep(ctx, repo.WorkingCopy().Path(), commitSHA, pattern, pathspec, 1000)
	if err != nil {
		return nil, fmt.Errorf("git grep: %w", err)
	}

	return groupByFile(matches, commitSHA, repoID, maxFiles), nil
}

// groupByFile groups flat grep matches by file path, preserving first-seen order
// and capping at maxFiles.
func groupByFile(matches []git.GrepMatch, commitSHA string, repoID int64, maxFiles int) []GrepResult {
	if len(matches) == 0 {
		return nil
	}

	order := make([]string, 0)
	groups := make(map[string][]git.GrepMatch)

	for _, m := range matches {
		if _, exists := groups[m.Path]; !exists {
			if len(order) >= maxFiles {
				continue
			}
			order = append(order, m.Path)
			groups[m.Path] = nil
		}
		groups[m.Path] = append(groups[m.Path], m)
	}

	results := make([]GrepResult, 0, len(order))
	for _, path := range order {
		results = append(results, GrepResult{
			Path:      path,
			Language:  filepath.Ext(path),
			Matches:   groups[path],
			CommitSHA: commitSHA,
			RepoID:    repoID,
		})
	}

	return results
}
