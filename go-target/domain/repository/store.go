package repository

import (
	"context"
)

// RepositoryStore defines the interface for Repository persistence.
type RepositoryStore interface {
	Get(ctx context.Context, id int64) (Repository, error)
	FindAll(ctx context.Context) ([]Repository, error)
	Save(ctx context.Context, repo Repository) (Repository, error)
	Delete(ctx context.Context, repo Repository) error
	GetByRemoteURL(ctx context.Context, url string) (Repository, error)
	ExistsByRemoteURL(ctx context.Context, url string) (bool, error)
}

// CommitStore defines the interface for Commit persistence.
type CommitStore interface {
	Get(ctx context.Context, id int64) (Commit, error)
	Save(ctx context.Context, commit Commit) (Commit, error)
	SaveAll(ctx context.Context, commits []Commit) ([]Commit, error)
	Delete(ctx context.Context, commit Commit) error
	GetByRepoAndSHA(ctx context.Context, repoID int64, sha string) (Commit, error)
	FindByRepoID(ctx context.Context, repoID int64) ([]Commit, error)
	ExistsBySHA(ctx context.Context, repoID int64, sha string) (bool, error)
}

// BranchStore defines the interface for Branch persistence.
type BranchStore interface {
	Get(ctx context.Context, id int64) (Branch, error)
	Save(ctx context.Context, branch Branch) (Branch, error)
	SaveAll(ctx context.Context, branches []Branch) ([]Branch, error)
	Delete(ctx context.Context, branch Branch) error
	GetByName(ctx context.Context, repoID int64, name string) (Branch, error)
	FindByRepoID(ctx context.Context, repoID int64) ([]Branch, error)
	GetDefaultBranch(ctx context.Context, repoID int64) (Branch, error)
}

// TagStore defines the interface for Tag persistence.
type TagStore interface {
	Get(ctx context.Context, id int64) (Tag, error)
	Save(ctx context.Context, tag Tag) (Tag, error)
	SaveAll(ctx context.Context, tags []Tag) ([]Tag, error)
	Delete(ctx context.Context, tag Tag) error
	GetByName(ctx context.Context, repoID int64, name string) (Tag, error)
	FindByRepoID(ctx context.Context, repoID int64) ([]Tag, error)
}

// FileStore defines the interface for File persistence.
type FileStore interface {
	Get(ctx context.Context, id int64) (File, error)
	Save(ctx context.Context, file File) (File, error)
	SaveAll(ctx context.Context, files []File) ([]File, error)
	Delete(ctx context.Context, file File) error
	FindByCommitSHA(ctx context.Context, sha string) ([]File, error)
	DeleteByCommitSHA(ctx context.Context, sha string) error
	GetByCommitAndBlobSHA(ctx context.Context, commitSHA, blobSHA string) (File, error)
}
