// Package tracking provides progress tracking and reporting for long-running tasks.
package tracking

// ReferenceType represents types of git references that can be tracked.
type ReferenceType string

// ReferenceType values.
const (
	ReferenceTypeBranch    ReferenceType = "branch"
	ReferenceTypeTag       ReferenceType = "tag"
	ReferenceTypeCommitSHA ReferenceType = "commit_sha"
)

// String returns the string representation.
func (r ReferenceType) String() string {
	return string(r)
}

// Trackable represents a trackable reference point in a git repository.
// It is an immutable value object that identifies a specific point in git history.
type Trackable struct {
	refType    ReferenceType
	identifier string
	repoID     int64
}

// NewTrackable creates a new Trackable.
func NewTrackable(refType ReferenceType, identifier string, repoID int64) Trackable {
	return Trackable{
		refType:    refType,
		identifier: identifier,
		repoID:     repoID,
	}
}

// Type returns the reference type (branch, tag, or commit_sha).
func (t Trackable) Type() ReferenceType {
	return t.refType
}

// Identifier returns the identifier (branch name, tag name, or commit SHA).
func (t Trackable) Identifier() string {
	return t.identifier
}

// RepoID returns the repository ID.
func (t Trackable) RepoID() int64 {
	return t.repoID
}

// IsBranch returns true if this is a branch reference.
func (t Trackable) IsBranch() bool {
	return t.refType == ReferenceTypeBranch
}

// IsTag returns true if this is a tag reference.
func (t Trackable) IsTag() bool {
	return t.refType == ReferenceTypeTag
}

// IsCommitSHA returns true if this is a direct commit SHA reference.
func (t Trackable) IsCommitSHA() bool {
	return t.refType == ReferenceTypeCommitSHA
}
