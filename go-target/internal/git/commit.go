package git

import "time"

// Commit represents a Git commit.
type Commit struct {
	id              int64
	sha             string
	repoID          int64
	message         string
	author          Author
	committer       Author
	authoredAt      time.Time
	committedAt     time.Time
	createdAt       time.Time
	parentCommitSHA string
}

// NewCommit creates a new Commit.
func NewCommit(sha string, repoID int64, message string, author, committer Author, authoredAt, committedAt time.Time) Commit {
	return Commit{
		sha:         sha,
		repoID:      repoID,
		message:     message,
		author:      author,
		committer:   committer,
		authoredAt:  authoredAt,
		committedAt: committedAt,
		createdAt:   time.Now(),
	}
}

// NewCommitWithParent creates a new Commit with a parent SHA.
func NewCommitWithParent(sha string, repoID int64, message string, author, committer Author, authoredAt, committedAt time.Time, parentSHA string) Commit {
	c := NewCommit(sha, repoID, message, author, committer, authoredAt, committedAt)
	c.parentCommitSHA = parentSHA
	return c
}

// ReconstructCommit reconstructs a Commit from persistence.
func ReconstructCommit(
	id int64,
	sha string,
	repoID int64,
	message string,
	author, committer Author,
	authoredAt, committedAt, createdAt time.Time,
	parentCommitSHA string,
) Commit {
	return Commit{
		id:              id,
		sha:             sha,
		repoID:          repoID,
		message:         message,
		author:          author,
		committer:       committer,
		authoredAt:      authoredAt,
		committedAt:     committedAt,
		createdAt:       createdAt,
		parentCommitSHA: parentCommitSHA,
	}
}

// ID returns the commit ID.
func (c Commit) ID() int64 { return c.id }

// SHA returns the commit SHA.
func (c Commit) SHA() string { return c.sha }

// RepoID returns the repository ID.
func (c Commit) RepoID() int64 { return c.repoID }

// Message returns the commit message.
func (c Commit) Message() string { return c.message }

// Author returns the author.
func (c Commit) Author() Author { return c.author }

// Committer returns the committer.
func (c Commit) Committer() Author { return c.committer }

// AuthoredAt returns the authored timestamp.
func (c Commit) AuthoredAt() time.Time { return c.authoredAt }

// CommittedAt returns the committed timestamp.
func (c Commit) CommittedAt() time.Time { return c.committedAt }

// CreatedAt returns the creation timestamp.
func (c Commit) CreatedAt() time.Time { return c.createdAt }

// ParentCommitSHA returns the parent commit SHA.
func (c Commit) ParentCommitSHA() string { return c.parentCommitSHA }

// ShortSHA returns the first 7 characters of the SHA.
func (c Commit) ShortSHA() string {
	if len(c.sha) >= 7 {
		return c.sha[:7]
	}
	return c.sha
}

// ShortMessage returns the first line of the commit message.
func (c Commit) ShortMessage() string {
	for i, r := range c.message {
		if r == '\n' {
			return c.message[:i]
		}
	}
	return c.message
}

// WithID returns a new Commit with the specified ID.
func (c Commit) WithID(id int64) Commit {
	c.id = id
	return c
}
