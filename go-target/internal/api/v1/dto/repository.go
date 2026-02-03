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

// TaskStatusAttributes represents task status attributes in JSON:API format.
type TaskStatusAttributes struct {
	Step      string     `json:"step"`
	State     string     `json:"state"`
	Progress  float64    `json:"progress"`
	Total     int        `json:"total"`
	Current   int        `json:"current"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Error     string     `json:"error"`
	Message   string     `json:"message"`
}

// TaskStatusData represents task status data in JSON:API format.
type TaskStatusData struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Attributes TaskStatusAttributes `json:"attributes"`
}

// TaskStatusListResponse represents a list of task statuses in JSON:API format.
type TaskStatusListResponse struct {
	Data []TaskStatusData `json:"data"`
}

// RepositoryStatusSummaryAttributes represents status summary attributes.
type RepositoryStatusSummaryAttributes struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RepositoryStatusSummaryData represents status summary data in JSON:API format.
type RepositoryStatusSummaryData struct {
	Type       string                            `json:"type"`
	ID         string                            `json:"id"`
	Attributes RepositoryStatusSummaryAttributes `json:"attributes"`
}

// RepositoryStatusSummaryResponse represents a repository status summary response.
type RepositoryStatusSummaryResponse struct {
	Data RepositoryStatusSummaryData `json:"data"`
}

// TrackingConfigAttributes represents tracking configuration attributes in JSON:API format.
type TrackingConfigAttributes struct {
	Type   string `json:"type"`
	Branch string `json:"branch,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Commit string `json:"commit,omitempty"`
}

// TrackingConfigData represents tracking configuration data in JSON:API format.
type TrackingConfigData struct {
	Type       string                   `json:"type"`
	ID         string                   `json:"id"`
	Attributes TrackingConfigAttributes `json:"attributes"`
}

// TrackingConfigResponse represents a tracking configuration response in JSON:API format.
type TrackingConfigResponse struct {
	Data TrackingConfigData `json:"data"`
}

// TrackingConfigRequest represents a request to update tracking configuration.
type TrackingConfigRequest struct {
	Branch string `json:"branch,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Commit string `json:"commit,omitempty"`
}
