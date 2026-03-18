package git

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/service"
)

// isFileURI reports whether uri uses the file:// scheme.
func isFileURI(uri string) bool {
	return strings.HasPrefix(uri, "file://")
}

// localPathFromFileURI extracts the filesystem path from a file:// URI.
// file:///home/user/project → /home/user/project
func localPathFromFileURI(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}

// isGitRepo reports whether path is recognised as a git repository by git itself.
// It handles regular repos (.git/), bare repos, and worktrees.
func isGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run() == nil
}

// RepositoryCloner handles repository cloning and updating operations.
// Implements domain/service.Cloner interface.
type RepositoryCloner struct {
	adapter  Adapter
	cloneDir string
	logger   zerolog.Logger
}

// NewRepositoryCloner creates a new RepositoryCloner with the specified adapter and clone directory.
func NewRepositoryCloner(adapter Adapter, cloneDir string, logger zerolog.Logger) *RepositoryCloner {
	return &RepositoryCloner{
		adapter:  adapter,
		cloneDir: cloneDir,
		logger:   logger,
	}
}

// ClonePathFromURI returns the local clone path for a given repository URI.
// For file:// URIs the local path is returned directly; for all other URIs
// a sanitized subdirectory of the configured clone directory is returned.
func (c *RepositoryCloner) ClonePathFromURI(uri string) string {
	if isFileURI(uri) {
		return localPathFromFileURI(uri)
	}
	// Sanitize the URI to create a safe directory name
	sanitized := sanitizeURIForPath(uri)
	return filepath.Join(c.cloneDir, sanitized)
}

// Clone clones a repository and returns the local path.
// For file:// URIs the directory already exists locally; no git clone is performed.
func (c *RepositoryCloner) Clone(ctx context.Context, remoteURI string) (string, error) {
	if isFileURI(remoteURI) {
		localPath := localPathFromFileURI(remoteURI)
		c.logger.Info().Str("uri", remoteURI).Str("path", localPath).Msg("file:// repository; skipping clone")
		return localPath, nil
	}

	clonePath := c.ClonePathFromURI(remoteURI)

	c.logger.Info().Str("uri", remoteURI).Str("path", clonePath).Msg("cloning repository")

	err := c.adapter.CloneRepository(ctx, remoteURI, clonePath)
	if err != nil {
		// Clean up on failure
		_ = os.RemoveAll(clonePath)
		return "", fmt.Errorf("clone repository: %w", err)
	}

	return clonePath, nil
}

// CloneToPath clones a repository to a specific path.
func (c *RepositoryCloner) CloneToPath(ctx context.Context, remoteURI string, clonePath string) error {
	c.logger.Info().Str("uri", remoteURI).Str("path", clonePath).Msg("cloning repository to path")

	err := c.adapter.CloneRepository(ctx, remoteURI, clonePath)
	if err != nil {
		// Clean up on failure
		_ = os.RemoveAll(clonePath)
		return fmt.Errorf("clone repository: %w", err)
	}

	return nil
}

// Update updates a repository based on its tracking configuration.
// Returns the actual clone path used, which may differ from the stored
// path if the repository was relocated (e.g. after migration).
func (c *RepositoryCloner) Update(ctx context.Context, repo repository.Repository) (string, error) {
	if !repo.HasWorkingCopy() {
		return "", repository.ErrNotCloned
	}

	clonePath := repo.WorkingCopy().Path()

	// For file:// repositories the directory is owned by the user; never
	// attempt to re-clone it even if the path is temporarily inaccessible.
	if isFileURI(repo.RemoteURL()) {
		if !isGitRepo(clonePath) {
			// Plain local directory — no git operations to run.
			c.logger.Debug().Int64("repo_id", repo.ID()).Str("path", clonePath).Msg("file:// repository is not a git repo; skipping fetch/pull")
			return clonePath, nil
		}
		// Fall through to the normal branch/tag update logic below.
	} else {
		// Check if the path exists and is accessible (git repos only).
		if _, err := os.Stat(clonePath); err != nil {
			// The stored path is stale (e.g. from a previous container).
			// Clone to the correct location for the current environment.
			clonePath = c.ClonePathFromURI(repo.RemoteURL())

			c.logger.Info().Int64("repo_id", repo.ID()).Str("old_path", repo.WorkingCopy().Path()).Str("new_path", clonePath).Msg("relocating repository clone")

			if err := c.adapter.CloneRepository(ctx, repo.RemoteURL(), clonePath); err != nil {
				_ = os.RemoveAll(clonePath)
				return "", fmt.Errorf("clone repository: %w", err)
			}

			return clonePath, nil
		}
	}

	if !repo.HasTrackingConfig() {
		c.logger.Debug().Int64("repo_id", repo.ID()).Msg("repository has no tracking config")
		return clonePath, nil
	}

	tc := repo.TrackingConfig()

	if tc.IsBranch() {
		return clonePath, c.updateBranch(ctx, clonePath, tc.Branch())
	}

	if tc.IsTag() {
		return clonePath, c.updateTag(ctx, clonePath)
	}

	return "", fmt.Errorf("invalid tracking type for repository %d", repo.ID())
}

func (c *RepositoryCloner) updateBranch(ctx context.Context, clonePath string, branchName string) error {
	// Fetch latest changes
	if err := c.adapter.FetchRepository(ctx, clonePath); err != nil {
		return fmt.Errorf("fetch repository: %w", err)
	}

	// Try to checkout the branch
	if err := c.adapter.CheckoutBranch(ctx, clonePath, branchName); err != nil {
		// Branch might not exist - detect default branch
		c.logger.Warn().Str("branch", branchName).Str("error", err.Error()).Msg("checkout failed, detecting default branch")

		defaultBranch, err := c.adapter.DefaultBranch(ctx, clonePath)
		if err != nil {
			return fmt.Errorf("detect default branch: %w", err)
		}

		if err := c.adapter.CheckoutBranch(ctx, clonePath, defaultBranch); err != nil {
			return fmt.Errorf("checkout default branch: %w", err)
		}
	}

	// Pull latest changes
	if err := c.adapter.PullRepository(ctx, clonePath); err != nil {
		c.logger.Debug().Str("error", err.Error()).Msg("pull failed (possibly detached HEAD)")
	}

	return nil
}

func (c *RepositoryCloner) updateTag(ctx context.Context, clonePath string) error {
	// Fetch all tags
	if err := c.adapter.FetchRepository(ctx, clonePath); err != nil {
		return fmt.Errorf("fetch repository: %w", err)
	}

	// Get all tags
	tags, err := c.adapter.AllTags(ctx, clonePath)
	if err != nil {
		return fmt.Errorf("get tags: %w", err)
	}

	if len(tags) == 0 {
		return fmt.Errorf("no tags found in repository")
	}

	// Find the latest tag (by commit SHA - this is a simplification)
	// In a production system, you'd sort by tag date or semantic version
	latestTag := tags[len(tags)-1]

	// Checkout the tag's commit
	if err := c.adapter.CheckoutCommit(ctx, clonePath, latestTag.TargetCommitSHA); err != nil {
		return fmt.Errorf("checkout tag commit: %w", err)
	}

	return nil
}

// Ensure clones the repository if it doesn't exist, otherwise pulls latest changes.
func (c *RepositoryCloner) Ensure(ctx context.Context, remoteURI string) (string, error) {
	clonePath := c.ClonePathFromURI(remoteURI)

	c.logger.Info().Str("uri", remoteURI).Str("path", clonePath).Msg("ensuring repository exists")

	err := c.adapter.EnsureRepository(ctx, remoteURI, clonePath)
	if err != nil {
		return "", fmt.Errorf("ensure repository: %w", err)
	}

	return clonePath, nil
}

func sanitizeURIForPath(uri string) string {
	result := make([]byte, 0, len(uri))

	for _, b := range []byte(uri) {
		switch b {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '@':
			result = append(result, '_')
		default:
			result = append(result, b)
		}
	}

	// Remove common prefixes
	s := string(result)
	for _, prefix := range []string{"https___", "http___", "git___", "file____", "file___"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			s = s[len(prefix):]
			break
		}
	}

	// Keep the directory name short enough that the full clone path
	// (cloneDir + sanitized + .git/objects/...) stays reasonable.
	const maxLen = 80
	if len(s) > maxLen {
		hash := sha256.Sum256([]byte(uri))
		suffix := hex.EncodeToString(hash[:8])
		s = s[:maxLen-len(suffix)-1] + "-" + suffix
	}

	return s
}

// Ensure RepositoryCloner implements Cloner.
var _ service.Cloner = (*RepositoryCloner)(nil)
