// Package dto provides data transfer objects for the API layer.
package dto

import "time"

// RepositoryAttributes represents repository attributes in JSON:API format.
type RepositoryAttributes struct {
	RemoteURI      string     `json:"remote_uri"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	LastScannedAt  *time.Time `json:"last_scanned_at,omitempty"`
	ClonedPath     *string    `json:"cloned_path,omitempty"`
	TrackingBranch *string    `json:"tracking_branch,omitempty"`
	NumCommits     int        `json:"num_commits"`
	NumBranches    int        `json:"num_branches"`
	NumTags        int        `json:"num_tags"`
}

// RepositoryData represents repository data in JSON:API format.
type RepositoryData struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Attributes RepositoryAttributes `json:"attributes"`
}

// RepositoryResponse represents a single repository response in JSON:API format.
type RepositoryResponse struct {
	Data RepositoryData `json:"data"`
}

// RepositoryListResponse represents a list of repositories in JSON:API format.
type RepositoryListResponse struct {
	Data []RepositoryData `json:"data"`
}

// RepositoryBranchData represents branch data for repository details.
type RepositoryBranchData struct {
	Name        string `json:"name"`
	IsDefault   bool   `json:"is_default"`
	CommitCount int    `json:"commit_count"`
}

// RepositoryCommitData represents commit data for repository details.
type RepositoryCommitData struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

// RepositoryDetailsResponse represents repository details with branches and commits.
type RepositoryDetailsResponse struct {
	Data          RepositoryData         `json:"data"`
	Branches      []RepositoryBranchData `json:"branches"`
	RecentCommits []RepositoryCommitData `json:"recent_commits"`
}

// RepositoryCreateAttributes represents repository creation attributes.
type RepositoryCreateAttributes struct {
	RemoteURI string `json:"remote_uri"`
}

// RepositoryCreateData represents repository creation data.
type RepositoryCreateData struct {
	Type       string                     `json:"type"`
	Attributes RepositoryCreateAttributes `json:"attributes"`
}

// RepositoryCreateRequest represents a repository creation request in JSON:API format.
type RepositoryCreateRequest struct {
	Data RepositoryCreateData `json:"data"`
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

// TrackingMode represents the tracking mode (branch or tag).
type TrackingMode string

const (
	TrackingModeBranch TrackingMode = "branch"
	TrackingModeTag    TrackingMode = "tag"
)

// TrackingConfigAttributes represents tracking configuration attributes in JSON:API format.
type TrackingConfigAttributes struct {
	Mode  TrackingMode `json:"mode"`
	Value *string      `json:"value,omitempty"`
}

// TrackingConfigData represents tracking configuration data in JSON:API format.
type TrackingConfigData struct {
	Type       string                   `json:"type"`
	Attributes TrackingConfigAttributes `json:"attributes"`
}

// TrackingConfigResponse represents a tracking configuration response in JSON:API format.
type TrackingConfigResponse struct {
	Data TrackingConfigData `json:"data"`
}

// TrackingConfigUpdateAttributes represents tracking config update attributes.
type TrackingConfigUpdateAttributes struct {
	Mode  TrackingMode `json:"mode"`
	Value *string      `json:"value,omitempty"`
}

// TrackingConfigUpdateData represents tracking config update data.
type TrackingConfigUpdateData struct {
	Type       string                         `json:"type"`
	Attributes TrackingConfigUpdateAttributes `json:"attributes"`
}

// TrackingConfigUpdateRequest represents a tracking config update request.
type TrackingConfigUpdateRequest struct {
	Data TrackingConfigUpdateData `json:"data"`
}

// Legacy types for backwards compatibility during migration

// RepositoryRequest represents a legacy request to add a repository.
// Deprecated: Use RepositoryCreateRequest for JSON:API compliance.
type RepositoryRequest struct {
	RemoteURL string `json:"remote_url"`
	Branch    string `json:"branch,omitempty"`
	Tag       string `json:"tag,omitempty"`
	Commit    string `json:"commit,omitempty"`
}

// TrackingConfigRequest represents a legacy request to update tracking configuration.
// Deprecated: Use TrackingConfigUpdateRequest for JSON:API compliance.
type TrackingConfigRequest struct {
	Branch string `json:"branch,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Commit string `json:"commit,omitempty"`
}
