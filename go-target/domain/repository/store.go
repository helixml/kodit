package repository

import "context"

// Store is the generic persistence interface for domain entities.
type Store[T any] interface {
	Find(ctx context.Context, options ...Option) ([]T, error)
	FindOne(ctx context.Context, options ...Option) (T, error)
	Save(ctx context.Context, entity T) (T, error)
	Delete(ctx context.Context, entity T) error
}

// RepositoryStore defines the interface for Repository persistence.
type RepositoryStore interface {
	Store[Repository]
	Exists(ctx context.Context, options ...Option) (bool, error)
}

// CommitStore defines the interface for Commit persistence.
type CommitStore interface {
	Store[Commit]
	Exists(ctx context.Context, options ...Option) (bool, error)
	SaveAll(ctx context.Context, commits []Commit) ([]Commit, error)
}

// BranchStore defines the interface for Branch persistence.
type BranchStore interface {
	Store[Branch]
	SaveAll(ctx context.Context, branches []Branch) ([]Branch, error)
}

// TagStore defines the interface for Tag persistence.
type TagStore interface {
	Store[Tag]
	SaveAll(ctx context.Context, tags []Tag) ([]Tag, error)
}

// FileStore defines the interface for File persistence.
type FileStore interface {
	Store[File]
	DeleteBy(ctx context.Context, options ...Option) error
	SaveAll(ctx context.Context, files []File) ([]File, error)
}
