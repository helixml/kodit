package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/internal/database"
)

var commitSHAPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// BlobContent holds the resolved content and metadata for a file at a given blob reference.
type BlobContent struct {
	content   []byte
	commitSHA string
}

// NewBlobContent creates a BlobContent from raw bytes and a commit SHA.
func NewBlobContent(content []byte, commitSHA string) BlobContent {
	return BlobContent{content: content, commitSHA: commitSHA}
}

// Content returns the raw file bytes.
func (b BlobContent) Content() []byte { return b.content }

// CommitSHA returns the resolved commit SHA.
func (b BlobContent) CommitSHA() string { return b.commitSHA }

// Blob resolves blob references (commit SHA, tag, branch) and retrieves file content.
type Blob struct {
	repositories repository.RepositoryStore
	commits      repository.CommitStore
	tags         repository.TagStore
	branches     repository.BranchStore
	git          git.Adapter
}

// NewBlob creates a new Blob service.
func NewBlob(
	repositories repository.RepositoryStore,
	commits repository.CommitStore,
	tags repository.TagStore,
	branches repository.BranchStore,
	gitAdapter git.Adapter,
) *Blob {
	return &Blob{
		repositories: repositories,
		commits:      commits,
		tags:         tags,
		branches:     branches,
		git:          gitAdapter,
	}
}

// Resolve resolves a blob name to a commit SHA.
// It tries commit SHA, then tag name, then branch name.
func (b *Blob) Resolve(ctx context.Context, repoID int64, blobName string) (string, error) {
	if commitSHAPattern.MatchString(blobName) {
		exists, err := b.commits.Exists(ctx, repository.WithRepoID(repoID), repository.WithSHA(blobName))
		if err != nil {
			return "", fmt.Errorf("check commit: %w", err)
		}
		if exists {
			return blobName, nil
		}
	}

	tags, err := b.tags.Find(ctx, repository.WithRepoID(repoID), repository.WithName(blobName))
	if err != nil {
		return "", fmt.Errorf("find tag: %w", err)
	}
	if len(tags) > 0 {
		return tags[0].CommitSHA(), nil
	}

	branches, err := b.branches.Find(ctx, repository.WithRepoID(repoID), repository.WithName(blobName))
	if err != nil {
		return "", fmt.Errorf("find branch: %w", err)
	}
	if len(branches) > 0 {
		return branches[0].HeadCommitSHA(), nil
	}

	return "", fmt.Errorf("blob reference %q not found for repository %d", blobName, repoID)
}

// Content resolves the blob reference and returns the file content at the given path.
func (b *Blob) Content(ctx context.Context, repoID int64, blobName, filePath string) (BlobContent, error) {
	commitSHA, err := b.Resolve(ctx, repoID, blobName)
	if err != nil {
		return BlobContent{}, err
	}

	repo, err := b.repositories.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return BlobContent{}, fmt.Errorf("find repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return BlobContent{}, fmt.Errorf("repository %d has no working copy", repoID)
	}

	content, err := b.git.FileContent(ctx, repo.WorkingCopy().Path(), commitSHA, filePath)
	if err != nil {
		if errors.Is(err, git.ErrFileNotFound) {
			return BlobContent{}, fmt.Errorf("%s: %w", filePath, database.ErrNotFound)
		}
		return BlobContent{}, fmt.Errorf("read file content: %w", err)
	}

	return BlobContent{
		content:   content,
		commitSHA: commitSHA,
	}, nil
}
