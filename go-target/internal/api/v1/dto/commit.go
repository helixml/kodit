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
