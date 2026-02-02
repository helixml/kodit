package indexing

import (
	"time"

	"github.com/helixml/kodit/internal/domain"
)

// CommitIndex represents an indexed commit with its snippets.
// This is the aggregate root for indexed commit data.
type CommitIndex struct {
	commitSHA             string
	snippets              []Snippet
	status                domain.IndexStatus
	indexedAt             time.Time
	errorMessage          string
	filesProcessed        int
	processingTimeSeconds float64
	createdAt             time.Time
	updatedAt             time.Time
}

// NewCommitIndex creates a new CommitIndex in pending status.
func NewCommitIndex(commitSHA string) CommitIndex {
	now := time.Now()
	return CommitIndex{
		commitSHA:             commitSHA,
		snippets:              []Snippet{},
		status:                domain.IndexStatusPending,
		filesProcessed:        0,
		processingTimeSeconds: 0,
		createdAt:             now,
		updatedAt:             now,
	}
}

// ReconstructCommitIndex reconstructs a CommitIndex from persistence.
func ReconstructCommitIndex(
	commitSHA string,
	snippets []Snippet,
	status domain.IndexStatus,
	indexedAt time.Time,
	errorMessage string,
	filesProcessed int,
	processingTimeSeconds float64,
	createdAt, updatedAt time.Time,
) CommitIndex {
	snips := make([]Snippet, len(snippets))
	copy(snips, snippets)

	return CommitIndex{
		commitSHA:             commitSHA,
		snippets:              snips,
		status:                status,
		indexedAt:             indexedAt,
		errorMessage:          errorMessage,
		filesProcessed:        filesProcessed,
		processingTimeSeconds: processingTimeSeconds,
		createdAt:             createdAt,
		updatedAt:             updatedAt,
	}
}

// ID returns the unique identifier (commit SHA).
func (c CommitIndex) ID() string { return c.commitSHA }

// CommitSHA returns the commit SHA.
func (c CommitIndex) CommitSHA() string { return c.commitSHA }

// Snippets returns the indexed snippets.
func (c CommitIndex) Snippets() []Snippet {
	result := make([]Snippet, len(c.snippets))
	copy(result, c.snippets)
	return result
}

// SnippetCount returns the number of snippets.
func (c CommitIndex) SnippetCount() int { return len(c.snippets) }

// Status returns the indexing status.
func (c CommitIndex) Status() domain.IndexStatus { return c.status }

// IndexedAt returns when the indexing completed.
func (c CommitIndex) IndexedAt() time.Time { return c.indexedAt }

// ErrorMessage returns the error message if indexing failed.
func (c CommitIndex) ErrorMessage() string { return c.errorMessage }

// FilesProcessed returns the number of files processed.
func (c CommitIndex) FilesProcessed() int { return c.filesProcessed }

// ProcessingTimeSeconds returns the processing duration.
func (c CommitIndex) ProcessingTimeSeconds() float64 { return c.processingTimeSeconds }

// CreatedAt returns the creation timestamp.
func (c CommitIndex) CreatedAt() time.Time { return c.createdAt }

// UpdatedAt returns the last update timestamp.
func (c CommitIndex) UpdatedAt() time.Time { return c.updatedAt }

// Start transitions the index to in-progress status.
func (c CommitIndex) Start() CommitIndex {
	return CommitIndex{
		commitSHA:             c.commitSHA,
		snippets:              c.snippets,
		status:                domain.IndexStatusInProgress,
		indexedAt:             c.indexedAt,
		errorMessage:          c.errorMessage,
		filesProcessed:        c.filesProcessed,
		processingTimeSeconds: c.processingTimeSeconds,
		createdAt:             c.createdAt,
		updatedAt:             time.Now(),
	}
}

// Complete marks the indexing as successfully completed.
func (c CommitIndex) Complete(
	snippets []Snippet,
	filesProcessed int,
	processingTimeSeconds float64,
) CommitIndex {
	snips := make([]Snippet, len(snippets))
	copy(snips, snippets)

	now := time.Now()
	return CommitIndex{
		commitSHA:             c.commitSHA,
		snippets:              snips,
		status:                domain.IndexStatusCompleted,
		indexedAt:             now,
		errorMessage:          "",
		filesProcessed:        filesProcessed,
		processingTimeSeconds: processingTimeSeconds,
		createdAt:             c.createdAt,
		updatedAt:             now,
	}
}

// Fail marks the indexing as failed with an error message.
func (c CommitIndex) Fail(errorMessage string) CommitIndex {
	return CommitIndex{
		commitSHA:             c.commitSHA,
		snippets:              c.snippets,
		status:                domain.IndexStatusFailed,
		indexedAt:             c.indexedAt,
		errorMessage:          errorMessage,
		filesProcessed:        c.filesProcessed,
		processingTimeSeconds: c.processingTimeSeconds,
		createdAt:             c.createdAt,
		updatedAt:             time.Now(),
	}
}

// IsCompleted returns true if indexing completed successfully.
func (c CommitIndex) IsCompleted() bool {
	return c.status == domain.IndexStatusCompleted
}

// IsFailed returns true if indexing failed.
func (c CommitIndex) IsFailed() bool {
	return c.status == domain.IndexStatusFailed
}

// IsInProgress returns true if indexing is in progress.
func (c CommitIndex) IsInProgress() bool {
	return c.status == domain.IndexStatusInProgress
}

// IsPending returns true if indexing has not started.
func (c CommitIndex) IsPending() bool {
	return c.status == domain.IndexStatusPending
}
