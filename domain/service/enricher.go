package service

import "context"

// EnrichmentRequest represents an enrichment request with a custom system prompt.
type EnrichmentRequest struct {
	id           string
	text         string
	systemPrompt string
}

// NewEnrichmentRequest creates a new enrichment request.
func NewEnrichmentRequest(id, text, systemPrompt string) EnrichmentRequest {
	return EnrichmentRequest{
		id:           id,
		text:         text,
		systemPrompt: systemPrompt,
	}
}

// ID returns the request identifier.
func (r EnrichmentRequest) ID() string { return r.id }

// Text returns the text to be enriched.
func (r EnrichmentRequest) Text() string { return r.text }

// SystemPrompt returns the custom system prompt.
func (r EnrichmentRequest) SystemPrompt() string { return r.systemPrompt }

// EnrichmentResponse represents an enrichment response.
type EnrichmentResponse struct {
	id   string
	text string
}

// NewEnrichmentResponse creates a new enrichment response.
func NewEnrichmentResponse(id, text string) EnrichmentResponse {
	return EnrichmentResponse{id: id, text: text}
}

// ID returns the response identifier (matches the request ID).
func (r EnrichmentResponse) ID() string { return r.id }

// Text returns the enriched text.
func (r EnrichmentResponse) Text() string { return r.text }

// Enricher generates enrichments using an AI provider.
type Enricher interface {
	// Enrich processes requests and returns responses for each.
	Enrich(ctx context.Context, requests []EnrichmentRequest, opts ...EnrichOption) ([]EnrichmentResponse, error)
}
