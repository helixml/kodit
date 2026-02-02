package dto

import "time"

// EnrichmentResponse represents an enrichment in API responses.
type EnrichmentResponse struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Subtype   string    `json:"subtype"`
	Content   string    `json:"content"`
	Language  string    `json:"language,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EnrichmentListResponse represents a list of enrichments.
type EnrichmentListResponse struct {
	Data       []EnrichmentResponse `json:"data"`
	TotalCount int                  `json:"total_count"`
}

// EnrichmentFilterRequest represents filters for enrichment queries.
type EnrichmentFilterRequest struct {
	Type       string   `json:"type,omitempty"`
	Subtype    string   `json:"subtype,omitempty"`
	SnippetIDs []string `json:"snippet_ids,omitempty"`
	CommitSHAs []string `json:"commit_shas,omitempty"`
}
