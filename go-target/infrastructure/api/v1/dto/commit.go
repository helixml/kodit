package dto

import "time"

// CommitResponse represents a commit in API responses.
type CommitResponse struct {
	SHA          string    `json:"sha"`
	RepositoryID int64     `json:"repository_id"`
	Message      string    `json:"message"`
	AuthorName   string    `json:"author_name"`
	AuthorEmail  string    `json:"author_email"`
	CommittedAt  time.Time `json:"committed_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CommitListResponse represents a list of commits.
type CommitListResponse struct {
	Data       []CommitResponse `json:"data"`
	TotalCount int              `json:"total_count"`
}

// CommitFilterRequest represents filters for commit queries.
type CommitFilterRequest struct {
	RepositoryID int64      `json:"repository_id,omitempty"`
	Author       string     `json:"author,omitempty"`
	Since        *time.Time `json:"since,omitempty"`
	Until        *time.Time `json:"until,omitempty"`
}

// CommitAttributes represents commit attributes in JSON:API format.
type CommitAttributes struct {
	CommitSHA       string    `json:"commit_sha"`
	Date            time.Time `json:"date"`
	Message         string    `json:"message"`
	ParentCommitSHA string    `json:"parent_commit_sha"`
	Author          string    `json:"author"`
}

// CommitData represents commit data in JSON:API format.
type CommitData struct {
	Type       string           `json:"type"`
	ID         string           `json:"id"`
	Attributes CommitAttributes `json:"attributes"`
}

// CommitJSONAPIListResponse represents a list of commits in JSON:API format.
type CommitJSONAPIListResponse struct {
	Data []CommitData `json:"data"`
}

// CommitJSONAPIResponse represents a single commit in JSON:API format.
type CommitJSONAPIResponse struct {
	Data CommitData `json:"data"`
}

// FileAttributes represents file attributes in JSON:API format.
type FileAttributes struct {
	BlobSHA   string `json:"blob_sha"`
	Path      string `json:"path"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
}

// FileData represents file data in JSON:API format.
type FileData struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Attributes FileAttributes `json:"attributes"`
}

// FileJSONAPIResponse represents a single file in JSON:API format.
type FileJSONAPIResponse struct {
	Data FileData `json:"data"`
}

// FileJSONAPIListResponse represents a list of files in JSON:API format.
type FileJSONAPIListResponse struct {
	Data []FileData `json:"data"`
}

// TagAttributes represents tag attributes in JSON:API format.
type TagAttributes struct {
	Name            string `json:"name"`
	TargetCommitSHA string `json:"target_commit_sha"`
	IsVersionTag    bool   `json:"is_version_tag"`
}

// TagData represents tag data in JSON:API format.
type TagData struct {
	Type       string        `json:"type"`
	ID         string        `json:"id"`
	Attributes TagAttributes `json:"attributes"`
}

// TagJSONAPIResponse represents a single tag in JSON:API format.
type TagJSONAPIResponse struct {
	Data TagData `json:"data"`
}

// TagJSONAPIListResponse represents a list of tags in JSON:API format.
type TagJSONAPIListResponse struct {
	Data []TagData `json:"data"`
}
