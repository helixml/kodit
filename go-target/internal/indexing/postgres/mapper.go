package postgres

import (
	"database/sql"
	"time"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
)

// CommitIndexMapper converts between CommitIndex domain objects and database entities.
type CommitIndexMapper struct{}

// ToEntity converts a domain CommitIndex to a database entity.
func (CommitIndexMapper) ToEntity(index indexing.CommitIndex) CommitIndexEntity {
	var indexedAt sql.NullTime
	if !index.IndexedAt().IsZero() {
		indexedAt = sql.NullTime{Time: index.IndexedAt(), Valid: true}
	}

	var errorMessage sql.NullString
	if index.ErrorMessage() != "" {
		errorMessage = sql.NullString{String: index.ErrorMessage(), Valid: true}
	}

	return CommitIndexEntity{
		CommitSHA:             index.CommitSHA(),
		Status:                string(index.Status()),
		IndexedAt:             indexedAt,
		ErrorMessage:          errorMessage,
		FilesProcessed:        index.FilesProcessed(),
		ProcessingTimeSeconds: index.ProcessingTimeSeconds(),
		CreatedAt:             index.CreatedAt(),
		UpdatedAt:             index.UpdatedAt(),
	}
}

// ToDomain converts a database entity to a domain CommitIndex.
func (CommitIndexMapper) ToDomain(entity CommitIndexEntity) indexing.CommitIndex {
	var indexedAt time.Time
	if entity.IndexedAt.Valid {
		indexedAt = entity.IndexedAt.Time
	}

	var errorMessage string
	if entity.ErrorMessage.Valid {
		errorMessage = entity.ErrorMessage.String
	}

	return indexing.ReconstructCommitIndex(
		entity.CommitSHA,
		nil, // Snippets loaded separately
		domain.IndexStatus(entity.Status),
		indexedAt,
		errorMessage,
		entity.FilesProcessed,
		entity.ProcessingTimeSeconds,
		entity.CreatedAt,
		entity.UpdatedAt,
	)
}

// SnippetMapper converts between Snippet domain objects and database entities.
type SnippetMapper struct{}

// ToEntity converts a domain Snippet to a database entity.
func (SnippetMapper) ToEntity(snippet indexing.Snippet) SnippetEntity {
	return SnippetEntity{
		SHA:       snippet.SHA(),
		Content:   snippet.Content(),
		Extension: snippet.Extension(),
		CreatedAt: snippet.CreatedAt(),
		UpdatedAt: snippet.UpdatedAt(),
	}
}

// ToDomain converts a database entity to a domain Snippet.
// Note: DerivesFrom and Enrichments must be loaded separately.
func (SnippetMapper) ToDomain(entity SnippetEntity) indexing.Snippet {
	return indexing.ReconstructSnippet(
		entity.SHA,
		entity.Content,
		entity.Extension,
		nil, // DerivesFrom loaded separately
		nil, // Enrichments loaded separately
		entity.CreatedAt,
		entity.UpdatedAt,
	)
}
