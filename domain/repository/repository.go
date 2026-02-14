package repository

import (
	"errors"
	"time"
)

// ErrEmptyRemoteURL indicates a repo was created with an empty remote URL.
var ErrEmptyRemoteURL = errors.New("remote URL cannot be empty")

// Repository represents a tracked Git repository (aggregate root).
type Repository struct {
	id             int64
	remoteURL      string
	workingCopy    WorkingCopy
	trackingConfig TrackingConfig
	createdAt      time.Time
	updatedAt      time.Time
}

// NewRepository creates a new Repository with a remote URL.
func NewRepository(remoteURL string) (Repository, error) {
	if remoteURL == "" {
		return Repository{}, ErrEmptyRemoteURL
	}
	now := time.Now()
	return Repository{
		remoteURL: remoteURL,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// ReconstructRepository reconstructs a Repository from persistence.
func ReconstructRepository(
	id int64,
	remoteURL string,
	workingCopy WorkingCopy,
	trackingConfig TrackingConfig,
	createdAt, updatedAt time.Time,
) Repository {
	return Repository{
		id:             id,
		remoteURL:      remoteURL,
		workingCopy:    workingCopy,
		trackingConfig: trackingConfig,
		createdAt:      createdAt,
		updatedAt:      updatedAt,
	}
}

// ID returns the repository ID.
func (r Repository) ID() int64 { return r.id }

// RemoteURL returns the remote URL.
func (r Repository) RemoteURL() string { return r.remoteURL }

// WorkingCopy returns the local working copy.
func (r Repository) WorkingCopy() WorkingCopy { return r.workingCopy }

// TrackingConfig returns the tracking configuration.
func (r Repository) TrackingConfig() TrackingConfig { return r.trackingConfig }

// CreatedAt returns the creation timestamp.
func (r Repository) CreatedAt() time.Time { return r.createdAt }

// UpdatedAt returns the last update timestamp.
func (r Repository) UpdatedAt() time.Time { return r.updatedAt }

// HasWorkingCopy returns true if a working copy exists.
func (r Repository) HasWorkingCopy() bool { return !r.workingCopy.IsEmpty() }

// HasTrackingConfig returns true if tracking is configured.
func (r Repository) HasTrackingConfig() bool { return !r.trackingConfig.IsEmpty() }

// WithWorkingCopy returns a new Repository with the specified working copy.
func (r Repository) WithWorkingCopy(wc WorkingCopy) Repository {
	r.workingCopy = wc
	r.updatedAt = time.Now()
	return r
}

// WithTrackingConfig returns a new Repository with the specified tracking config.
func (r Repository) WithTrackingConfig(tc TrackingConfig) Repository {
	r.trackingConfig = tc
	r.updatedAt = time.Now()
	return r
}

// WithID returns a new Repository with the specified ID (used after persistence).
func (r Repository) WithID(id int64) Repository {
	r.id = id
	return r
}
