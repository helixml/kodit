package dto

// EmbeddingAttributes represents embedding attributes in JSON:API format.
type EmbeddingAttributes struct {
	SnippetSHA    string    `json:"snippet_sha"`
	EmbeddingType string    `json:"embedding_type"`
	Embedding     []float64 `json:"embedding"`
}

// EmbeddingData represents embedding data in JSON:API format.
type EmbeddingData struct {
	Type       string              `json:"type"`
	ID         string              `json:"id"`
	Attributes EmbeddingAttributes `json:"attributes"`
}

// EmbeddingJSONAPIListResponse represents a list of embeddings in JSON:API format.
type EmbeddingJSONAPIListResponse struct {
	Data []EmbeddingData `json:"data"`
}
