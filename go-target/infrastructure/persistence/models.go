package persistence

import (
	"database/sql"
	"encoding/json"
	"time"
)

// RepositoryModel represents a Git repository in the database.
type RepositoryModel struct {
	ID                 int64      `gorm:"primaryKey;autoIncrement"`
	SanitizedRemoteURI string     `gorm:"column:sanitized_remote_uri;index;uniqueIndex;size:1024"`
	RemoteURI          string     `gorm:"column:remote_uri;size:1024"`
	ClonedPath         *string    `gorm:"column:cloned_path;size:1024"`
	LastScannedAt      *time.Time `gorm:"column:last_scanned_at"`
	NumCommits         int        `gorm:"column:num_commits;default:0"`
	NumBranches        int        `gorm:"column:num_branches;default:0"`
	NumTags            int        `gorm:"column:num_tags;default:0"`
	TrackingType       string     `gorm:"column:tracking_type;index;size:255"`
	TrackingName       string     `gorm:"column:tracking_name;index;size:255"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

// TableName returns the table name.
func (RepositoryModel) TableName() string {
	return "git_repos"
}

// CommitModel represents a Git commit in the database.
type CommitModel struct {
	CommitSHA       string    `gorm:"column:commit_sha;primaryKey;size:64"`
	RepoID          int64     `gorm:"column:repo_id;index"`
	Date            time.Time `gorm:"column:date"`
	Message         string    `gorm:"column:message;type:text"`
	ParentCommitSHA *string   `gorm:"column:parent_commit_sha;index;size:64"`
	Author          string    `gorm:"column:author;index;size:255"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

// TableName returns the table name.
func (CommitModel) TableName() string {
	return "git_commits"
}

// BranchModel represents a Git branch in the database.
type BranchModel struct {
	RepoID        int64     `gorm:"column:repo_id;primaryKey;index"`
	Name          string    `gorm:"column:name;primaryKey;index;size:255"`
	HeadCommitSHA string    `gorm:"column:head_commit_sha;index;size:64"`
	IsDefault     bool      `gorm:"column:is_default;default:false"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

// TableName returns the table name.
func (BranchModel) TableName() string {
	return "git_branches"
}

// TagModel represents a Git tag in the database.
type TagModel struct {
	RepoID          int64      `gorm:"column:repo_id;primaryKey;index"`
	Name            string     `gorm:"column:name;primaryKey;index;size:255"`
	TargetCommitSHA string     `gorm:"column:target_commit_sha;index;size:64"`
	Message         *string    `gorm:"column:message;type:text"`
	TaggerName      *string    `gorm:"column:tagger_name;size:255"`
	TaggerEmail     *string    `gorm:"column:tagger_email;size:255"`
	TaggedAt        *time.Time `gorm:"column:tagged_at"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

// TableName returns the table name.
func (TagModel) TableName() string {
	return "git_tags"
}

// FileModel represents a Git file in the database.
type FileModel struct {
	CommitSHA string    `gorm:"column:commit_sha;primaryKey;size:64"`
	Path      string    `gorm:"column:path;primaryKey;size:1024"`
	BlobSHA   string    `gorm:"column:blob_sha;index;size:64"`
	MimeType  string    `gorm:"column:mime_type;index;size:255"`
	Extension string    `gorm:"column:extension;index;size:255"`
	Size      int64     `gorm:"column:size"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

// TableName returns the table name.
func (FileModel) TableName() string {
	return "git_commit_files"
}

// SnippetModel represents a code snippet in the database.
type SnippetModel struct {
	SHA       string    `gorm:"column:sha;primaryKey"`
	Content   string    `gorm:"column:content;type:text"`
	Extension string    `gorm:"column:extension"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name.
func (SnippetModel) TableName() string {
	return "snippets"
}

// SnippetCommitAssociationModel links snippets to commits.
type SnippetCommitAssociationModel struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetSHA string    `gorm:"column:snippet_sha;index"`
	CommitSHA  string    `gorm:"column:commit_sha;index"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
}

// TableName returns the table name.
func (SnippetCommitAssociationModel) TableName() string {
	return "snippet_commit_associations"
}

// SnippetFileDerivationModel links snippets to source files.
type SnippetFileDerivationModel struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetSHA string    `gorm:"column:snippet_sha;index"`
	FileID     int64     `gorm:"column:file_id;index"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
}

// TableName returns the table name.
func (SnippetFileDerivationModel) TableName() string {
	return "snippet_file_derivations"
}

// CommitIndexModel represents the commit indexing status.
type CommitIndexModel struct {
	CommitSHA             string         `gorm:"column:commit_sha;primaryKey"`
	Status                string         `gorm:"column:status;index"`
	IndexedAt             sql.NullTime   `gorm:"column:indexed_at"`
	ErrorMessage          sql.NullString `gorm:"column:error_message"`
	FilesProcessed        int            `gorm:"column:files_processed;default:0"`
	ProcessingTimeSeconds float64        `gorm:"column:processing_time_seconds;default:0.0"`
	CreatedAt             time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time      `gorm:"column:updated_at;not null"`
}

// TableName returns the table name.
func (CommitIndexModel) TableName() string {
	return "commit_indexes"
}

// EnrichmentModel represents an enrichment in the database.
type EnrichmentModel struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Type      string    `gorm:"column:type;not null;index"`
	Subtype   string    `gorm:"column:subtype;not null;index"`
	Content   string    `gorm:"column:content;type:text;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name.
func (EnrichmentModel) TableName() string {
	return "enrichments_v2"
}

// EnrichmentAssociationModel links enrichments to entities.
type EnrichmentAssociationModel struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	EnrichmentID int64     `gorm:"column:enrichment_id;not null;index"`
	EntityType   string    `gorm:"column:entity_type;size:50;not null;index"`
	EntityID     string    `gorm:"column:entity_id;size:255;not null;index"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name.
func (EnrichmentAssociationModel) TableName() string {
	return "enrichment_associations"
}

// EmbeddingModel represents a vector embedding in the database.
type EmbeddingModel struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SnippetID string    `gorm:"column:snippet_id;index"`
	Type      int       `gorm:"column:type;index"`
	Embedding []float64 `gorm:"column:embedding;type:json"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name.
func (EmbeddingModel) TableName() string {
	return "embeddings"
}

// EmbeddingType constants.
const (
	EmbeddingTypeCode = 1
	EmbeddingTypeText = 2
)

// TaskModel represents a task in the database.
type TaskModel struct {
	ID        int64           `gorm:"column:id;primaryKey;autoIncrement"`
	DedupKey  string          `gorm:"column:dedup_key;type:varchar(255);index;not null"`
	Type      string          `gorm:"column:type;type:varchar(255);index;not null"`
	Payload   json.RawMessage `gorm:"column:payload;type:jsonb"`
	Priority  int             `gorm:"column:priority;not null"`
	CreatedAt time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName returns the table name.
func (TaskModel) TableName() string {
	return "tasks"
}

// TaskStatusModel represents task status in the database.
type TaskStatusModel struct {
	ID            string    `gorm:"column:id;type:varchar(255);primaryKey;index;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
	Operation     string    `gorm:"column:operation;type:varchar(255);index;not null"`
	TrackableID   *int64    `gorm:"column:trackable_id;index"`
	TrackableType *string   `gorm:"column:trackable_type;type:varchar(255);index"`
	ParentID      *string   `gorm:"column:parent;type:varchar(255);index"`
	Message       string    `gorm:"column:message;type:text;default:''"`
	State         string    `gorm:"column:state;type:varchar(255);default:''"`
	Error         string    `gorm:"column:error;type:text;default:''"`
	Total         int       `gorm:"column:total;default:0"`
	Current       int       `gorm:"column:current;default:0"`
}

// TableName returns the table name.
func (TaskStatusModel) TableName() string {
	return "task_status"
}
