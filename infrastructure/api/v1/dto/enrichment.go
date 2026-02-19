package dto

import (
	"time"

	"github.com/helixml/kodit/infrastructure/api/jsonapi"
)

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

// EnrichmentAttributes represents enrichment attributes in JSON:API format.
type EnrichmentAttributes struct {
	Type      string    `json:"type"`
	Subtype   string    `json:"subtype"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EnrichmentData represents enrichment data in JSON:API format.
type EnrichmentData struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Attributes EnrichmentAttributes `json:"attributes"`
}

// EnrichmentJSONAPIResponse represents a single enrichment in JSON:API format.
type EnrichmentJSONAPIResponse struct {
	Data EnrichmentData `json:"data"`
}

// EnrichmentJSONAPIListResponse represents a list of enrichments in JSON:API format.
type EnrichmentJSONAPIListResponse struct {
	Data  []EnrichmentData `json:"data"`
	Meta  *jsonapi.Meta    `json:"meta,omitempty"`
	Links *jsonapi.Links   `json:"links,omitempty"`
}

// EnrichmentUpdateAttributes represents the attributes that can be updated.
type EnrichmentUpdateAttributes struct {
	Content string `json:"content"`
}

// EnrichmentUpdateData represents the data for updating an enrichment.
type EnrichmentUpdateData struct {
	Type       string                     `json:"type"`
	Attributes EnrichmentUpdateAttributes `json:"attributes"`
}

// EnrichmentUpdateRequest represents a JSON:API request to update an enrichment.
type EnrichmentUpdateRequest struct {
	Data EnrichmentUpdateData `json:"data"`
}
