// Package service provides application layer services that orchestrate domain operations.
package service

import (
	"context"
	"log/slog"
	"maps"
	"sync/atomic"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/internal/config"
)

// SearchOption configures a search request.
type SearchOption func(*searchConfig)

// searchConfig holds search parameters.
type searchConfig struct {
	semanticWeight   float64
	limit            int
	offset           int
	languages        []string
	repositories     []int64
	enrichmentTypes  []string
	minScore         float64
	includeSnippets  bool
	includeDocuments bool
}

// newSearchConfig creates a searchConfig with defaults.
func newSearchConfig() *searchConfig {
	return &searchConfig{
		limit: config.DefaultSearchLimit,
	}
}

// WithSemanticWeight sets the weight for semantic (vector) search (0-1).
func WithSemanticWeight(w float64) SearchOption {
	return func(c *searchConfig) {
		if w >= 0 && w <= 1 {
			c.semanticWeight = w
		}
	}
}

// WithLimit sets the maximum number of results.
func WithLimit(n int) SearchOption {
	return func(c *searchConfig) {
		if n > 0 {
			c.limit = n
		}
	}
}

// WithOffset sets the offset for pagination.
func WithOffset(n int) SearchOption {
	return func(c *searchConfig) {
		if n >= 0 {
			c.offset = n
		}
	}
}

// WithLanguages filters results by programming languages.
func WithLanguages(langs ...string) SearchOption {
	return func(c *searchConfig) {
		c.languages = langs
	}
}

// WithRepositories filters results by repository IDs.
func WithRepositories(ids ...int64) SearchOption {
	return func(c *searchConfig) {
		c.repositories = ids
	}
}

// WithEnrichmentTypes includes specific enrichment types in results.
func WithEnrichmentTypes(types ...string) SearchOption {
	return func(c *searchConfig) {
		c.enrichmentTypes = types
	}
}

// WithMinScore filters results below a minimum score threshold.
func WithMinScore(score float64) SearchOption {
	return func(c *searchConfig) {
		if score >= 0 {
			c.minScore = score
		}
	}
}

// WithSnippets includes code snippets in search results.
func WithSnippets(include bool) SearchOption {
	return func(c *searchConfig) {
		c.includeSnippets = include
	}
}

// WithDocuments includes enrichment documents in search results.
func WithDocuments(include bool) SearchOption {
	return func(c *searchConfig) {
		c.includeDocuments = include
	}
}

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

// Search orchestrates hybrid code search across text and code vector indexes.
type Search struct {
	textVectorStore search.VectorStore
	codeVectorStore search.VectorStore
	bm25Store       search.BM25Store
	snippetStore    snippet.SnippetStore
	enrichmentStore enrichment.EnrichmentStore
	fusion          search.Fusion
	closed          *atomic.Bool
	logger          *slog.Logger
}

// NewSearch creates a new Search service.
func NewSearch(
	textVectorStore search.VectorStore,
	codeVectorStore search.VectorStore,
	bm25Store search.BM25Store,
	snippetStore snippet.SnippetStore,
	enrichmentStore enrichment.EnrichmentStore,
	closed *atomic.Bool,
	logger *slog.Logger,
) *Search {
	if logger == nil {
		logger = slog.Default()
	}
	return &Search{
		textVectorStore: textVectorStore,
		codeVectorStore: codeVectorStore,
		bm25Store:       bm25Store,
		snippetStore:    snippetStore,
		enrichmentStore: enrichmentStore,
		fusion:          search.NewFusion(),
		closed:          closed,
		logger:          logger,
	}
}

// Available reports whether code search is configured.
func (s Search) Available() bool {
	return s.textVectorStore != nil || s.codeVectorStore != nil || s.bm25Store != nil
}

// Query performs a simple hybrid code search with options.
func (s Search) Query(ctx context.Context, query string, opts ...SearchOption) (SearchResult, error) {
	if s.closed != nil && s.closed.Load() {
		return SearchResult{}, ErrClientClosed
	}

	searchCfg := newSearchConfig()
	for _, opt := range opts {
		opt(searchCfg)
	}

	var filterOpts []search.FiltersOption
	if len(searchCfg.languages) > 0 && len(searchCfg.languages) == 1 {
		filterOpts = append(filterOpts, search.WithLanguage(searchCfg.languages[0]))
	}
	filters := search.NewFilters(filterOpts...)

	request := search.NewMultiRequest(searchCfg.limit, query, query, nil, filters)

	result, err := s.Search(ctx, request)
	if err != nil {
		return SearchResult{}, err
	}

	return SearchResult{
		snippets:    result.Snippets(),
		enrichments: result.Enrichments(),
		scores:      result.FusedScores(),
	}, nil
}

// Search performs a hybrid search combining text and code vector search results.
func (s Search) Search(ctx context.Context, request search.MultiRequest) (MultiSearchResult, error) {
	textQuery := request.TextQuery()
	codeQuery := request.CodeQuery()
	topK := request.TopK()

	if topK <= 0 {
		topK = 10
	}

	var fusionLists [][]search.FusionRequest

	// Run text vector search if text query provided and store is available
	if textQuery != "" && s.textVectorStore != nil {
		searchReq := search.NewRequest(textQuery, topK*2, nil)
		results, err := s.textVectorStore.Search(ctx, searchReq)
		if err != nil {
			s.logger.Warn("text vector search failed", "error", err)
		} else if len(results) > 0 {
			fusionLists = append(fusionLists, toFusionRequests(results))
		}
	}

	// Run code vector search if code query provided and store is available
	if codeQuery != "" && s.codeVectorStore != nil {
		searchReq := search.NewRequest(codeQuery, topK*2, nil)
		results, err := s.codeVectorStore.Search(ctx, searchReq)
		if err != nil {
			s.logger.Warn("code vector search failed", "error", err)
		} else if len(results) > 0 {
			fusionLists = append(fusionLists, toFusionRequests(results))
		}
	}

	// Run BM25 keyword search per-keyword if keywords provided and store is available
	keywords := request.Keywords()
	if len(keywords) > 0 && s.bm25Store != nil {
		var bm25Fusion []search.FusionRequest
		for _, keyword := range keywords {
			bm25Req := search.NewRequest(keyword, topK*2, nil)
			results, err := s.bm25Store.Search(ctx, bm25Req)
			if err != nil {
				s.logger.Warn("bm25 keyword search failed", "keyword", keyword, "error", err)
				continue
			}
			bm25Fusion = append(bm25Fusion, toFusionRequests(results)...)
		}
		if len(bm25Fusion) > 0 {
			fusionLists = append(fusionLists, bm25Fusion)
		}
	}

	if len(fusionLists) == 0 {
		return NewMultiSearchResult(nil, nil, nil), nil
	}

	// Fuse all result lists together
	fusedResults := s.fusion.FuseTopK(topK, fusionLists...)

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
func (s Search) SearchText(ctx context.Context, query string, topK int) ([]snippet.Snippet, error) {
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
func (s Search) SearchCode(ctx context.Context, query string, topK int) ([]snippet.Snippet, error) {
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
func (s Search) fetchEnrichments(_ context.Context, _ []string) ([]enrichment.Enrichment, error) {
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
