package git

import (
	"errors"
	"time"
)

// ErrEmptyRemoteURL indicates a repo was created with an empty remote URL.
var ErrEmptyRemoteURL = errors.New("remote URL cannot be empty")

// Repo represents a tracked Git repository (aggregate root).
type Repo struct {
	id             int64
	remoteURL      string
	workingCopy    WorkingCopy
	trackingConfig TrackingConfig
	createdAt      time.Time
	updatedAt      time.Time
}

// NewRepo creates a new Repo with a remote URL.
func NewRepo(remoteURL string) (Repo, error) {
	if remoteURL == "" {
		return Repo{}, ErrEmptyRemoteURL
	}
	now := time.Now()
	return Repo{
		remoteURL: remoteURL,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// ReconstructRepo reconstructs a Repo from persistence.
func ReconstructRepo(
	id int64,
	remoteURL string,
	workingCopy WorkingCopy,
	trackingConfig TrackingConfig,
	createdAt, updatedAt time.Time,
) Repo {
	return Repo{
		id:             id,
		remoteURL:      remoteURL,
		workingCopy:    workingCopy,
		trackingConfig: trackingConfig,
		createdAt:      createdAt,
		updatedAt:      updatedAt,
	}
}

// ID returns the repository ID.
func (r Repo) ID() int64 { return r.id }

// RemoteURL returns the remote URL.
func (r Repo) RemoteURL() string { return r.remoteURL }

// WorkingCopy returns the local working copy.
func (r Repo) WorkingCopy() WorkingCopy { return r.workingCopy }

// TrackingConfig returns the tracking configuration.
func (r Repo) TrackingConfig() TrackingConfig { return r.trackingConfig }

// CreatedAt returns the creation timestamp.
func (r Repo) CreatedAt() time.Time { return r.createdAt }

// UpdatedAt returns the last update timestamp.
func (r Repo) UpdatedAt() time.Time { return r.updatedAt }

// HasWorkingCopy returns true if a working copy exists.
func (r Repo) HasWorkingCopy() bool { return !r.workingCopy.IsEmpty() }

// HasTrackingConfig returns true if tracking is configured.
func (r Repo) HasTrackingConfig() bool { return !r.trackingConfig.IsEmpty() }

// WithWorkingCopy returns a new Repo with the specified working copy.
func (r Repo) WithWorkingCopy(wc WorkingCopy) Repo {
	r.workingCopy = wc
	r.updatedAt = time.Now()
	return r
}

// WithTrackingConfig returns a new Repo with the specified tracking config.
func (r Repo) WithTrackingConfig(tc TrackingConfig) Repo {
	r.trackingConfig = tc
	r.updatedAt = time.Now()
	return r
}

// WithID returns a new Repo with the specified ID (used after persistence).
func (r Repo) WithID(id int64) Repo {
	r.id = id
	return r
}
