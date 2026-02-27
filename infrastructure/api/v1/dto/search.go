package dto

import (
	"time"

	"github.com/helixml/kodit/infrastructure/api/jsonapi"
)

// SearchFilters represents search filters in JSON:API format.
type SearchFilters struct {
	Languages          []string   `json:"languages,omitempty"`
	Authors            []string   `json:"authors,omitempty"`
	StartDate          *time.Time `json:"start_date,omitempty"`
	EndDate            *time.Time `json:"end_date,omitempty"`
	Sources            []string   `json:"sources,omitempty"`
	FilePatterns       []string   `json:"file_patterns,omitempty"`
	EnrichmentTypes    []string   `json:"enrichment_types,omitempty"`
	EnrichmentSubtypes []string   `json:"enrichment_subtypes,omitempty"`
	CommitSHA          []string   `json:"commit_sha,omitempty"`
}

// SearchAttributes represents search request attributes in JSON:API format.
type SearchAttributes struct {
	Keywords []string       `json:"keywords,omitempty"`
	Code     *string        `json:"code,omitempty"`
	Text     *string        `json:"text,omitempty"`
	Limit    *int           `json:"limit,omitempty"`
	Filters  *SearchFilters `json:"filters,omitempty"`
}

// SearchData represents search request data in JSON:API format.
type SearchData struct {
	Type       string           `json:"type"`
	Attributes SearchAttributes `json:"attributes"`
}

// SearchRequest represents a JSON:API search request.
type SearchRequest struct {
	Data SearchData `json:"data"`
}

// GitFileSchema represents a git file reference in search results.
type GitFileSchema struct {
	BlobSHA  string `json:"blob_sha"`
	Path     string `json:"path"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// SnippetContentSchema represents snippet content in search results.
type SnippetContentSchema struct {
	Value     string `json:"value"`
	Language  string `json:"language"`
	StartLine *int   `json:"start_line,omitempty"`
	EndLine   *int   `json:"end_line,omitempty"`
}

// EnrichmentSchema represents an enrichment in search results.
type EnrichmentSchema struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// SnippetAttributes represents snippet attributes in search results.
type SnippetAttributes struct {
	CreatedAt      *time.Time           `json:"created_at,omitempty"`
	UpdatedAt      *time.Time           `json:"updated_at,omitempty"`
	DerivesFrom    []GitFileSchema      `json:"derives_from"`
	Content        SnippetContentSchema `json:"content"`
	Enrichments    []EnrichmentSchema   `json:"enrichments"`
	OriginalScores []float64            `json:"original_scores"`
}

// SnippetLinks holds API path links for a search result snippet.
type SnippetLinks struct {
	Repository string `json:"repository,omitempty"`
	Commit     string `json:"commit,omitempty"`
	File       string `json:"file,omitempty"`
}

// SnippetData represents snippet data in JSON:API format.
type SnippetData struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Attributes SnippetAttributes `json:"attributes"`
	Links      *SnippetLinks     `json:"links,omitempty"`
}

// SearchResponse represents a search API response in JSON:API format.
type SearchResponse struct {
	Data []SnippetData `json:"data"`
}

// SnippetListResponse represents a list of snippets in JSON:API format.
type SnippetListResponse struct {
	Data  []SnippetData  `json:"data"`
	Meta  *jsonapi.Meta  `json:"meta,omitempty"`
	Links *jsonapi.Links `json:"links,omitempty"`
}

// Legacy types for backwards compatibility during migration

// LegacySearchRequest represents a legacy search API request (flat format).
// Deprecated: Use SearchRequest for JSON:API compliance.
type LegacySearchRequest struct {
	Query      string   `json:"query"`
	TextQuery  string   `json:"text_query,omitempty"`
	CodeQuery  string   `json:"code_query,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`
	TopK       int      `json:"top_k,omitempty"`
	Language   string   `json:"language,omitempty"`
	Author     string   `json:"author,omitempty"`
	SourceRepo string   `json:"source_repo,omitempty"`
	FilePath   string   `json:"file_path,omitempty"`
	CommitSHAs []string `json:"commit_shas,omitempty"`
}

// LegacySearchResultResponse represents a legacy single search result.
// Deprecated: Use SnippetData for JSON:API compliance.
type LegacySearchResultResponse struct {
	SnippetSHA  string                     `json:"snippet_sha"`
	Content     string                     `json:"content"`
	Extension   string                     `json:"extension"`
	Score       float64                    `json:"score"`
	FilePath    string                     `json:"file_path,omitempty"`
	Language    string                     `json:"language,omitempty"`
	Enrichments []LegacyEnrichmentResponse `json:"enrichments,omitempty"`
}

// LegacyEnrichmentResponse is a legacy enrichment response type.
type LegacyEnrichmentResponse struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Subtype   string    `json:"subtype"`
	Content   string    `json:"content"`
	Language  string    `json:"language,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LegacySearchResponse represents a legacy search API response.
// Deprecated: Use SearchResponse for JSON:API compliance.
type LegacySearchResponse struct {
	Results    []LegacySearchResultResponse `json:"results"`
	TotalCount int                          `json:"total_count"`
	Query      string                       `json:"query"`
}
