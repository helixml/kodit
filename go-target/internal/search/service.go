package search

import (
	"context"
	"log/slog"
	"maps"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
)

// MultiSearchResult represents the result of a multi-modal search.
type MultiSearchResult struct {
	snippets    []indexing.Snippet
	enrichments []enrichment.Enrichment
	fusedScores map[string]float64
}

// NewMultiSearchResult creates a new MultiSearchResult.
func NewMultiSearchResult(
	snippets []indexing.Snippet,
	enrichments []enrichment.Enrichment,
	fusedScores map[string]float64,
) MultiSearchResult {
	snips := make([]indexing.Snippet, len(snippets))
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
func (r MultiSearchResult) Snippets() []indexing.Snippet {
	result := make([]indexing.Snippet, len(r.snippets))
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

// Service orchestrates hybrid code search across BM25 and vector indexes.
type Service struct {
	bm25Repo       indexing.BM25Repository
	vectorRepo     indexing.VectorSearchRepository
	snippetRepo    indexing.SnippetRepository
	enrichmentRepo enrichment.EnrichmentStore
	fusionService  FusionService
	logger         *slog.Logger
}

// NewService creates a new code search Service.
func NewService(
	bm25Repo indexing.BM25Repository,
	vectorRepo indexing.VectorSearchRepository,
	snippetRepo indexing.SnippetRepository,
	enrichmentRepo enrichment.EnrichmentStore,
	logger *slog.Logger,
) Service {
	if logger == nil {
		logger = slog.Default()
	}
	return Service{
		bm25Repo:       bm25Repo,
		vectorRepo:     vectorRepo,
		snippetRepo:    snippetRepo,
		enrichmentRepo: enrichmentRepo,
		fusionService:  NewFusionService(),
		logger:         logger,
	}
}

// Search performs a hybrid search combining BM25 and vector search results.
func (s Service) Search(ctx context.Context, request domain.MultiSearchRequest) (MultiSearchResult, error) {
	// Determine which queries to run
	textQuery := request.TextQuery()
	codeQuery := request.CodeQuery()
	topK := request.TopK()

	if topK <= 0 {
		topK = 10
	}

	// Collect results from each search method
	var bm25Results, vectorResults []domain.SearchResult

	// Run BM25 search if text query provided
	if textQuery != "" {
		searchReq := domain.NewSearchRequest(textQuery, topK*2, nil) // Get more for fusion
		results, err := s.bm25Repo.Search(ctx, searchReq)
		if err != nil {
			s.logger.Warn("BM25 search failed", "error", err)
		} else {
			bm25Results = results
		}
	}

	// Run vector search if code query provided
	if codeQuery != "" {
		searchReq := domain.NewSearchRequest(codeQuery, topK*2, nil)
		results, err := s.vectorRepo.Search(ctx, searchReq)
		if err != nil {
			s.logger.Warn("vector search failed", "error", err)
		} else {
			vectorResults = results
		}
	}

	// If both queries are empty, try using text query for both
	if textQuery == "" && codeQuery == "" {
		return NewMultiSearchResult(nil, nil, nil), nil
	}

	// Convert to fusion requests
	bm25Fusion := toFusionRequests(bm25Results)
	vectorFusion := toFusionRequests(vectorResults)

	// Fuse results
	var fusedResults []domain.FusionResult
	if len(bm25Fusion) > 0 && len(vectorFusion) > 0 {
		fusedResults = s.fusionService.FuseTopK(topK, bm25Fusion, vectorFusion)
	} else if len(bm25Fusion) > 0 {
		fusedResults = s.fusionService.FuseTopK(topK, bm25Fusion)
	} else if len(vectorFusion) > 0 {
		fusedResults = s.fusionService.FuseTopK(topK, vectorFusion)
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
	snippets, err := s.snippetRepo.ByIDs(ctx, snippetIDs)
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

// SearchBM25 performs BM25-only search.
func (s Service) SearchBM25(ctx context.Context, query string, topK int) ([]indexing.Snippet, error) {
	if topK <= 0 {
		topK = 10
	}

	searchReq := domain.NewSearchRequest(query, topK, nil)
	results, err := s.bm25Repo.Search(ctx, searchReq)
	if err != nil {
		return nil, err
	}

	snippetIDs := make([]string, len(results))
	for i, r := range results {
		snippetIDs[i] = r.SnippetID()
	}

	return s.snippetRepo.ByIDs(ctx, snippetIDs)
}

// SearchVector performs vector-only search.
func (s Service) SearchVector(ctx context.Context, query string, topK int) ([]indexing.Snippet, error) {
	if topK <= 0 {
		topK = 10
	}

	searchReq := domain.NewSearchRequest(query, topK, nil)
	results, err := s.vectorRepo.Search(ctx, searchReq)
	if err != nil {
		return nil, err
	}

	snippetIDs := make([]string, len(results))
	for i, r := range results {
		snippetIDs[i] = r.SnippetID()
	}

	return s.snippetRepo.ByIDs(ctx, snippetIDs)
}

// fetchEnrichments retrieves enrichments for the given snippet IDs.
func (s Service) fetchEnrichments(_ context.Context, snippetIDs []string) ([]enrichment.Enrichment, error) {
	if len(snippetIDs) == 0 {
		return nil, nil
	}

	// Use the association repository to find enrichments linked to these snippets
	// For now, return empty - full implementation requires EnrichmentAssociationRepository
	return nil, nil
}

// toFusionRequests converts search results to fusion requests.
func toFusionRequests(results []domain.SearchResult) []domain.FusionRequest {
	requests := make([]domain.FusionRequest, len(results))
	for i, r := range results {
		requests[i] = domain.NewFusionRequest(r.SnippetID(), r.Score())
	}
	return requests
}

// orderByScore orders snippets by their fused scores.
func orderByScore(snippets []indexing.Snippet, scores map[string]float64) []indexing.Snippet {
	// Create a map of SHA to snippet for lookup
	snippetMap := make(map[string]indexing.Snippet, len(snippets))
	for _, s := range snippets {
		snippetMap[s.SHA()] = s
	}

	// Create ordered list based on scores
	type scored struct {
		snippet indexing.Snippet
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

	result := make([]indexing.Snippet, len(scoredSnippets))
	for i, s := range scoredSnippets {
		result[i] = s.snippet
	}

	return result
}
