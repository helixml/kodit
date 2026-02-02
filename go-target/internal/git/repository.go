package git

import (
	"context"

	"github.com/helixml/kodit/internal/database"
)

// RepoRepository defines the interface for GitRepo persistence.
type RepoRepository interface {
	Get(ctx context.Context, id int64) (Repo, error)
	Find(ctx context.Context, query database.Query) ([]Repo, error)
	FindAll(ctx context.Context) ([]Repo, error)
	Save(ctx context.Context, repo Repo) (Repo, error)
	Delete(ctx context.Context, repo Repo) error
	GetByRemoteURL(ctx context.Context, url string) (Repo, error)
	ExistsByRemoteURL(ctx context.Context, url string) (bool, error)
}

// CommitRepository defines the interface for GitCommit persistence.
type CommitRepository interface {
	Get(ctx context.Context, id int64) (Commit, error)
	Find(ctx context.Context, query database.Query) ([]Commit, error)
	Save(ctx context.Context, commit Commit) (Commit, error)
	SaveAll(ctx context.Context, commits []Commit) ([]Commit, error)
	Delete(ctx context.Context, commit Commit) error
	GetByRepoAndSHA(ctx context.Context, repoID int64, sha string) (Commit, error)
	FindByRepoID(ctx context.Context, repoID int64) ([]Commit, error)
	ExistsBySHA(ctx context.Context, repoID int64, sha string) (bool, error)
}

// BranchRepository defines the interface for GitBranch persistence.
type BranchRepository interface {
	Get(ctx context.Context, id int64) (Branch, error)
	Find(ctx context.Context, query database.Query) ([]Branch, error)
	Save(ctx context.Context, branch Branch) (Branch, error)
	SaveAll(ctx context.Context, branches []Branch) ([]Branch, error)
	Delete(ctx context.Context, branch Branch) error
	GetByName(ctx context.Context, repoID int64, name string) (Branch, error)
	FindByRepoID(ctx context.Context, repoID int64) ([]Branch, error)
	GetDefaultBranch(ctx context.Context, repoID int64) (Branch, error)
}

// TagRepository defines the interface for GitTag persistence.
type TagRepository interface {
	Get(ctx context.Context, id int64) (Tag, error)
	Find(ctx context.Context, query database.Query) ([]Tag, error)
	Save(ctx context.Context, tag Tag) (Tag, error)
	SaveAll(ctx context.Context, tags []Tag) ([]Tag, error)
	Delete(ctx context.Context, tag Tag) error
	GetByName(ctx context.Context, repoID int64, name string) (Tag, error)
	FindByRepoID(ctx context.Context, repoID int64) ([]Tag, error)
}

// FileRepository defines the interface for GitFile persistence.
type FileRepository interface {
	Get(ctx context.Context, id int64) (File, error)
	Find(ctx context.Context, query database.Query) ([]File, error)
	Save(ctx context.Context, file File) (File, error)
	SaveAll(ctx context.Context, files []File) ([]File, error)
	Delete(ctx context.Context, file File) error
	FindByCommitSHA(ctx context.Context, sha string) ([]File, error)
	DeleteByCommitSHA(ctx context.Context, sha string) error
}
