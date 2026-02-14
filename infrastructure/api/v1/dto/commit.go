package dto

import (
	"time"

	"github.com/helixml/kodit/infrastructure/api/jsonapi"
)

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
	Data  []CommitData   `json:"data"`
	Meta  *jsonapi.Meta  `json:"meta,omitempty"`
	Links *jsonapi.Links `json:"links,omitempty"`
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
	Data  []FileData     `json:"data"`
	Meta  *jsonapi.Meta  `json:"meta,omitempty"`
	Links *jsonapi.Links `json:"links,omitempty"`
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
	Data  []TagData      `json:"data"`
	Meta  *jsonapi.Meta  `json:"meta,omitempty"`
	Links *jsonapi.Links `json:"links,omitempty"`
}
