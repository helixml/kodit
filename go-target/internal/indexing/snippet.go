// Package indexing provides snippet extraction and indexing functionality.
package indexing

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/git"
)

// Snippet represents a content-addressed code snippet.
// Snippets are identified by SHA256 hash of their content to prevent duplicates.
type Snippet struct {
	sha         string
	content     string
	extension   string
	derivesFrom []git.File
	enrichments []domain.Enrichment
	createdAt   time.Time
	updatedAt   time.Time
}

// NewSnippet creates a new Snippet with content-addressed SHA.
func NewSnippet(content, extension string, derivesFrom []git.File) Snippet {
	now := time.Now()

	files := make([]git.File, len(derivesFrom))
	copy(files, derivesFrom)

	return Snippet{
		sha:         computeSHA(content),
		content:     content,
		extension:   extension,
		derivesFrom: files,
		enrichments: []domain.Enrichment{},
		createdAt:   now,
		updatedAt:   now,
	}
}

// ReconstructSnippet reconstructs a Snippet from persistence.
func ReconstructSnippet(
	sha, content, extension string,
	derivesFrom []git.File,
	enrichments []domain.Enrichment,
	createdAt, updatedAt time.Time,
) Snippet {
	files := make([]git.File, len(derivesFrom))
	copy(files, derivesFrom)

	enrich := make([]domain.Enrichment, len(enrichments))
	copy(enrich, enrichments)

	return Snippet{
		sha:         sha,
		content:     content,
		extension:   extension,
		derivesFrom: files,
		enrichments: enrich,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}

// SHA returns the content-addressed identifier.
func (s Snippet) SHA() string { return s.sha }

// Content returns the snippet code content.
func (s Snippet) Content() string { return s.content }

// Extension returns the file extension.
func (s Snippet) Extension() string { return s.extension }

// DerivesFrom returns the source files this snippet was extracted from.
func (s Snippet) DerivesFrom() []git.File {
	result := make([]git.File, len(s.derivesFrom))
	copy(result, s.derivesFrom)
	return result
}

// Enrichments returns the enrichments attached to this snippet.
func (s Snippet) Enrichments() []domain.Enrichment {
	result := make([]domain.Enrichment, len(s.enrichments))
	copy(result, s.enrichments)
	return result
}

// CreatedAt returns the creation timestamp.
func (s Snippet) CreatedAt() time.Time { return s.createdAt }

// UpdatedAt returns the last update timestamp.
func (s Snippet) UpdatedAt() time.Time { return s.updatedAt }

// WithEnrichments returns a new Snippet with additional enrichments.
func (s Snippet) WithEnrichments(enrichments []domain.Enrichment) Snippet {
	existing := make([]domain.Enrichment, len(s.enrichments))
	copy(existing, s.enrichments)
	existing = append(existing, enrichments...)

	return Snippet{
		sha:         s.sha,
		content:     s.content,
		extension:   s.extension,
		derivesFrom: s.derivesFrom,
		enrichments: existing,
		createdAt:   s.createdAt,
		updatedAt:   time.Now(),
	}
}

// computeSHA computes the SHA256 hash of the content.
func computeSHA(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// ComputeSHA is a public function to compute SHA for external use.
func ComputeSHA(content string) string {
	return computeSHA(content)
}
