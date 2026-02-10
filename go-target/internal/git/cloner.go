package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/helixml/kodit/domain/repository"
)

// Cloner handles repository cloning and updating operations.
type Cloner struct {
	adapter  Adapter
	cloneDir string
	logger   *slog.Logger
}

// NewCloner creates a new Cloner with the specified adapter and clone directory.
func NewCloner(adapter Adapter, cloneDir string, logger *slog.Logger) Cloner {
	return Cloner{
		adapter:  adapter,
		cloneDir: cloneDir,
		logger:   logger,
	}
}

// ClonePathFromURI returns the local clone path for a given repository URI.
func (c Cloner) ClonePathFromURI(uri string) string {
	// Sanitize the URI to create a safe directory name
	sanitized := sanitizeURIForPath(uri)
	return filepath.Join(c.cloneDir, sanitized)
}

// Clone clones a repository and returns the local path.
func (c Cloner) Clone(ctx context.Context, remoteURI string) (string, error) {
	clonePath := c.ClonePathFromURI(remoteURI)

	c.logger.Info("cloning repository",
		slog.String("uri", remoteURI),
		slog.String("path", clonePath),
	)

	err := c.adapter.CloneRepository(ctx, remoteURI, clonePath)
	if err != nil {
		// Clean up on failure
		_ = os.RemoveAll(clonePath)
		return "", fmt.Errorf("clone repository: %w", err)
	}

	return clonePath, nil
}

// CloneToPath clones a repository to a specific path.
func (c Cloner) CloneToPath(ctx context.Context, remoteURI string, clonePath string) error {
	c.logger.Info("cloning repository to path",
		slog.String("uri", remoteURI),
		slog.String("path", clonePath),
	)

	err := c.adapter.CloneRepository(ctx, remoteURI, clonePath)
	if err != nil {
		// Clean up on failure
		_ = os.RemoveAll(clonePath)
		return fmt.Errorf("clone repository: %w", err)
	}

	return nil
}

// Update updates a repository based on its tracking configuration.
func (c Cloner) Update(ctx context.Context, repo repository.Repository) error {
	if !repo.HasWorkingCopy() {
		return fmt.Errorf("repository %d has never been cloned", repo.ID())
	}

	clonePath := repo.WorkingCopy().Path()

	// Check if the path exists
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		// Re-clone if directory doesn't exist
		c.logger.Info("re-cloning missing repository",
			slog.Int64("repo_id", repo.ID()),
			slog.String("path", clonePath),
		)
		return c.adapter.CloneRepository(ctx, repo.RemoteURL(), clonePath)
	}

	if !repo.HasTrackingConfig() {
		c.logger.Debug("repository has no tracking config",
			slog.Int64("repo_id", repo.ID()),
		)
		return nil
	}

	tc := repo.TrackingConfig()

	if tc.IsBranch() {
		return c.updateBranch(ctx, clonePath, tc.Branch())
	}

	if tc.IsTag() {
		return c.updateTag(ctx, clonePath)
	}

	return fmt.Errorf("invalid tracking type for repository %d", repo.ID())
}

func (c Cloner) updateBranch(ctx context.Context, clonePath string, branchName string) error {
	// Fetch latest changes
	if err := c.adapter.FetchRepository(ctx, clonePath); err != nil {
		return fmt.Errorf("fetch repository: %w", err)
	}

	// Try to checkout the branch
	if err := c.adapter.CheckoutBranch(ctx, clonePath, branchName); err != nil {
		// Branch might not exist - detect default branch
		c.logger.Warn("checkout failed, detecting default branch",
			slog.String("branch", branchName),
			slog.String("error", err.Error()),
		)

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
		c.logger.Debug("pull failed (possibly detached HEAD)",
			slog.String("error", err.Error()),
		)
	}

	return nil
}

func (c Cloner) updateTag(ctx context.Context, clonePath string) error {
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
func (c Cloner) Ensure(ctx context.Context, remoteURI string) (string, error) {
	clonePath := c.ClonePathFromURI(remoteURI)

	c.logger.Info("ensuring repository exists",
		slog.String("uri", remoteURI),
		slog.String("path", clonePath),
	)

	err := c.adapter.EnsureRepository(ctx, remoteURI, clonePath)
	if err != nil {
		return "", fmt.Errorf("ensure repository: %w", err)
	}

	return clonePath, nil
}

func sanitizeURIForPath(uri string) string {
	// Simple sanitization - replace problematic characters
	// A more robust implementation would parse the URL properly
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
	for _, prefix := range []string{"https___", "http___", "git___"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			s = s[len(prefix):]
			break
		}
	}

	return s
}
