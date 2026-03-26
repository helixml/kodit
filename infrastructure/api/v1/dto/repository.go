// Package dto provides data transfer objects for the API layer.
package dto

import (
	"time"

	"github.com/helixml/kodit/infrastructure/api/jsonapi"
)

// RepositoryAttributes represents repository attributes in JSON:API format.
type RepositoryAttributes struct {
	RemoteURI      string     `json:"remote_uri"`
	UpstreamURL    string     `json:"upstream_url"` // The canonical upstream URL (e.g. github.com/org/repo); falls back to remote_uri when not set
	PipelineID     int64      `json:"pipeline_id"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	LastScannedAt  *time.Time `json:"last_scanned_at,omitempty"`
	ClonedPath     *string    `json:"cloned_path,omitempty"`
	TrackingBranch *string    `json:"tracking_branch,omitempty"`
	NumCommits     int        `json:"num_commits"`
	NumBranches    int        `json:"num_branches"`
	NumTags        int        `json:"num_tags"`
}

// RepositoryLinks holds links for a repository resource.
type RepositoryLinks struct {
	Pipeline *string `json:"pipeline,omitempty"`
}

// RepositoryData represents repository data in JSON:API format.
type RepositoryData struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Attributes RepositoryAttributes `json:"attributes"`
	Links      *RepositoryLinks     `json:"links,omitempty"`
}

// RepositoryResponse represents a single repository response in JSON:API format.
type RepositoryResponse struct {
	Data RepositoryData `json:"data"`
}

// RepositoryListResponse represents a list of repositories in JSON:API format.
type RepositoryListResponse struct {
	Data  []RepositoryData `json:"data"`
	Meta  *jsonapi.Meta    `json:"meta,omitempty"`
	Links *jsonapi.Links   `json:"links,omitempty"`
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
	RemoteURI   string `json:"remote_uri"`
	UpstreamURL string `json:"upstream_url,omitempty"` // Optional canonical upstream URL; used for deduplication when multiple clone URLs point to the same repo
	Pipeline    string `json:"pipeline,omitempty"`     // Optional pipeline name; looked up by name and assigned to the repository (defaults to the system default pipeline)
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
	Data  []TaskStatusData `json:"data"`
	Meta  *jsonapi.Meta    `json:"meta,omitempty"`
	Links *jsonapi.Links   `json:"links,omitempty"`
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

// ChunkingConfigAttributes represents chunking configuration attributes in JSON:API format.
type ChunkingConfigAttributes struct {
	ChunkSize    int `json:"chunk_size"`
	ChunkOverlap int `json:"chunk_overlap"`
	MinChunkSize int `json:"min_chunk_size"`
}

// ChunkingConfigData represents chunking configuration data in JSON:API format.
type ChunkingConfigData struct {
	Type       string                   `json:"type"`
	Attributes ChunkingConfigAttributes `json:"attributes"`
}

// ChunkingConfigResponse represents a chunking configuration response in JSON:API format.
type ChunkingConfigResponse struct {
	Data ChunkingConfigData `json:"data"`
}

// ChunkingConfigUpdateData represents chunking config update data.
type ChunkingConfigUpdateData struct {
	Type       string                   `json:"type"`
	Attributes ChunkingConfigAttributes `json:"attributes"`
}

// ChunkingConfigUpdateRequest represents a chunking config update request.
type ChunkingConfigUpdateRequest struct {
	Data ChunkingConfigUpdateData `json:"data"`
}

// PipelineConfigAttributes represents pipeline configuration attributes in JSON:API format.
type PipelineConfigAttributes struct {
	PipelineID int64 `json:"pipeline_id"`
}

// PipelineConfigLinks holds links for a pipeline config resource.
type PipelineConfigLinks struct {
	Pipeline string `json:"pipeline"`
}

// PipelineConfigData represents pipeline configuration data in JSON:API format.
type PipelineConfigData struct {
	Type       string                   `json:"type"`
	Attributes PipelineConfigAttributes `json:"attributes"`
	Links      PipelineConfigLinks      `json:"links"`
}

// PipelineConfigResponse represents a pipeline configuration response in JSON:API format.
type PipelineConfigResponse struct {
	Data     PipelineConfigData `json:"data"`
	Included []PipelineData     `json:"included,omitempty"`
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
