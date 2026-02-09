// Package service provides application layer services that orchestrate domain operations.
package service

import (
	"context"
	"log/slog"
	"maps"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
)

// MultiSearchResult represents the result of a multi-modal search.
type MultiSearchResult struct {
	snippets    []snippet.Snippet
	enrichments []enrichment.Enrichment
	fusedScores map[string]float64
}

// NewMultiSearchResult creates a new MultiSearchResult.
func NewMultiSearchResult(
	snippets []snippet.Snippet,
	enrichments []enrichment.Enrichment,
	fusedScores map[string]float64,
) MultiSearchResult {
	snips := make([]snippet.Snippet, len(snippets))
	copy(snips, snippets)

	enrich := make([]enrichment.Enrichment, len(enrichments))
	copy(enrich, enrichments)

	scores := make(map[string]float64, len(fusedScores))
	maps.Copy(scores, fusedScores)

	return MultiSearchResult{
		snippets:    snips,
		enrichments: enrich,
		fusedScores: scores,
	}
}

// Snippets returns the matched snippets.
func (r MultiSearchResult) Snippets() []snippet.Snippet {
	result := make([]snippet.Snippet, len(r.snippets))
	copy(result, r.snippets)
	return result
}

// Enrichments returns the enrichments associated with matched snippets.
func (r MultiSearchResult) Enrichments() []enrichment.Enrichment {
	result := make([]enrichment.Enrichment, len(r.enrichments))
	copy(result, r.enrichments)
	return result
}

// FusedScores returns a map of snippet SHA to fused score.
func (r MultiSearchResult) FusedScores() map[string]float64 {
	result := make(map[string]float64, len(r.fusedScores))
	maps.Copy(result, r.fusedScores)
	return result
}

// Count returns the number of snippets in the result.
func (r MultiSearchResult) Count() int {
	return len(r.snippets)
}

// CodeSearch orchestrates hybrid code search across text and code vector indexes.
type CodeSearch struct {
	textVectorStore search.VectorStore
	codeVectorStore search.VectorStore
	snippetStore    snippet.SnippetStore
	enrichmentStore enrichment.EnrichmentStore
	fusion          search.Fusion
	logger          *slog.Logger
}

// NewCodeSearch creates a new code search service.
func NewCodeSearch(
	textVectorStore search.VectorStore,
	codeVectorStore search.VectorStore,
	snippetStore snippet.SnippetStore,
	enrichmentStore enrichment.EnrichmentStore,
	logger *slog.Logger,
) *CodeSearch {
	if logger == nil {
		logger = slog.Default()
	}
	return &CodeSearch{
		textVectorStore: textVectorStore,
		codeVectorStore: codeVectorStore,
		snippetStore:    snippetStore,
		enrichmentStore: enrichmentStore,
		fusion:          search.NewFusion(),
		logger:          logger,
	}
}

// Search performs a hybrid search combining text and code vector search results.
func (s CodeSearch) Search(ctx context.Context, request search.MultiRequest) (MultiSearchResult, error) {
	textQuery := request.TextQuery()
	codeQuery := request.CodeQuery()
	topK := request.TopK()

	if topK <= 0 {
		topK = 10
	}

	var textResults, codeResults []search.Result

	// Run text vector search if text query provided and store is available
	// This searches against enrichment summary embeddings
	if textQuery != "" && s.textVectorStore != nil {
		searchReq := search.NewRequest(textQuery, topK*2, nil) // Get more for fusion
		results, err := s.textVectorStore.Search(ctx, searchReq)
		if err != nil {
			s.logger.Warn("text vector search failed", "error", err)
		} else {
			textResults = results
		}
	}

	// Run code vector search if code query provided and store is available
	// This searches against code snippet embeddings
	if codeQuery != "" && s.codeVectorStore != nil {
		searchReq := search.NewRequest(codeQuery, topK*2, nil)
		results, err := s.codeVectorStore.Search(ctx, searchReq)
		if err != nil {
			s.logger.Warn("code vector search failed", "error", err)
		} else {
			codeResults = results
		}
	}

	// If both queries are empty, return empty result
	if textQuery == "" && codeQuery == "" {
		return NewMultiSearchResult(nil, nil, nil), nil
	}

	// Convert to fusion requests
	textFusion := toFusionRequests(textResults)
	codeFusion := toFusionRequests(codeResults)

	// Fuse results
	var fusedResults []search.FusionResult
	if len(textFusion) > 0 && len(codeFusion) > 0 {
		fusedResults = s.fusion.FuseTopK(topK, textFusion, codeFusion)
	} else if len(textFusion) > 0 {
		fusedResults = s.fusion.FuseTopK(topK, textFusion)
	} else if len(codeFusion) > 0 {
		fusedResults = s.fusion.FuseTopK(topK, codeFusion)
	}

	// Extract snippet IDs from fused results
	snippetIDs := make([]string, len(fusedResults))
	fusedScores := make(map[string]float64, len(fusedResults))
	for i, result := range fusedResults {
		snippetIDs[i] = result.ID()
		fusedScores[result.ID()] = result.Score()
	}

	if len(snippetIDs) == 0 {
		return NewMultiSearchResult(nil, nil, nil), nil
	}

	// Fetch full snippets
	snippets, err := s.snippetStore.ByIDs(ctx, snippetIDs)
	if err != nil {
		return MultiSearchResult{}, err
	}

	// Order snippets by fused score
	orderedSnippets := orderByScore(snippets, fusedScores)

	// Fetch associated enrichments
	enrichments, err := s.fetchEnrichments(ctx, snippetIDs)
	if err != nil {
		s.logger.Warn("failed to fetch enrichments", "error", err)
		enrichments = nil
	}

	return NewMultiSearchResult(orderedSnippets, enrichments, fusedScores), nil
}

// SearchText performs text vector search against enrichment summaries.
func (s CodeSearch) SearchText(ctx context.Context, query string, topK int) ([]snippet.Snippet, error) {
	if topK <= 0 {
		topK = 10
	}

	searchReq := search.NewRequest(query, topK, nil)
	results, err := s.textVectorStore.Search(ctx, searchReq)
	if err != nil {
		return nil, err
	}

	snippetIDs := make([]string, len(results))
	for i, r := range results {
		snippetIDs[i] = r.SnippetID()
	}

	return s.snippetStore.ByIDs(ctx, snippetIDs)
}

// SearchCode performs code vector search against code snippet embeddings.
func (s CodeSearch) SearchCode(ctx context.Context, query string, topK int) ([]snippet.Snippet, error) {
	if topK <= 0 {
		topK = 10
	}

	searchReq := search.NewRequest(query, topK, nil)
	results, err := s.codeVectorStore.Search(ctx, searchReq)
	if err != nil {
		return nil, err
	}

	snippetIDs := make([]string, len(results))
	for i, r := range results {
		snippetIDs[i] = r.SnippetID()
	}

	return s.snippetStore.ByIDs(ctx, snippetIDs)
}

// fetchEnrichments retrieves enrichments for the given snippet IDs.
func (s CodeSearch) fetchEnrichments(_ context.Context, _ []string) ([]enrichment.Enrichment, error) {
	// For now, return empty - full implementation requires EnrichmentAssociationStore
	return nil, nil
}

// toFusionRequests converts search results to fusion requests.
func toFusionRequests(results []search.Result) []search.FusionRequest {
	requests := make([]search.FusionRequest, len(results))
	for i, r := range results {
		requests[i] = search.NewFusionRequest(r.SnippetID(), r.Score())
	}
	return requests
}

// orderByScore orders snippets by their fused scores.
func orderByScore(snippets []snippet.Snippet, scores map[string]float64) []snippet.Snippet {
	type scored struct {
		snippet snippet.Snippet
		score   float64
	}

	scoredSnippets := make([]scored, 0, len(snippets))
	for _, s := range snippets {
		score := scores[s.SHA()]
		scoredSnippets = append(scoredSnippets, scored{snippet: s, score: score})
	}

	// Sort by score descending
	for i := 0; i < len(scoredSnippets)-1; i++ {
		for j := i + 1; j < len(scoredSnippets); j++ {
			if scoredSnippets[j].score > scoredSnippets[i].score {
				scoredSnippets[i], scoredSnippets[j] = scoredSnippets[j], scoredSnippets[i]
			}
		}
	}

	result := make([]snippet.Snippet, len(scoredSnippets))
	for i, s := range scoredSnippets {
		result[i] = s.snippet
	}

	return result
}
