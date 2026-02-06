package postgres

import (
	"time"
)

// RepoEntity represents a Git repository in the database.
type RepoEntity struct {
	ID                int64     `gorm:"primaryKey;autoIncrement"`
	SanitizedRemoteURI string    `gorm:"column:sanitized_remote_uri;index;uniqueIndex;size:1024"`
	RemoteURI         string    `gorm:"column:remote_uri;size:1024"`
	ClonedPath        *string   `gorm:"column:cloned_path;size:1024"`
	LastScannedAt     *time.Time `gorm:"column:last_scanned_at"`
	NumCommits        int       `gorm:"column:num_commits;default:0"`
	NumBranches       int       `gorm:"column:num_branches;default:0"`
	NumTags           int       `gorm:"column:num_tags;default:0"`
	TrackingType      string    `gorm:"column:tracking_type;index;size:255"`
	TrackingName      string    `gorm:"column:tracking_name;index;size:255"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

// TableName returns the table name for GitRepoEntity.
func (RepoEntity) TableName() string {
	return "git_repos"
}

// CommitEntity represents a Git commit in the database.
type CommitEntity struct {
	CommitSHA       string    `gorm:"column:commit_sha;primaryKey;size:64"`
	RepoID          int64     `gorm:"column:repo_id;index"`
	Date            time.Time `gorm:"column:date"`
	Message         string    `gorm:"column:message;type:text"`
	ParentCommitSHA *string   `gorm:"column:parent_commit_sha;index;size:64"`
	Author          string    `gorm:"column:author;index;size:255"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

// TableName returns the table name for GitCommitEntity.
func (CommitEntity) TableName() string {
	return "git_commits"
}

// BranchEntity represents a Git branch in the database.
// Note: This has a composite primary key (repo_id, name).
type BranchEntity struct {
	RepoID        int64     `gorm:"column:repo_id;primaryKey;index"`
	Name          string    `gorm:"column:name;primaryKey;index;size:255"`
	HeadCommitSHA string    `gorm:"column:head_commit_sha;index;size:64"`
	IsDefault     bool      `gorm:"column:is_default;default:false"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

// TableName returns the table name for GitBranchEntity.
func (BranchEntity) TableName() string {
	return "git_branches"
}

// TagEntity represents a Git tag in the database.
// Note: This has a composite primary key (repo_id, name).
type TagEntity struct {
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

// TableName returns the table name for GitTagEntity.
func (TagEntity) TableName() string {
	return "git_tags"
}

// FileEntity represents a Git file in the database.
type FileEntity struct {
	ID        int64     `gorm:"column:id;autoIncrement"`
	CommitSHA string    `gorm:"column:commit_sha;primaryKey;uniqueIndex:idx_commit_path;size:64"`
	Path      string    `gorm:"column:path;primaryKey;uniqueIndex:idx_commit_path;size:1024"`
	BlobSHA   string    `gorm:"column:blob_sha;index;size:64"`
	MimeType  string    `gorm:"column:mime_type;index;size:255"`
	Extension string    `gorm:"column:extension;index;size:255"`
	Size      int64     `gorm:"column:size"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

// TableName returns the table name for GitFileEntity.
func (FileEntity) TableName() string {
	return "git_commit_files"
}
