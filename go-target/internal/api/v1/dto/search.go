package dto

import "time"

// SearchRequest represents a search API request.
type SearchRequest struct {
	Query        string   `json:"query"`
	TextQuery    string   `json:"text_query,omitempty"`
	CodeQuery    string   `json:"code_query,omitempty"`
	Keywords     []string `json:"keywords,omitempty"`
	TopK         int      `json:"top_k,omitempty"`
	Language     string   `json:"language,omitempty"`
	Author       string   `json:"author,omitempty"`
	SourceRepo   string   `json:"source_repo,omitempty"`
	FilePath     string   `json:"file_path,omitempty"`
	CommitSHAs   []string `json:"commit_shas,omitempty"`
}

// SearchResultResponse represents a single search result.
type SearchResultResponse struct {
	SnippetSHA  string                `json:"snippet_sha"`
	Content     string                `json:"content"`
	Extension   string                `json:"extension"`
	Score       float64               `json:"score"`
	FilePath    string                `json:"file_path,omitempty"`
	Language    string                `json:"language,omitempty"`
	Enrichments []EnrichmentResponse  `json:"enrichments,omitempty"`
}

// SearchResponse represents a search API response.
type SearchResponse struct {
	Results    []SearchResultResponse `json:"results"`
	TotalCount int                    `json:"total_count"`
	Query      string                 `json:"query"`
}

// SnippetResponse represents a snippet in API responses.
type SnippetResponse struct {
	SHA         string                `json:"sha"`
	Content     string                `json:"content"`
	Extension   string                `json:"extension"`
	FilePaths   []string              `json:"file_paths,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	Enrichments []EnrichmentResponse  `json:"enrichments,omitempty"`
}
