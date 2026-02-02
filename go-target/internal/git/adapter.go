package git

import (
	"context"
	"time"
)

// Adapter defines the interface for Git repository operations.
// Implementations wrap specific git libraries (e.g., gitea modules).
type Adapter interface {
	// CloneRepository clones a repository to local path.
	CloneRepository(ctx context.Context, remoteURI string, localPath string) error

	// CheckoutCommit checks out a specific commit.
	CheckoutCommit(ctx context.Context, localPath string, commitSHA string) error

	// CheckoutBranch checks out a specific branch.
	CheckoutBranch(ctx context.Context, localPath string, branchName string) error

	// FetchRepository fetches latest changes for existing repository.
	FetchRepository(ctx context.Context, localPath string) error

	// PullRepository pulls latest changes for existing repository.
	PullRepository(ctx context.Context, localPath string) error

	// AllBranches returns all branches in repository.
	AllBranches(ctx context.Context, localPath string) ([]BranchInfo, error)

	// BranchCommits returns commit history for a specific branch.
	BranchCommits(ctx context.Context, localPath string, branchName string) ([]CommitInfo, error)

	// AllCommitsBulk returns all commits from all branches in bulk.
	AllCommitsBulk(ctx context.Context, localPath string, since *time.Time) (map[string]CommitInfo, error)

	// BranchCommitSHAs returns only commit SHAs for a branch.
	BranchCommitSHAs(ctx context.Context, localPath string, branchName string) ([]string, error)

	// AllBranchHeadSHAs returns head commit SHAs for all branches in one operation.
	AllBranchHeadSHAs(ctx context.Context, localPath string, branchNames []string) (map[string]string, error)

	// CommitFiles returns all files in a specific commit from the git tree.
	CommitFiles(ctx context.Context, localPath string, commitSHA string) ([]FileInfo, error)

	// RepositoryExists checks if repository exists at local path.
	RepositoryExists(ctx context.Context, localPath string) (bool, error)

	// CommitDetails returns detailed information about a specific commit.
	CommitDetails(ctx context.Context, localPath string, commitSHA string) (CommitInfo, error)

	// EnsureRepository clones if doesn't exist, otherwise fetches latest changes.
	EnsureRepository(ctx context.Context, remoteURI string, localPath string) error

	// FileContent returns file content at specific commit.
	FileContent(ctx context.Context, localPath string, commitSHA string, filePath string) ([]byte, error)

	// DefaultBranch returns the default branch name with fallback strategies.
	DefaultBranch(ctx context.Context, localPath string) (string, error)

	// LatestCommitSHA returns the latest commit SHA for a branch.
	LatestCommitSHA(ctx context.Context, localPath string, branchName string) (string, error)

	// AllTags returns all tags in repository.
	AllTags(ctx context.Context, localPath string) ([]TagInfo, error)

	// CommitDiff returns the diff for a specific commit.
	CommitDiff(ctx context.Context, localPath string, commitSHA string) (string, error)
}

// CommitInfo holds commit metadata returned from the adapter.
type CommitInfo struct {
	SHA           string
	Message       string
	AuthorName    string
	AuthorEmail   string
	CommitterName string
	CommitterEmail string
	AuthoredAt    time.Time
	CommittedAt   time.Time
	ParentSHA     string
}

// BranchInfo holds branch metadata returned from the adapter.
type BranchInfo struct {
	Name      string
	HeadSHA   string
	IsDefault bool
}

// FileInfo holds file metadata returned from the adapter.
type FileInfo struct {
	Path     string
	BlobSHA  string
	Size     int64
	MimeType string
}

// TagInfo holds tag metadata returned from the adapter.
type TagInfo struct {
	Name            string
	TargetCommitSHA string
	Message         string
	TaggerName      string
	TaggerEmail     string
	TaggedAt        time.Time
}
