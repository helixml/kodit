package repository

import (
	"errors"
	"strings"
	"time"
)

// ErrEmptyRemoteURL indicates a repo was created with an empty remote URL.
var ErrEmptyRemoteURL = errors.New("remote URL cannot be empty")

// ErrNotCloned indicates an operation requires a working copy that does not exist.
var ErrNotCloned = errors.New("repository has not been cloned")

// Repository represents a tracked Git repository (aggregate root).
type Repository struct {
	id             int64
	pipelineID     int64
	remoteURL      string
	sanitizedURL   string
	upstreamURL    string
	workingCopy    WorkingCopy
	trackingConfig TrackingConfig
	chunkingConfig ChunkingConfig
	createdAt      time.Time
	updatedAt      time.Time
	lastScannedAt  time.Time
}

// NewRepository creates a new Repository with a remote URL.
func NewRepository(remoteURL string) (Repository, error) {
	if remoteURL == "" {
		return Repository{}, ErrEmptyRemoteURL
	}
	now := time.Now()
	return Repository{
		remoteURL:      remoteURL,
		chunkingConfig: DefaultChunkingConfig(),
		createdAt:      now,
		updatedAt:      now,
	}, nil
}

// ReconstructRepository reconstructs a Repository from persistence.
func ReconstructRepository(
	id int64,
	pipelineID int64,
	remoteURL string,
	sanitizedURL string,
	upstreamURL string,
	workingCopy WorkingCopy,
	trackingConfig TrackingConfig,
	chunkingConfig ChunkingConfig,
	createdAt, updatedAt time.Time,
	lastScannedAt time.Time,
) Repository {
	return Repository{
		id:             id,
		pipelineID:     pipelineID,
		remoteURL:      remoteURL,
		sanitizedURL:   sanitizedURL,
		upstreamURL:    upstreamURL,
		workingCopy:    workingCopy,
		trackingConfig: trackingConfig,
		chunkingConfig: chunkingConfig,
		createdAt:      createdAt,
		updatedAt:      updatedAt,
		lastScannedAt:  lastScannedAt,
	}
}

// ID returns the repository ID.
func (r Repository) ID() int64 { return r.id }

// PipelineID returns the pipeline identifier.
func (r Repository) PipelineID() int64 { return r.pipelineID }

// WithPipelineID returns a copy with the given pipeline assigned.
func (r Repository) WithPipelineID(id int64) Repository {
	r.pipelineID = id
	r.updatedAt = time.Now()
	return r
}

// RemoteURL returns the remote URL.
func (r Repository) RemoteURL() string { return r.remoteURL }

// SanitizedURL returns the remote URL with credentials stripped.
func (r Repository) SanitizedURL() string { return r.sanitizedURL }

// UpstreamURL returns the upstream URL. Falls back to sanitizedURL when
// no explicit upstream has been set.
func (r Repository) UpstreamURL() string {
	if r.upstreamURL != "" {
		return r.upstreamURL
	}
	return r.sanitizedURL
}

// WithUpstreamURL returns a copy with the given upstream URL.
func (r Repository) WithUpstreamURL(url string) Repository {
	r.upstreamURL = url
	r.updatedAt = time.Now()
	return r
}

// WorkingCopy returns the local working copy.
func (r Repository) WorkingCopy() WorkingCopy { return r.workingCopy }

// TrackingConfig returns the tracking configuration.
func (r Repository) TrackingConfig() TrackingConfig { return r.trackingConfig }

// ChunkingConfig returns the chunking configuration.
func (r Repository) ChunkingConfig() ChunkingConfig { return r.chunkingConfig }

// WithChunkingConfig returns a new Repository with the specified chunking config.
func (r Repository) WithChunkingConfig(cc ChunkingConfig) Repository {
	r.chunkingConfig = cc
	r.updatedAt = time.Now()
	return r
}

// CreatedAt returns the creation timestamp.
func (r Repository) CreatedAt() time.Time { return r.createdAt }

// UpdatedAt returns the last update timestamp.
func (r Repository) UpdatedAt() time.Time { return r.updatedAt }

// HasWorkingCopy returns true if a working copy exists.
func (r Repository) HasWorkingCopy() bool { return !r.workingCopy.IsEmpty() }

// IsLocal returns true if the repository uses a file:// URI, meaning the
// working copy directory is owned by the user and was not cloned by Kodit.
func (r Repository) IsLocal() bool { return strings.HasPrefix(r.remoteURL, "file://") }

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

// LastScannedAt returns the last scanned timestamp.
func (r Repository) LastScannedAt() time.Time { return r.lastScannedAt }

// WithLastScannedAt returns a new Repository with the specified last scanned time.
func (r Repository) WithLastScannedAt(t time.Time) Repository {
	r.lastScannedAt = t
	return r
}

// WithID returns a new Repository with the specified ID (used after persistence).
func (r Repository) WithID(id int64) Repository {
	r.id = id
	return r
}
