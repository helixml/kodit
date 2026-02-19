// Package service provides application layer services that orchestrate domain operations.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"sync/atomic"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
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
	enrichments []enrichment.Enrichment
	scores      map[string]float64
}

// Enrichments returns the matched enrichments.
func (r SearchResult) Enrichments() []enrichment.Enrichment {
	result := make([]enrichment.Enrichment, len(r.enrichments))
	copy(result, r.enrichments)
	return result
}

// Scores returns a map of enrichment ID string to fused search score.
func (r SearchResult) Scores() map[string]float64 {
	result := make(map[string]float64, len(r.scores))
	for k, v := range r.scores {
		result[k] = v
	}
	return result
}

// Count returns the number of enrichments in the result.
func (r SearchResult) Count() int {
	return len(r.enrichments)
}

// MultiSearchResult represents the result of a multi-modal search.
type MultiSearchResult struct {
	enrichments    []enrichment.Enrichment
	fusedScores    map[string]float64
	originalScores map[string][]float64
}

// NewMultiSearchResult creates a new MultiSearchResult.
func NewMultiSearchResult(
	enrichments []enrichment.Enrichment,
	fusedScores map[string]float64,
	originalScores map[string][]float64,
) MultiSearchResult {
	enrich := make([]enrichment.Enrichment, len(enrichments))
	copy(enrich, enrichments)

	scores := make(map[string]float64, len(fusedScores))
	maps.Copy(scores, fusedScores)

	originals := make(map[string][]float64, len(originalScores))
	for k, v := range originalScores {
		cp := make([]float64, len(v))
		copy(cp, v)
		originals[k] = cp
	}

	return MultiSearchResult{
		enrichments:    enrich,
		fusedScores:    scores,
		originalScores: originals,
	}
}

// Enrichments returns the matched enrichments.
func (r MultiSearchResult) Enrichments() []enrichment.Enrichment {
	result := make([]enrichment.Enrichment, len(r.enrichments))
	copy(result, r.enrichments)
	return result
}

// FusedScores returns a map of enrichment ID string to fused score.
func (r MultiSearchResult) FusedScores() map[string]float64 {
	result := make(map[string]float64, len(r.fusedScores))
	maps.Copy(result, r.fusedScores)
	return result
}

// OriginalScores returns a map of enrichment ID string to raw scores from each search method.
func (r MultiSearchResult) OriginalScores() map[string][]float64 {
	result := make(map[string][]float64, len(r.originalScores))
	for k, v := range r.originalScores {
		cp := make([]float64, len(v))
		copy(cp, v)
		result[k] = cp
	}
	return result
}

// Count returns the number of enrichments in the result.
func (r MultiSearchResult) Count() int {
	return len(r.enrichments)
}

// Search orchestrates hybrid code search across text and code vector indexes.
type Search struct {
	embedder        search.Embedder
	textVectorStore search.EmbeddingStore
	codeVectorStore search.EmbeddingStore
	bm25Store       search.BM25Store
	enrichmentStore enrichment.EnrichmentStore
	fusion          search.Fusion
	closed          *atomic.Bool
	logger          *slog.Logger
}

// NewSearch creates a new Search service.
func NewSearch(
	embedder search.Embedder,
	textVectorStore search.EmbeddingStore,
	codeVectorStore search.EmbeddingStore,
	bm25Store search.BM25Store,
	enrichmentStore enrichment.EnrichmentStore,
	closed *atomic.Bool,
	logger *slog.Logger,
) *Search {
	if logger == nil {
		logger = slog.Default()
	}
	return &Search{
		embedder:        embedder,
		textVectorStore: textVectorStore,
		codeVectorStore: codeVectorStore,
		bm25Store:       bm25Store,
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
	if len(searchCfg.languages) > 0 {
		filterOpts = append(filterOpts, search.WithLanguages(searchCfg.languages))
	}
	filters := search.NewFilters(filterOpts...)

	request := search.NewMultiRequest(searchCfg.limit, query, query, nil, filters)

	result, err := s.Search(ctx, request)
	if err != nil {
		return SearchResult{}, err
	}

	return SearchResult{
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

	filterOpt := search.WithFilters(request.Filters())

	var fusionLists [][]search.FusionRequest

	// Embed queries for vector search
	var textEmbedding, codeEmbedding []float64
	if s.embedder != nil {
		var textsToEmbed []string
		var textIdx, codeIdx = -1, -1
		if textQuery != "" && s.textVectorStore != nil {
			textIdx = len(textsToEmbed)
			textsToEmbed = append(textsToEmbed, textQuery)
		}
		if codeQuery != "" && s.codeVectorStore != nil && codeQuery != textQuery {
			codeIdx = len(textsToEmbed)
			textsToEmbed = append(textsToEmbed, codeQuery)
		}

		if len(textsToEmbed) > 0 {
			embeddings, err := s.embedder.Embed(ctx, textsToEmbed)
			if err != nil {
				s.logger.Warn("embedding failed", "error", err)
			} else {
				if textIdx >= 0 && textIdx < len(embeddings) {
					textEmbedding = embeddings[textIdx]
				}
				if codeIdx >= 0 && codeIdx < len(embeddings) {
					codeEmbedding = embeddings[codeIdx]
				} else if codeQuery == textQuery && textEmbedding != nil {
					codeEmbedding = textEmbedding
				}
			}
		}
	}

	// Run text vector search if embedding available
	if len(textEmbedding) > 0 && s.textVectorStore != nil {
		results, err := s.textVectorStore.Search(ctx,
			filterOpt,
			search.WithEmbedding(textEmbedding),
			repository.WithLimit(topK*2),
		)
		if err != nil {
			s.logger.Warn("text vector search failed", "error", err)
		} else if len(results) > 0 {
			fusionLists = append(fusionLists, toFusionRequests(results))
		}
	}

	// Run code vector search if embedding available
	if len(codeEmbedding) > 0 && s.codeVectorStore != nil {
		results, err := s.codeVectorStore.Search(ctx,
			filterOpt,
			search.WithEmbedding(codeEmbedding),
			repository.WithLimit(topK*2),
		)
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
			results, err := s.bm25Store.Find(ctx,
				filterOpt,
				search.WithQuery(keyword),
				repository.WithLimit(topK*2),
			)
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

	// Extract enrichment IDs and scores from fused results
	fusedScores := make(map[string]float64, len(fusedResults))
	originalScores := make(map[string][]float64, len(fusedResults))
	ids := make([]int64, 0, len(fusedResults))
	for _, result := range fusedResults {
		fusedScores[result.ID()] = result.Score()
		originalScores[result.ID()] = result.OriginalScores()
		id, err := strconv.ParseInt(result.ID(), 10, 64)
		if err != nil {
			s.logger.Warn("failed to parse enrichment ID", "id", result.ID(), "error", err)
			continue
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return NewMultiSearchResult(nil, nil, nil), nil
	}

	// Fetch full enrichments — filtering already happened in store queries
	enrichments, err := s.enrichmentStore.Find(ctx, repository.WithIDIn(ids))
	if err != nil {
		return MultiSearchResult{}, fmt.Errorf("fetch enrichments: %w", err)
	}

	// Order enrichments by fused score
	ordered := orderByScore(enrichments, fusedScores)

	return NewMultiSearchResult(ordered, fusedScores, originalScores), nil
}

// SearchText performs text vector search against enrichment summaries.
func (s Search) SearchText(ctx context.Context, query string, topK int) ([]enrichment.Enrichment, error) {
	if s.textVectorStore == nil || s.embedder == nil {
		return nil, nil
	}

	if topK <= 0 {
		topK = 10
	}

	embeddings, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, nil
	}

	results, err := s.textVectorStore.Search(ctx,
		search.WithEmbedding(embeddings[0]),
		repository.WithLimit(topK),
	)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(results))
	for _, r := range results {
		id, err := strconv.ParseInt(r.SnippetID(), 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}

	return s.enrichmentStore.Find(ctx, repository.WithIDIn(ids))
}

// SearchCode performs code vector search against code snippet embeddings.
func (s Search) SearchCode(ctx context.Context, query string, topK int) ([]enrichment.Enrichment, error) {
	if s.codeVectorStore == nil || s.embedder == nil {
		return nil, nil
	}

	if topK <= 0 {
		topK = 10
	}

	embeddings, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, nil
	}

	results, err := s.codeVectorStore.Search(ctx,
		search.WithEmbedding(embeddings[0]),
		repository.WithLimit(topK),
	)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(results))
	for _, r := range results {
		id, err := strconv.ParseInt(r.SnippetID(), 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}

	return s.enrichmentStore.Find(ctx, repository.WithIDIn(ids))
}

// toFusionRequests converts search results to fusion requests.
func toFusionRequests(results []search.Result) []search.FusionRequest {
	requests := make([]search.FusionRequest, len(results))
	for i, r := range results {
		requests[i] = search.NewFusionRequest(r.SnippetID(), r.Score())
	}
	return requests
}

// orderByScore orders enrichments by their fused scores.
func orderByScore(enrichments []enrichment.Enrichment, scores map[string]float64) []enrichment.Enrichment {
	type scored struct {
		enrichment enrichment.Enrichment
		score      float64
	}

	scoredItems := make([]scored, 0, len(enrichments))
	for _, e := range enrichments {
		key := strconv.FormatInt(e.ID(), 10)
		score := scores[key]
		scoredItems = append(scoredItems, scored{enrichment: e, score: score})
	}

	// Sort by score descending
	for i := 0; i < len(scoredItems)-1; i++ {
		for j := i + 1; j < len(scoredItems); j++ {
			if scoredItems[j].score > scoredItems[i].score {
				scoredItems[i], scoredItems[j] = scoredItems[j], scoredItems[i]
			}
		}
	}

	result := make([]enrichment.Enrichment, len(scoredItems))
	for i, s := range scoredItems {
		result[i] = s.enrichment
	}

	return result
}
