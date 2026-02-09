package kodit

import (
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/snippet"
)

// SearchResult represents the result of a hybrid search.
type SearchResult struct {
	snippets    []snippet.Snippet
	enrichments []enrichment.Enrichment
	scores      map[string]float64
}

// Snippets returns the matched code snippets.
func (r SearchResult) Snippets() []snippet.Snippet {
	result := make([]snippet.Snippet, len(r.snippets))
	copy(result, r.snippets)
	return result
}

// Enrichments returns the enrichments associated with matched snippets.
func (r SearchResult) Enrichments() []enrichment.Enrichment {
	result := make([]enrichment.Enrichment, len(r.enrichments))
	copy(result, r.enrichments)
	return result
}

// Scores returns a map of snippet SHA to fused search score.
func (r SearchResult) Scores() map[string]float64 {
	result := make(map[string]float64, len(r.scores))
	for k, v := range r.scores {
		result[k] = v
	}
	return result
}

// Count returns the number of snippets in the result.
func (r SearchResult) Count() int {
	return len(r.snippets)
}
