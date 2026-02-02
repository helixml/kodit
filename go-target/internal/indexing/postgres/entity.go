package postgres

import (
	"database/sql"
	"time"
)

// CommitIndexEntity represents the commit_indexes table.
type CommitIndexEntity struct {
	CommitSHA             string         `gorm:"column:commit_sha;primaryKey"`
	Status                string         `gorm:"column:status;index"`
	IndexedAt             sql.NullTime   `gorm:"column:indexed_at"`
	ErrorMessage          sql.NullString `gorm:"column:error_message"`
	FilesProcessed        int            `gorm:"column:files_processed;default:0"`
	ProcessingTimeSeconds float64        `gorm:"column:processing_time_seconds;default:0.0"`
	CreatedAt             time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time      `gorm:"column:updated_at;not null"`
}

// TableName returns the table name for GORM.
func (CommitIndexEntity) TableName() string {
	return "commit_indexes"
}

// SnippetEntity represents a denormalized snippet view.
// Note: Snippets are content-addressed and associated with commits via
// enrichment associations. This entity is primarily used for reading.
type SnippetEntity struct {
	SHA       string    `gorm:"column:sha;primaryKey"`
	Content   string    `gorm:"column:content;type:text"`
	Extension string    `gorm:"column:extension"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name for GORM.
func (SnippetEntity) TableName() string {
	return "snippets"
}

// SnippetCommitAssociationEntity links snippets to commits.
type SnippetCommitAssociationEntity struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetSHA string    `gorm:"column:snippet_sha;index"`
	CommitSHA  string    `gorm:"column:commit_sha;index"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
}

// TableName returns the table name for GORM.
func (SnippetCommitAssociationEntity) TableName() string {
	return "snippet_commit_associations"
}

// SnippetFileDerivationEntity links snippets to source files.
type SnippetFileDerivationEntity struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetSHA string    `gorm:"column:snippet_sha;index"`
	FileID     int64     `gorm:"column:file_id;index"` // References git_commit_files
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
}

// TableName returns the table name for GORM.
func (SnippetFileDerivationEntity) TableName() string {
	return "snippet_file_derivations"
}

// EnrichmentEntity represents the enrichments_v2 table.
type EnrichmentEntity struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Type      string    `gorm:"column:type;not null;index"`
	Subtype   string    `gorm:"column:subtype;not null;index"`
	Content   string    `gorm:"column:content;type:text;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name for GORM.
func (EnrichmentEntity) TableName() string {
	return "enrichments_v2"
}

// EnrichmentAssociationEntity links enrichments to entities.
type EnrichmentAssociationEntity struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	EnrichmentID int64     `gorm:"column:enrichment_id;not null;index"`
	EntityType   string    `gorm:"column:entity_type;not null;index"`
	EntityID     string    `gorm:"column:entity_id;not null;index"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name for GORM.
func (EnrichmentAssociationEntity) TableName() string {
	return "enrichment_associations"
}

// EmbeddingEntity represents the embeddings table.
type EmbeddingEntity struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string    `gorm:"column:snippet_id;index"`
	Type      int       `gorm:"column:type;index"` // 1=CODE, 2=TEXT
	Embedding []float64 `gorm:"column:embedding;type:json"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name for GORM.
func (EmbeddingEntity) TableName() string {
	return "embeddings"
}

// EmbeddingTypeCode represents code embeddings.
const EmbeddingTypeCode = 1

// EmbeddingTypeText represents text/summary embeddings.
const EmbeddingTypeText = 2
