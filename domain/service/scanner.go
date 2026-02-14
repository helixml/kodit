// Package service provides domain service interfaces.
package service

import (
	"context"

	"github.com/helixml/kodit/domain/repository"
)

// ScanCommitResult holds the result of scanning a single commit.
type ScanCommitResult struct {
	commit repository.Commit
	files  []repository.File
}

// NewScanCommitResult creates a new ScanCommitResult.
func NewScanCommitResult(commit repository.Commit, files []repository.File) ScanCommitResult {
	f := make([]repository.File, len(files))
	copy(f, files)
	return ScanCommitResult{
		commit: commit,
		files:  f,
	}
}

// Commit returns the scanned commit.
func (r ScanCommitResult) Commit() repository.Commit { return r.commit }

// Files returns the scanned files.
func (r ScanCommitResult) Files() []repository.File {
	result := make([]repository.File, len(r.files))
	copy(result, r.files)
	return result
}

// Scanner extracts data from Git repositories without mutation.
type Scanner interface {
	// ScanCommit scans a specific commit and returns commit with its files.
	ScanCommit(ctx context.Context, clonedPath string, commitSHA string, repoID int64) (ScanCommitResult, error)

	// ScanBranch scans all commits on a branch.
	ScanBranch(ctx context.Context, clonedPath string, branchName string, repoID int64) ([]repository.Commit, error)

	// ScanAllBranches scans metadata for all branches.
	ScanAllBranches(ctx context.Context, clonedPath string, repoID int64) ([]repository.Branch, error)

	// ScanAllTags scans metadata for all tags.
	ScanAllTags(ctx context.Context, clonedPath string, repoID int64) ([]repository.Tag, error)

	// FilesForCommitsBatch processes files for a batch of commits.
	FilesForCommitsBatch(ctx context.Context, clonedPath string, commitSHAs []string) ([]repository.File, error)
}
