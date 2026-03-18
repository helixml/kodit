package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/internal/database"
)

var commitSHAPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// safeRelativePath normalises filePath to a clean, repository-relative,
// forward-slash form and rejects anything that would escape the root:
// absolute paths, empty paths, and any path whose clean form starts with "..".
func safeRelativePath(filePath string) (string, error) {
	// Normalise to forward slashes then apply path.Clean so that sequences
	// like "a/../b" become "b" and "a/./b" becomes "a/b".
	cleaned := path.Clean(filepath.ToSlash(filePath))
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("invalid file path %q", filePath)
	}
	if filepath.IsAbs(filePath) {
		return "", fmt.Errorf("absolute file path not allowed: %q", filePath)
	}
	// Reject any remaining ".." component after cleaning.
	for _, part := range strings.Split(cleaned, "/") {
		if part == ".." {
			return "", fmt.Errorf("path traversal not allowed: %q", filePath)
		}
	}
	return cleaned, nil
}

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

// ListFiles walks the repository working copy on disk and returns files matching the pattern.
func (b *Blob) ListFiles(ctx context.Context, repoID int64, pattern string) ([]FileEntry, error) {
	repo, err := b.repositories.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return nil, fmt.Errorf("find repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return nil, fmt.Errorf("repository %d has no working copy", repoID)
	}

	wc := repo.WorkingCopy()
	exists, err := b.git.RepositoryExists(ctx, wc.Path())
	if err != nil {
		return nil, fmt.Errorf("check repository: %w", err)
	}
	if !exists {
		if err := b.git.CloneRepository(ctx, wc.URI(), wc.Path()); err != nil {
			return nil, fmt.Errorf("clone repository: %w", err)
		}
	}

	root := wc.Path()
	matchAll := pattern == "" || pattern == "*" || pattern == "**"

	var entries []FileEntry
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		// Normalize to forward slashes for consistent glob matching.
		rel = filepath.ToSlash(rel)

		if !matchAll && !matchGlob(pattern, rel) {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			if os.IsNotExist(infoErr) {
				return nil
			}
			return infoErr
		}

		entries = append(entries, FileEntry{
			Path: rel,
			Size: info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return entries, nil
}

// ListFilesForCommit returns files in a specific commit from the git tree, filtered by glob pattern.
func (b *Blob) ListFilesForCommit(ctx context.Context, repoID int64, commitSHA, pattern string) ([]FileEntry, error) {
	repo, err := b.repositories.FindOne(ctx, repository.WithID(repoID))
	if err != nil {
		return nil, fmt.Errorf("find repository: %w", err)
	}

	if !repo.HasWorkingCopy() {
		return nil, fmt.Errorf("repository %d has no working copy", repoID)
	}

	wc := repo.WorkingCopy()
	exists, err := b.git.RepositoryExists(ctx, wc.Path())
	if err != nil {
		return nil, fmt.Errorf("check repository: %w", err)
	}
	if !exists {
		if err := b.git.CloneRepository(ctx, wc.URI(), wc.Path()); err != nil {
			return nil, fmt.Errorf("clone repository: %w", err)
		}
	}

	files, err := b.git.CommitFiles(ctx, wc.Path(), commitSHA)
	if err != nil {
		return nil, fmt.Errorf("commit files: %w", err)
	}

	matchAll := pattern == "" || pattern == "*" || pattern == "**"
	entries := make([]FileEntry, 0, len(files))
	for _, f := range files {
		if !matchAll && !matchGlob(pattern, f.Path) {
			continue
		}
		entries = append(entries, FileEntry{
			Path:    f.Path,
			Size:    f.Size,
			BlobSHA: f.BlobSHA,
		})
	}

	return entries, nil
}

// Content resolves the blob reference and returns the file content at the given path.
func (b *Blob) Content(ctx context.Context, repoID int64, blobName, filePath string) (BlobContent, error) {
	safePath, err := safeRelativePath(filePath)
	if err != nil {
		return BlobContent{}, fmt.Errorf("%s: %w", filePath, database.ErrNotFound)
	}

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

	content, err := b.git.FileContent(ctx, repo.WorkingCopy().Path(), commitSHA, safePath)
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
