// Package enrichment provides domain types for AI-generated semantic metadata.
package enrichment

import (
	"time"
)

// Type represents the main enrichment category.
type Type string

// Enrichment type constants.
const (
	TypeArchitecture Type = "architecture"
	TypeDevelopment  Type = "development"
	TypeHistory      Type = "history"
	TypeUsage        Type = "usage"
)

// Subtype represents a specific enrichment within a type category.
type Subtype string

// Architecture subtypes.
const (
	SubtypePhysical       Subtype = "physical"
	SubtypeDatabaseSchema Subtype = "database_schema"
)

// Development subtypes.
const (
	SubtypeSnippet        Subtype = "snippet"
	SubtypeSnippetSummary Subtype = "snippet_summary"
	SubtypeExample        Subtype = "example"
	SubtypeExampleSummary Subtype = "example_summary"
	SubtypeChunk          Subtype = "chunk"
)

// History subtypes.
const (
	SubtypeCommitDescription Subtype = "commit_description"
)

// Usage subtypes.
const (
	SubtypeCookbook Subtype = "cookbook"
	SubtypeAPIDocs  Subtype = "api_docs"
	SubtypeWiki     Subtype = "wiki"
)

// EntityTypeKey represents the type of entity an enrichment is attached to.
type EntityTypeKey string

// Entity type key constants.
const (
	EntityTypeCommit     EntityTypeKey = "git_commits"
	EntityTypeSnippet    EntityTypeKey = "enrichments_v2"
	EntityTypeFile       EntityTypeKey = "git_commit_files"
	EntityTypeRepository EntityTypeKey = "git_repos"
)

// Enrichment represents AI-generated semantic metadata that can be attached to entities.
// This is an immutable value object identified by its ID once persisted.
type Enrichment struct {
	id        int64
	content   string
	typ       Type
	subtype   Subtype
	entityKey EntityTypeKey
	language  string
	createdAt time.Time
	updatedAt time.Time
}

// NewEnrichment creates an enrichment for new instances (not yet persisted).
func NewEnrichment(typ Type, subtype Subtype, entityKey EntityTypeKey, content string) Enrichment {
	now := time.Now()
	return Enrichment{
		id:        0,
		content:   content,
		typ:       typ,
		subtype:   subtype,
		entityKey: entityKey,
		language:  "",
		createdAt: now,
		updatedAt: now,
	}
}

// NewEnrichmentWithLanguage creates an enrichment with an associated language (for API docs).
func NewEnrichmentWithLanguage(typ Type, subtype Subtype, entityKey EntityTypeKey, content, language string) Enrichment {
	e := NewEnrichment(typ, subtype, entityKey, content)
	e.language = language
	return e
}

// ReconstructEnrichment recreates an enrichment from persistence (for repository use).
func ReconstructEnrichment(
	id int64,
	typ Type,
	subtype Subtype,
	entityKey EntityTypeKey,
	content string,
	language string,
	createdAt time.Time,
	updatedAt time.Time,
) Enrichment {
	return Enrichment{
		id:        id,
		content:   content,
		typ:       typ,
		subtype:   subtype,
		entityKey: entityKey,
		language:  language,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// ID returns the enrichment's database identifier.
func (e Enrichment) ID() int64 {
	return e.id
}

// Content returns the enrichment content (AI-generated text).
func (e Enrichment) Content() string {
	return e.content
}

// Type returns the enrichment's main type category.
func (e Enrichment) Type() Type {
	return e.typ
}

// Subtype returns the enrichment's specific subtype.
func (e Enrichment) Subtype() Subtype {
	return e.subtype
}

// EntityTypeKey returns the type of entity this enrichment is attached to.
func (e Enrichment) EntityTypeKey() EntityTypeKey {
	return e.entityKey
}

// Language returns the associated language (only applicable for API docs enrichments).
func (e Enrichment) Language() string {
	return e.language
}

// CreatedAt returns when the enrichment was created.
func (e Enrichment) CreatedAt() time.Time {
	return e.createdAt
}

// UpdatedAt returns when the enrichment was last updated.
func (e Enrichment) UpdatedAt() time.Time {
	return e.updatedAt
}

// IsCommitEnrichment returns true if this enrichment is attached to commits.
func (e Enrichment) IsCommitEnrichment() bool {
	return e.entityKey == EntityTypeCommit
}

// WithID returns a copy of the enrichment with the specified ID.
func (e Enrichment) WithID(id int64) Enrichment {
	e.id = id
	return e
}

// WithContent returns a copy of the enrichment with updated content.
func (e Enrichment) WithContent(content string) Enrichment {
	e.content = content
	e.updatedAt = time.Now()
	return e
}
