// Package repository provides high-level repository lifecycle management.
package repository

import (
	"time"

	"github.com/helixml/kodit/internal/git"
)

// SourceStatus represents the current state of a repository source.
type SourceStatus string

// Status values for a repository source.
const (
	StatusPending  SourceStatus = "pending"
	StatusCloning  SourceStatus = "cloning"
	StatusCloned   SourceStatus = "cloned"
	StatusSyncing  SourceStatus = "syncing"
	StatusFailed   SourceStatus = "failed"
	StatusDeleting SourceStatus = "deleting"
)

// String returns the string representation of the status.
func (s SourceStatus) String() string {
	return string(s)
}

// IsTerminal returns true if the status is a terminal state.
func (s SourceStatus) IsTerminal() bool {
	return s == StatusCloned || s == StatusFailed
}

// Source represents a repository being managed by the system.
// It wraps a GitRepo and provides lifecycle management operations.
type Source struct {
	repo      git.Repo
	status    SourceStatus
	lastError string
}

// NewSource creates a new Source from a GitRepo.
func NewSource(repo git.Repo) Source {
	status := StatusPending
	if repo.HasWorkingCopy() {
		status = StatusCloned
	}
	return Source{
		repo:   repo,
		status: status,
	}
}

// ReconstructSource reconstructs a Source from persistence.
func ReconstructSource(repo git.Repo, status SourceStatus, lastError string) Source {
	return Source{
		repo:      repo,
		status:    status,
		lastError: lastError,
	}
}

// ID returns the repository ID.
func (s Source) ID() int64 {
	return s.repo.ID()
}

// RemoteURL returns the repository remote URL.
func (s Source) RemoteURL() string {
	return s.repo.RemoteURL()
}

// WorkingCopy returns the local working copy, if available.
func (s Source) WorkingCopy() git.WorkingCopy {
	return s.repo.WorkingCopy()
}

// TrackingConfig returns the tracking configuration.
func (s Source) TrackingConfig() git.TrackingConfig {
	return s.repo.TrackingConfig()
}

// Repo returns the underlying GitRepo.
func (s Source) Repo() git.Repo {
	return s.repo
}

// Status returns the current source status.
func (s Source) Status() SourceStatus {
	return s.status
}

// LastError returns the last error message, if any.
func (s Source) LastError() string {
	return s.lastError
}

// CreatedAt returns when the repository was created.
func (s Source) CreatedAt() time.Time {
	return s.repo.CreatedAt()
}

// UpdatedAt returns when the repository was last updated.
func (s Source) UpdatedAt() time.Time {
	return s.repo.UpdatedAt()
}

// IsCloned returns true if the repository has been cloned.
func (s Source) IsCloned() bool {
	return s.repo.HasWorkingCopy()
}

// ClonedPath returns the local filesystem path, or empty string if not cloned.
func (s Source) ClonedPath() string {
	if !s.repo.HasWorkingCopy() {
		return ""
	}
	return s.repo.WorkingCopy().Path()
}

// WithStatus returns a new Source with the given status.
func (s Source) WithStatus(status SourceStatus) Source {
	s.status = status
	return s
}

// WithError returns a new Source with an error status and message.
func (s Source) WithError(err error) Source {
	s.status = StatusFailed
	if err != nil {
		s.lastError = err.Error()
	}
	return s
}

// WithWorkingCopy returns a new Source with an updated working copy.
func (s Source) WithWorkingCopy(wc git.WorkingCopy) Source {
	s.repo = s.repo.WithWorkingCopy(wc)
	s.status = StatusCloned
	s.lastError = ""
	return s
}

// WithTrackingConfig returns a new Source with an updated tracking config.
func (s Source) WithTrackingConfig(tc git.TrackingConfig) Source {
	s.repo = s.repo.WithTrackingConfig(tc)
	return s
}

// WithRepo returns a new Source with an updated GitRepo.
func (s Source) WithRepo(repo git.Repo) Source {
	s.repo = repo
	return s
}

// CanSync returns true if the repository can be synced.
func (s Source) CanSync() bool {
	return s.IsCloned() && s.status != StatusSyncing && s.status != StatusDeleting
}

// CanDelete returns true if the repository can be deleted.
func (s Source) CanDelete() bool {
	return s.status != StatusDeleting
}
