package dto

import (
	"time"

	"github.com/helixml/kodit/infrastructure/api/jsonapi"
)

// TaskAttributes represents task attributes in JSON:API format.
type TaskAttributes struct {
	Type      string     `json:"type"`
	Priority  int        `json:"priority"`
	Payload   any        `json:"payload"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// TaskData represents task data in JSON:API format.
type TaskData struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Attributes TaskAttributes `json:"attributes"`
}

// TaskResponse represents a single task response in JSON:API format.
type TaskResponse struct {
	Data TaskData `json:"data"`
}

// TaskListResponse represents a list of tasks in JSON:API format.
type TaskListResponse struct {
	Data  []TaskData     `json:"data"`
	Meta  *jsonapi.Meta  `json:"meta,omitempty"`
	Links *jsonapi.Links `json:"links,omitempty"`
}

// Legacy types for backwards compatibility

// LegacyTaskResponse represents a legacy task in API responses.
// Deprecated: Use TaskResponse for JSON:API compliance.
type LegacyTaskResponse struct {
	ID        int64     `json:"id"`
	DedupKey  string    `json:"dedup_key"`
	Operation string    `json:"operation"`
	Priority  int       `json:"priority"`
	Payload   any       `json:"payload,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LegacyTaskListResponse represents a legacy list of tasks.
// Deprecated: Use TaskListResponse for JSON:API compliance.
type LegacyTaskListResponse struct {
	Data       []LegacyTaskResponse `json:"data"`
	TotalCount int                  `json:"total_count"`
}

// TaskStatusResponse represents a task status in API responses.
type TaskStatusResponse struct {
	ID           int64      `json:"id"`
	TaskID       int64      `json:"task_id"`
	ParentID     int64      `json:"parent_id,omitempty"`
	State        string     `json:"state"`
	ErrorMessage string     `json:"error_message,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
