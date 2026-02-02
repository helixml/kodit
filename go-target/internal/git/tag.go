package git

import "time"

// Tag represents a Git tag.
type Tag struct {
	id        int64
	repoID    int64
	name      string
	commitSHA string
	message   string
	tagger    Author
	taggedAt  time.Time
	createdAt time.Time
}

// NewTag creates a new Tag.
func NewTag(repoID int64, name, commitSHA string) Tag {
	return Tag{
		repoID:    repoID,
		name:      name,
		commitSHA: commitSHA,
		createdAt: time.Now(),
	}
}

// NewAnnotatedTag creates a new annotated Tag with message and tagger.
func NewAnnotatedTag(repoID int64, name, commitSHA, message string, tagger Author, taggedAt time.Time) Tag {
	return Tag{
		repoID:    repoID,
		name:      name,
		commitSHA: commitSHA,
		message:   message,
		tagger:    tagger,
		taggedAt:  taggedAt,
		createdAt: time.Now(),
	}
}

// ReconstructTag reconstructs a Tag from persistence.
func ReconstructTag(
	id, repoID int64,
	name, commitSHA, message string,
	tagger Author,
	taggedAt, createdAt time.Time,
) Tag {
	return Tag{
		id:        id,
		repoID:    repoID,
		name:      name,
		commitSHA: commitSHA,
		message:   message,
		tagger:    tagger,
		taggedAt:  taggedAt,
		createdAt: createdAt,
	}
}

// ID returns the tag ID.
func (t Tag) ID() int64 { return t.id }

// RepoID returns the repository ID.
func (t Tag) RepoID() int64 { return t.repoID }

// Name returns the tag name.
func (t Tag) Name() string { return t.name }

// CommitSHA returns the tagged commit SHA.
func (t Tag) CommitSHA() string { return t.commitSHA }

// Message returns the tag message (for annotated tags).
func (t Tag) Message() string { return t.message }

// Tagger returns the tagger (for annotated tags).
func (t Tag) Tagger() Author { return t.tagger }

// TaggedAt returns the tag timestamp (for annotated tags).
func (t Tag) TaggedAt() time.Time { return t.taggedAt }

// CreatedAt returns the creation timestamp.
func (t Tag) CreatedAt() time.Time { return t.createdAt }

// IsAnnotated returns true if this is an annotated tag.
func (t Tag) IsAnnotated() bool { return t.message != "" || !t.tagger.IsEmpty() }

// WithID returns a new Tag with the specified ID.
func (t Tag) WithID(id int64) Tag {
	t.id = id
	return t
}
