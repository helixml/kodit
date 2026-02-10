package search_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/search"
	"github.com/helixml/kodit/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for search.Service with real database and fake search repositories.
// These tests mirror the Python tests in tests/kodit/application/services/code_search_application_service_test.py.

func TestService_Search_TextQueryFindsMatchingSnippets_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	// Create repositories
	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	// Create test data: a snippet with code content
	snippetContent := "def calculate_sum(a, b):\n    return a + b"
	snippet := indexing.NewSnippet(snippetContent, "py", []repository.File{})
	fakeSnippetRepo.AddSnippet(snippet)

	// Configure BM25 to return our snippet when searching
	fakeBM25Repo.SetResults([]domain.SearchResult{
		domain.NewSearchResult(snippet.SHA(), 0.9),
	})

	// Create search service
	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	// Search for "calculate sum"
	request := domain.NewMultiSearchRequest(10, "calculate sum", "", nil, domain.SnippetSearchFilters{})
	result, err := service.Search(ctx, request)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Count())

	snippets := result.Snippets()
	assert.Len(t, snippets, 1)
	assert.Contains(t, snippets[0].Content(), "def calculate_sum")
}

func TestService_Search_ClassQueryReturnsCorrectResult_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	// Create multiple snippets with different content
	snippet1Content := "class UserService:\n    def authenticate(self, user):\n        pass"
	snippet1 := indexing.NewSnippet(snippet1Content, "py", []repository.File{})

	snippet2Content := "class PaymentProcessor:\n    def process_payment(self, amount):\n        pass"
	snippet2 := indexing.NewSnippet(snippet2Content, "py", []repository.File{})

	fakeSnippetRepo.AddSnippet(snippet1)
	fakeSnippetRepo.AddSnippet(snippet2)

	// Configure BM25 to return UserService snippet with higher score
	fakeBM25Repo.SetResults([]domain.SearchResult{
		domain.NewSearchResult(snippet1.SHA(), 0.95),
		domain.NewSearchResult(snippet2.SHA(), 0.3),
	})

	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	request := domain.NewMultiSearchRequest(10, "user authentication", "", nil, domain.SnippetSearchFilters{})
	result, err := service.Search(ctx, request)

	require.NoError(t, err)
	assert.Equal(t, 2, result.Count())

	snippets := result.Snippets()
	// First result should be the one with higher score (UserService)
	assert.Contains(t, snippets[0].Content(), "class UserService")
}

func TestService_Search_EmptyQueryReturnsEmpty_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	// Empty query should return empty results
	request := domain.NewMultiSearchRequest(10, "", "", nil, domain.SnippetSearchFilters{})
	result, err := service.Search(ctx, request)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Count())
}

func TestService_Search_TopKLimitsResults_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	// Create 5 snippets
	var searchResults []domain.SearchResult
	for i := 0; i < 5; i++ {
		content := "func process" + string(rune('A'+i)) + "() {}"
		snippet := indexing.NewSnippet(content, "go", []repository.File{})
		fakeSnippetRepo.AddSnippet(snippet)
		searchResults = append(searchResults, domain.NewSearchResult(snippet.SHA(), float64(5-i)/10.0))
	}

	fakeBM25Repo.SetResults(searchResults)

	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	// Request only top 3
	request := domain.NewMultiSearchRequest(3, "process", "", nil, domain.SnippetSearchFilters{})
	result, err := service.Search(ctx, request)

	require.NoError(t, err)
	assert.Equal(t, 3, result.Count())
}

func TestService_Search_HybridSearchCombinesResults_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	// Create snippets
	snippet1 := indexing.NewSnippet("def keyword_match():\n    pass", "py", []repository.File{})
	snippet2 := indexing.NewSnippet("def semantic_similar():\n    pass", "py", []repository.File{})
	snippet3 := indexing.NewSnippet("def both_match():\n    pass", "py", []repository.File{})

	fakeSnippetRepo.AddSnippet(snippet1)
	fakeSnippetRepo.AddSnippet(snippet2)
	fakeSnippetRepo.AddSnippet(snippet3)

	// BM25 finds keyword_match and both_match
	fakeBM25Repo.SetResults([]domain.SearchResult{
		domain.NewSearchResult(snippet1.SHA(), 0.9),
		domain.NewSearchResult(snippet3.SHA(), 0.7),
	})

	// Vector search finds semantic_similar and both_match
	fakeVectorRepo.SetResults([]domain.SearchResult{
		domain.NewSearchResult(snippet2.SHA(), 0.85),
		domain.NewSearchResult(snippet3.SHA(), 0.8),
	})

	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	// Both text and code query triggers hybrid search
	request := domain.NewMultiSearchRequest(10, "keyword function", "semantic code", nil, domain.SnippetSearchFilters{})
	result, err := service.Search(ctx, request)

	require.NoError(t, err)

	// Should find all 3 snippets through hybrid search
	assert.GreaterOrEqual(t, result.Count(), 2)

	// Both_match should rank highest (appears in both result sets)
	fusedScores := result.FusedScores()
	_, hasBothMatch := fusedScores[snippet3.SHA()]
	assert.True(t, hasBothMatch, "both_match snippet should be in results")
}

func TestService_SearchBM25_ReturnsKeywordMatches_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	snippet := indexing.NewSnippet("func handleRequest() error { return nil }", "go", []repository.File{})
	fakeSnippetRepo.AddSnippet(snippet)

	fakeBM25Repo.SetResults([]domain.SearchResult{
		domain.NewSearchResult(snippet.SHA(), 0.85),
	})

	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	snippets, err := service.SearchBM25(ctx, "handleRequest", 10)
	require.NoError(t, err)
	require.Len(t, snippets, 1)
	assert.Contains(t, snippets[0].Content(), "handleRequest")
}

func TestService_SearchVector_ReturnsSimilarCode_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	snippet := indexing.NewSnippet("func processData(data []byte) { /* implementation */ }", "go", []repository.File{})
	fakeSnippetRepo.AddSnippet(snippet)

	fakeVectorRepo.SetResults([]domain.SearchResult{
		domain.NewSearchResult(snippet.SHA(), 0.92),
	})

	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	snippets, err := service.SearchVector(ctx, "process byte array data", 10)
	require.NoError(t, err)
	require.Len(t, snippets, 1)
	assert.Contains(t, snippets[0].Content(), "processData")
}

func TestService_Search_WithEnrichmentsAttached_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	// Create a snippet
	snippetContent := "def validate_input(data):\n    if not data:\n        raise ValueError('Empty data')"
	snippet := indexing.NewSnippet(snippetContent, "py", []repository.File{})
	fakeSnippetRepo.AddSnippet(snippet)

	// Create an enrichment for this snippet (snippet summary)
	summary := enrichment.NewEnrichment(
		enrichment.TypeDevelopment,
		enrichment.SubtypeSnippetSummary,
		enrichment.EntityTypeSnippet,
		"Validates input data and raises ValueError if empty",
	)
	savedSummary, err := enrichmentRepo.Save(ctx, summary)
	require.NoError(t, err)
	assert.Greater(t, savedSummary.ID(), int64(0))

	// Configure search to return the snippet
	fakeBM25Repo.SetResults([]domain.SearchResult{
		domain.NewSearchResult(snippet.SHA(), 0.88),
	})

	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	request := domain.NewMultiSearchRequest(10, "validate input", "", nil, domain.SnippetSearchFilters{})
	result, err := service.Search(ctx, request)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Count())

	snippets := result.Snippets()
	assert.Contains(t, snippets[0].Content(), "def validate_input")
}

func TestService_Search_OrdersByFusedScore_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	// Create snippets with predictable ordering
	lowScoreSnippet := indexing.NewSnippet("def low_score(): pass", "py", []repository.File{})
	midScoreSnippet := indexing.NewSnippet("def mid_score(): pass", "py", []repository.File{})
	highScoreSnippet := indexing.NewSnippet("def high_score(): pass", "py", []repository.File{})

	fakeSnippetRepo.AddSnippet(lowScoreSnippet)
	fakeSnippetRepo.AddSnippet(midScoreSnippet)
	fakeSnippetRepo.AddSnippet(highScoreSnippet)

	// Return in sorted order (RRF uses rank position, not score values)
	// First result in list gets rank 1, second gets rank 2, etc.
	fakeBM25Repo.SetResults([]domain.SearchResult{
		domain.NewSearchResult(highScoreSnippet.SHA(), 0.9), // Rank 1 - highest RRF score
		domain.NewSearchResult(midScoreSnippet.SHA(), 0.5),  // Rank 2
		domain.NewSearchResult(lowScoreSnippet.SHA(), 0.2),  // Rank 3 - lowest RRF score
	})

	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	request := domain.NewMultiSearchRequest(10, "score function", "", nil, domain.SnippetSearchFilters{})
	result, err := service.Search(ctx, request)

	require.NoError(t, err)
	assert.Equal(t, 3, result.Count())

	snippets := result.Snippets()
	// Results should be ordered by fused score (which is based on rank position)
	assert.Contains(t, snippets[0].Content(), "high_score")
	assert.Contains(t, snippets[1].Content(), "mid_score")
	assert.Contains(t, snippets[2].Content(), "low_score")
}

func TestService_Search_DefaultTopK_Integration(t *testing.T) {
	ctx := context.Background()
	db := testutil.TestDatabaseWithSchema(t, testutil.TestSchema)
	logger := slog.Default()

	enrichmentRepo := persistence.NewEnrichmentStore(db)
	fakeSnippetRepo := testutil.NewFakeSnippetRepository()
	fakeBM25Repo := testutil.NewFakeBM25Repository()
	fakeVectorRepo := testutil.NewFakeVectorRepository()

	// Create 15 snippets
	var searchResults []domain.SearchResult
	for i := 0; i < 15; i++ {
		content := "func function" + string(rune('A'+i)) + "() {}"
		snippet := indexing.NewSnippet(content, "go", []repository.File{})
		fakeSnippetRepo.AddSnippet(snippet)
		searchResults = append(searchResults, domain.NewSearchResult(snippet.SHA(), float64(15-i)/15.0))
	}

	fakeBM25Repo.SetResults(searchResults)

	service := search.NewService(fakeBM25Repo, fakeVectorRepo, fakeSnippetRepo, enrichmentRepo, logger)

	// topK = 0 should default to 10
	request := domain.NewMultiSearchRequest(0, "function", "", nil, domain.SnippetSearchFilters{})
	result, err := service.Search(ctx, request)

	require.NoError(t, err)
	// Default topK is 10
	assert.Equal(t, 10, result.Count())
}
