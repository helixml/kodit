package git

import "time"

// Branch represents a Git branch.
type Branch struct {
	id            int64
	repoID        int64
	name          string
	headCommitSHA string
	isDefault     bool
	createdAt     time.Time
	updatedAt     time.Time
}

// NewBranch creates a new Branch.
func NewBranch(repoID int64, name, headCommitSHA string, isDefault bool) Branch {
	now := time.Now()
	return Branch{
		repoID:        repoID,
		name:          name,
		headCommitSHA: headCommitSHA,
		isDefault:     isDefault,
		createdAt:     now,
		updatedAt:     now,
	}
}

// ReconstructBranch reconstructs a Branch from persistence.
func ReconstructBranch(
	id, repoID int64,
	name, headCommitSHA string,
	isDefault bool,
	createdAt, updatedAt time.Time,
) Branch {
	return Branch{
		id:            id,
		repoID:        repoID,
		name:          name,
		headCommitSHA: headCommitSHA,
		isDefault:     isDefault,
		createdAt:     createdAt,
		updatedAt:     updatedAt,
	}
}

// ID returns the branch ID.
func (b Branch) ID() int64 { return b.id }

// RepoID returns the repository ID.
func (b Branch) RepoID() int64 { return b.repoID }

// Name returns the branch name.
func (b Branch) Name() string { return b.name }

// HeadCommitSHA returns the HEAD commit SHA.
func (b Branch) HeadCommitSHA() string { return b.headCommitSHA }

// IsDefault returns true if this is the default branch.
func (b Branch) IsDefault() bool { return b.isDefault }

// CreatedAt returns the creation timestamp.
func (b Branch) CreatedAt() time.Time { return b.createdAt }

// UpdatedAt returns the last update timestamp.
func (b Branch) UpdatedAt() time.Time { return b.updatedAt }

// WithHeadCommitSHA returns a new Branch with an updated HEAD commit.
func (b Branch) WithHeadCommitSHA(sha string) Branch {
	b.headCommitSHA = sha
	b.updatedAt = time.Now()
	return b
}

// WithID returns a new Branch with the specified ID.
func (b Branch) WithID(id int64) Branch {
	b.id = id
	return b
}
