package indexing

import (
	"testing"
	"time"

	"github.com/helixml/kodit/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewCommitIndex(t *testing.T) {
	commitSHA := "abc123def456"

	index := NewCommitIndex(commitSHA)

	assert.Equal(t, commitSHA, index.CommitSHA())
	assert.Equal(t, commitSHA, index.ID())
	assert.Equal(t, domain.IndexStatusPending, index.Status())
	assert.Empty(t, index.Snippets())
	assert.Equal(t, 0, index.SnippetCount())
	assert.Equal(t, 0, index.FilesProcessed())
	assert.Equal(t, 0.0, index.ProcessingTimeSeconds())
	assert.Empty(t, index.ErrorMessage())
	assert.True(t, index.IndexedAt().IsZero())
	assert.False(t, index.CreatedAt().IsZero())
	assert.False(t, index.UpdatedAt().IsZero())
}

func TestCommitIndexStart(t *testing.T) {
	index := NewCommitIndex("abc123")
	assert.True(t, index.IsPending())
	assert.False(t, index.IsInProgress())

	started := index.Start()

	// Original unchanged
	assert.True(t, index.IsPending())

	// Started index is in progress
	assert.True(t, started.IsInProgress())
	assert.False(t, started.IsPending())
	assert.Equal(t, domain.IndexStatusInProgress, started.Status())
}

func TestCommitIndexComplete(t *testing.T) {
	index := NewCommitIndex("abc123").Start()

	snippets := []Snippet{
		NewSnippet("func foo() {}", "go", nil),
		NewSnippet("func bar() {}", "go", nil),
	}

	completed := index.Complete(snippets, 5, 2.5)

	assert.True(t, completed.IsCompleted())
	assert.False(t, completed.IsInProgress())
	assert.Equal(t, domain.IndexStatusCompleted, completed.Status())
	assert.Equal(t, 2, completed.SnippetCount())
	assert.Len(t, completed.Snippets(), 2)
	assert.Equal(t, 5, completed.FilesProcessed())
	assert.Equal(t, 2.5, completed.ProcessingTimeSeconds())
	assert.Empty(t, completed.ErrorMessage())
	assert.False(t, completed.IndexedAt().IsZero())
}

func TestCommitIndexFail(t *testing.T) {
	index := NewCommitIndex("abc123").Start()

	failed := index.Fail("parsing error: unexpected token")

	assert.True(t, failed.IsFailed())
	assert.False(t, failed.IsInProgress())
	assert.Equal(t, domain.IndexStatusFailed, failed.Status())
	assert.Equal(t, "parsing error: unexpected token", failed.ErrorMessage())
}

func TestReconstructCommitIndex(t *testing.T) {
	commitSHA := "abc123"
	snippets := []Snippet{NewSnippet("func test() {}", "go", nil)}
	status := domain.IndexStatusCompleted
	indexedAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	errorMessage := ""
	filesProcessed := 10
	processingTime := 5.5
	createdAt := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	index := ReconstructCommitIndex(
		commitSHA, snippets, status,
		indexedAt, errorMessage,
		filesProcessed, processingTime,
		createdAt, updatedAt,
	)

	assert.Equal(t, commitSHA, index.CommitSHA())
	assert.Equal(t, 1, index.SnippetCount())
	assert.Equal(t, status, index.Status())
	assert.Equal(t, indexedAt, index.IndexedAt())
	assert.Equal(t, filesProcessed, index.FilesProcessed())
	assert.Equal(t, processingTime, index.ProcessingTimeSeconds())
	assert.Equal(t, createdAt, index.CreatedAt())
	assert.Equal(t, updatedAt, index.UpdatedAt())
}

func TestCommitIndexStatusHelpers(t *testing.T) {
	tests := []struct {
		name         string
		index        CommitIndex
		isPending    bool
		isInProgress bool
		isCompleted  bool
		isFailed     bool
	}{
		{
			name:         "pending",
			index:        NewCommitIndex("abc"),
			isPending:    true,
			isInProgress: false,
			isCompleted:  false,
			isFailed:     false,
		},
		{
			name:         "in_progress",
			index:        NewCommitIndex("abc").Start(),
			isPending:    false,
			isInProgress: true,
			isCompleted:  false,
			isFailed:     false,
		},
		{
			name:         "completed",
			index:        NewCommitIndex("abc").Start().Complete(nil, 0, 0),
			isPending:    false,
			isInProgress: false,
			isCompleted:  true,
			isFailed:     false,
		},
		{
			name:         "failed",
			index:        NewCommitIndex("abc").Start().Fail("error"),
			isPending:    false,
			isInProgress: false,
			isCompleted:  false,
			isFailed:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isPending, tt.index.IsPending())
			assert.Equal(t, tt.isInProgress, tt.index.IsInProgress())
			assert.Equal(t, tt.isCompleted, tt.index.IsCompleted())
			assert.Equal(t, tt.isFailed, tt.index.IsFailed())
		})
	}
}

func TestCommitIndexSnippetsIsCopied(t *testing.T) {
	snippet1 := NewSnippet("func foo() {}", "go", nil)
	snippet2 := NewSnippet("func bar() {}", "go", nil)
	snippets := []Snippet{snippet1, snippet2}

	index := NewCommitIndex("abc").Start().Complete(snippets, 1, 0.5)

	// Modifying the original slice should not affect the index
	snippets[0] = NewSnippet("func baz() {}", "go", nil)

	// Index should still have original snippet
	assert.Equal(t, snippet1.SHA(), index.Snippets()[0].SHA())

	// Getting snippets multiple times should return copies
	retrieved := index.Snippets()
	assert.Len(t, retrieved, 2)
}

func TestCommitIndexImmutability(t *testing.T) {
	original := NewCommitIndex("abc123")
	originalSHA := original.CommitSHA()
	originalStatus := original.Status()

	// Operations return new instances
	started := original.Start()
	completed := started.Complete(nil, 5, 1.0)
	failed := started.Fail("error")

	// Original is unchanged
	assert.Equal(t, originalSHA, original.CommitSHA())
	assert.Equal(t, originalStatus, original.Status())
	assert.True(t, original.IsPending())

	// New instances have correct states
	assert.True(t, started.IsInProgress())
	assert.True(t, completed.IsCompleted())
	assert.True(t, failed.IsFailed())
}
