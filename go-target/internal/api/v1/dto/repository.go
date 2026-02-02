// Package dto provides data transfer objects for the API layer.
package dto

import "time"

// RepositoryRequest represents a request to add a repository.
type RepositoryRequest struct {
	RemoteURL string `json:"remote_url"`
	Branch    string `json:"branch,omitempty"`
	Tag       string `json:"tag,omitempty"`
	Commit    string `json:"commit,omitempty"`
}

// RepositoryResponse represents a repository in API responses.
type RepositoryResponse struct {
	ID            int64     `json:"id"`
	RemoteURL     string    `json:"remote_url"`
	WorkingCopy   string    `json:"working_copy,omitempty"`
	TrackingType  string    `json:"tracking_type,omitempty"`
	TrackingValue string    `json:"tracking_value,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RepositoryListResponse represents a list of repositories.
type RepositoryListResponse struct {
	Data       []RepositoryResponse `json:"data"`
	TotalCount int                  `json:"total_count"`
}

// RepositorySummaryResponse represents a repository summary with status.
type RepositorySummaryResponse struct {
	Repository     RepositoryResponse `json:"repository"`
	SnippetCount   int                `json:"snippet_count"`
	CommitCount    int                `json:"commit_count"`
	IndexingStatus string             `json:"indexing_status"`
}
