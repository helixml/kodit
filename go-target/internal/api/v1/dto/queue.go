package dto

import "time"

// TaskResponse represents a task in API responses.
type TaskResponse struct {
	ID        int64     `json:"id"`
	DedupKey  string    `json:"dedup_key"`
	Operation string    `json:"operation"`
	Priority  int       `json:"priority"`
	Payload   any       `json:"payload,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskListResponse represents a list of tasks.
type TaskListResponse struct {
	Data       []TaskResponse `json:"data"`
	TotalCount int            `json:"total_count"`
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

// QueueStatsResponse represents queue statistics.
type QueueStatsResponse struct {
	PendingCount    int `json:"pending_count"`
	InProgressCount int `json:"in_progress_count"`
	CompletedCount  int `json:"completed_count"`
	FailedCount     int `json:"failed_count"`
}
