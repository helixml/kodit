package tracking

import (
	"context"
	"log/slog"

	"github.com/helixml/kodit/internal/git"
)

// Resolver resolves trackables to ordered lists of commit SHAs.
// This is a domain service that orchestrates multiple repositories
// (branches, tags, commits) to resolve references.
type Resolver struct {
	commitRepo git.CommitRepository
	branchRepo git.BranchRepository
	tagRepo    git.TagRepository
	logger     *slog.Logger
}

// NewResolver creates a new trackable resolver.
func NewResolver(
	commitRepo git.CommitRepository,
	branchRepo git.BranchRepository,
	tagRepo git.TagRepository,
	logger *slog.Logger,
) *Resolver {
	return &Resolver{
		commitRepo: commitRepo,
		branchRepo: branchRepo,
		tagRepo:    tagRepo,
		logger:     logger,
	}
}

// Commits resolves a trackable to an ordered list of commit SHAs.
// Returns commits from newest to oldest based on git history.
func (r *Resolver) Commits(ctx context.Context, trackable Trackable, limit int) ([]string, error) {
	switch trackable.Type() {
	case ReferenceTypeBranch:
		return r.resolveBranch(ctx, trackable, limit)
	case ReferenceTypeTag:
		return r.resolveTag(ctx, trackable, limit)
	case ReferenceTypeCommitSHA:
		return []string{trackable.Identifier()}, nil
	default:
		return []string{trackable.Identifier()}, nil
	}
}

func (r *Resolver) resolveBranch(ctx context.Context, trackable Trackable, limit int) ([]string, error) {
	branch, err := r.branchRepo.GetByName(ctx, trackable.RepoID(), trackable.Identifier())
	if err != nil {
		return nil, err
	}
	return r.walkCommitHistory(ctx, trackable.RepoID(), branch.HeadCommitSHA(), limit)
}

func (r *Resolver) resolveTag(ctx context.Context, trackable Trackable, limit int) ([]string, error) {
	tag, err := r.tagRepo.GetByName(ctx, trackable.RepoID(), trackable.Identifier())
	if err != nil {
		return nil, err
	}
	return r.walkCommitHistory(ctx, trackable.RepoID(), tag.CommitSHA(), limit)
}

func (r *Resolver) walkCommitHistory(_ context.Context, _ int64, startSHA string, limit int) ([]string, error) {
	// Note: Full parent traversal requires commit.ParentCommitSHA which isn't
	// currently implemented in the Go Commit type. For now, return only the
	// starting commit. This can be enhanced when parent_commit_sha is added.
	if limit <= 0 || startSHA == "" {
		return []string{}, nil
	}
	return []string{startSHA}, nil
}
