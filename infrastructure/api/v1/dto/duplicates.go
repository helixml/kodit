package dto

// DuplicateSearchAttributes holds the attributes of a duplicate search request.
type DuplicateSearchAttributes struct {
	RepositoryIDs []int64  `json:"repository_ids"`
	Threshold     *float64 `json:"threshold,omitempty"`
	Limit         *int     `json:"limit,omitempty"`
}

// DuplicateSearchData is the data envelope of a duplicate search request.
type DuplicateSearchData struct {
	Type       string                    `json:"type"`
	Attributes DuplicateSearchAttributes `json:"attributes"`
}

// DuplicateSearchRequest is the JSON:API-style request body for POST /search/duplicates.
type DuplicateSearchRequest struct {
	Data DuplicateSearchData `json:"data"`
}

// DuplicateSnippetSchema represents one snippet in a duplicate pair.
type DuplicateSnippetSchema struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Language string `json:"language"`
}

// DuplicatePairAttributes holds the attributes of a single duplicate pair.
type DuplicatePairAttributes struct {
	Similarity float64                `json:"similarity"`
	SnippetA   DuplicateSnippetSchema `json:"snippet_a"`
	SnippetB   DuplicateSnippetSchema `json:"snippet_b"`
}

// DuplicatePairData is a single duplicate pair in JSON:API format.
type DuplicatePairData struct {
	Type       string                  `json:"type"`
	Attributes DuplicatePairAttributes `json:"attributes"`
}

// DuplicatesMeta holds optional metadata for the duplicates response.
type DuplicatesMeta struct {
	Truncated bool `json:"truncated,omitempty"`
}

// DuplicatesResponse is the JSON:API-style response body for POST /search/duplicates.
type DuplicatesResponse struct {
	Data []DuplicatePairData `json:"data"`
	Meta *DuplicatesMeta     `json:"meta,omitempty"`
}
