package search

import (
	"context"
	"testing"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/stretchr/testify/assert"
)

// FakeBM25Repository is a test double for BM25Repository.
type FakeBM25Repository struct {
	indexFn  func(ctx context.Context, request domain.IndexRequest) error
	searchFn func(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error)
	deleteFn func(ctx context.Context, request domain.DeleteRequest) error
}

func (f *FakeBM25Repository) Index(ctx context.Context, request domain.IndexRequest) error {
	if f.indexFn != nil {
		return f.indexFn(ctx, request)
	}
	return nil
}

func (f *FakeBM25Repository) Search(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error) {
	if f.searchFn != nil {
		return f.searchFn(ctx, request)
	}
	return nil, nil
}

func (f *FakeBM25Repository) Delete(ctx context.Context, request domain.DeleteRequest) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, request)
	}
	return nil
}

// FakeVectorRepository is a test double for VectorSearchRepository.
type FakeVectorRepository struct {
	indexFn        func(ctx context.Context, request domain.IndexRequest) error
	searchFn       func(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error)
	hasEmbeddingFn func(ctx context.Context, snippetID string, embeddingType indexing.EmbeddingType) (bool, error)
	deleteFn       func(ctx context.Context, request domain.DeleteRequest) error
}

func (f *FakeVectorRepository) Index(ctx context.Context, request domain.IndexRequest) error {
	if f.indexFn != nil {
		return f.indexFn(ctx, request)
	}
	return nil
}

func (f *FakeVectorRepository) Search(ctx context.Context, request domain.SearchRequest) ([]domain.SearchResult, error) {
	if f.searchFn != nil {
		return f.searchFn(ctx, request)
	}
	return nil, nil
}

func (f *FakeVectorRepository) HasEmbedding(ctx context.Context, snippetID string, embeddingType indexing.EmbeddingType) (bool, error) {
	if f.hasEmbeddingFn != nil {
		return f.hasEmbeddingFn(ctx, snippetID, embeddingType)
	}
	return false, nil
}

func (f *FakeVectorRepository) Delete(ctx context.Context, request domain.DeleteRequest) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, request)
	}
	return nil
}

func (f *FakeVectorRepository) EmbeddingsForSnippets(_ context.Context, _ []string) ([]indexing.EmbeddingInfo, error) {
	return []indexing.EmbeddingInfo{}, nil
}

// FakeSnippetRepository is a test double for SnippetRepository.
type FakeSnippetRepository struct {
	saveFn              func(ctx context.Context, commitSHA string, snippets []indexing.Snippet) error
	snippetsForCommitFn func(ctx context.Context, commitSHA string) ([]indexing.Snippet, error)
	deleteForCommitFn   func(ctx context.Context, commitSHA string) error
	searchFn            func(ctx context.Context, request domain.MultiSearchRequest) ([]indexing.Snippet, error)
	byIDsFn             func(ctx context.Context, ids []string) ([]indexing.Snippet, error)
}

func (f *FakeSnippetRepository) Save(ctx context.Context, commitSHA string, snippets []indexing.Snippet) error {
	if f.saveFn != nil {
		return f.saveFn(ctx, commitSHA, snippets)
	}
	return nil
}

func (f *FakeSnippetRepository) SnippetsForCommit(ctx context.Context, commitSHA string) ([]indexing.Snippet, error) {
	if f.snippetsForCommitFn != nil {
		return f.snippetsForCommitFn(ctx, commitSHA)
	}
	return nil, nil
}

func (f *FakeSnippetRepository) DeleteForCommit(ctx context.Context, commitSHA string) error {
	if f.deleteForCommitFn != nil {
		return f.deleteForCommitFn(ctx, commitSHA)
	}
	return nil
}

func (f *FakeSnippetRepository) Search(ctx context.Context, request domain.MultiSearchRequest) ([]indexing.Snippet, error) {
	if f.searchFn != nil {
		return f.searchFn(ctx, request)
	}
	return nil, nil
}

func (f *FakeSnippetRepository) ByIDs(ctx context.Context, ids []string) ([]indexing.Snippet, error) {
	if f.byIDsFn != nil {
		return f.byIDsFn(ctx, ids)
	}
	return nil, nil
}

// FakeEnrichmentRepository is a test double for EnrichmentRepository.
type FakeEnrichmentRepository struct {
	saveFn                 func(ctx context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error)
	getFn                  func(ctx context.Context, id int64) (enrichment.Enrichment, error)
	findByTypeFn           func(ctx context.Context, typ enrichment.Type) ([]enrichment.Enrichment, error)
	findByTypeAndSubtypeFn func(ctx context.Context, typ enrichment.Type, subtype enrichment.Subtype) ([]enrichment.Enrichment, error)
	findByEntityKeyFn      func(ctx context.Context, key enrichment.EntityTypeKey) ([]enrichment.Enrichment, error)
	deleteFn               func(ctx context.Context, e enrichment.Enrichment) error
}

func (f *FakeEnrichmentRepository) Save(ctx context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error) {
	if f.saveFn != nil {
		return f.saveFn(ctx, e)
	}
	return e, nil
}

func (f *FakeEnrichmentRepository) Get(ctx context.Context, id int64) (enrichment.Enrichment, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return enrichment.Enrichment{}, nil
}

func (f *FakeEnrichmentRepository) FindByType(ctx context.Context, typ enrichment.Type) ([]enrichment.Enrichment, error) {
	if f.findByTypeFn != nil {
		return f.findByTypeFn(ctx, typ)
	}
	return nil, nil
}

func (f *FakeEnrichmentRepository) FindByTypeAndSubtype(ctx context.Context, typ enrichment.Type, subtype enrichment.Subtype) ([]enrichment.Enrichment, error) {
	if f.findByTypeAndSubtypeFn != nil {
		return f.findByTypeAndSubtypeFn(ctx, typ, subtype)
	}
	return nil, nil
}

func (f *FakeEnrichmentRepository) FindByEntityKey(ctx context.Context, key enrichment.EntityTypeKey) ([]enrichment.Enrichment, error) {
	if f.findByEntityKeyFn != nil {
		return f.findByEntityKeyFn(ctx, key)
	}
	return nil, nil
}

func (f *FakeEnrichmentRepository) Delete(ctx context.Context, e enrichment.Enrichment) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, e)
	}
	return nil
}

func TestNewService(t *testing.T) {
	svc := NewService(
		&FakeBM25Repository{},
		&FakeVectorRepository{},
		&FakeSnippetRepository{},
		&FakeEnrichmentRepository{},
		nil,
	)

	assert.NotNil(t, svc)
}

func TestService_Search_BothQueriesEmpty(t *testing.T) {
	svc := NewService(
		&FakeBM25Repository{},
		&FakeVectorRepository{},
		&FakeSnippetRepository{},
		&FakeEnrichmentRepository{},
		nil,
	)

	request := domain.NewMultiSearchRequest(10, "", "", nil, domain.SnippetSearchFilters{})
	result, err := svc.Search(context.Background(), request)

	assert.NoError(t, err)
	assert.Equal(t, 0, result.Count())
}

func TestService_Search_TextQueryOnly(t *testing.T) {
	snippets := []indexing.Snippet{
		indexing.NewSnippet("func main() {}", ".go", nil),
	}

	bm25Repo := &FakeBM25Repository{
		searchFn: func(_ context.Context, req domain.SearchRequest) ([]domain.SearchResult, error) {
			return []domain.SearchResult{
				domain.NewSearchResult(snippets[0].SHA(), 1.0),
			}, nil
		},
	}

	snippetRepo := &FakeSnippetRepository{
		byIDsFn: func(_ context.Context, ids []string) ([]indexing.Snippet, error) {
			return snippets, nil
		},
	}

	svc := NewService(
		bm25Repo,
		&FakeVectorRepository{},
		snippetRepo,
		&FakeEnrichmentRepository{},
		nil,
	)

	request := domain.NewMultiSearchRequest(10, "main function", "", nil, domain.SnippetSearchFilters{})
	result, err := svc.Search(context.Background(), request)

	assert.NoError(t, err)
	assert.Equal(t, 1, result.Count())
	assert.Equal(t, snippets[0].SHA(), result.Snippets()[0].SHA())
}

func TestService_Search_CodeQueryOnly(t *testing.T) {
	snippets := []indexing.Snippet{
		indexing.NewSnippet("func main() {}", ".go", nil),
	}

	vectorRepo := &FakeVectorRepository{
		searchFn: func(_ context.Context, req domain.SearchRequest) ([]domain.SearchResult, error) {
			return []domain.SearchResult{
				domain.NewSearchResult(snippets[0].SHA(), 1.0),
			}, nil
		},
	}

	snippetRepo := &FakeSnippetRepository{
		byIDsFn: func(_ context.Context, ids []string) ([]indexing.Snippet, error) {
			return snippets, nil
		},
	}

	svc := NewService(
		&FakeBM25Repository{},
		vectorRepo,
		snippetRepo,
		&FakeEnrichmentRepository{},
		nil,
	)

	request := domain.NewMultiSearchRequest(10, "", "func main", nil, domain.SnippetSearchFilters{})
	result, err := svc.Search(context.Background(), request)

	assert.NoError(t, err)
	assert.Equal(t, 1, result.Count())
}

func TestService_Search_HybridSearch(t *testing.T) {
	snippet1 := indexing.NewSnippet("func main() { fmt.Println() }", ".go", nil)
	snippet2 := indexing.NewSnippet("func helper() { return }", ".go", nil)

	bm25Repo := &FakeBM25Repository{
		searchFn: func(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
			return []domain.SearchResult{
				domain.NewSearchResult(snippet1.SHA(), 1.0),
				domain.NewSearchResult(snippet2.SHA(), 0.5),
			}, nil
		},
	}

	vectorRepo := &FakeVectorRepository{
		searchFn: func(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
			return []domain.SearchResult{
				domain.NewSearchResult(snippet2.SHA(), 1.0),
				domain.NewSearchResult(snippet1.SHA(), 0.5),
			}, nil
		},
	}

	snippetRepo := &FakeSnippetRepository{
		byIDsFn: func(_ context.Context, ids []string) ([]indexing.Snippet, error) {
			result := make([]indexing.Snippet, 0)
			for _, id := range ids {
				if id == snippet1.SHA() {
					result = append(result, snippet1)
				} else if id == snippet2.SHA() {
					result = append(result, snippet2)
				}
			}
			return result, nil
		},
	}

	svc := NewService(
		bm25Repo,
		vectorRepo,
		snippetRepo,
		&FakeEnrichmentRepository{},
		nil,
	)

	request := domain.NewMultiSearchRequest(10, "print function", "func main", nil, domain.SnippetSearchFilters{})
	result, err := svc.Search(context.Background(), request)

	assert.NoError(t, err)
	assert.Equal(t, 2, result.Count())

	// Both snippets appear in both lists, so they should have similar fused scores
	// snippet2: rank 2 in bm25 + rank 1 in vector = highest combined
	// snippet1: rank 1 in bm25 + rank 2 in vector = same combined
	// Order may vary due to same score

	scores := result.FusedScores()
	assert.Len(t, scores, 2)
	assert.Greater(t, scores[snippet1.SHA()], 0.0)
	assert.Greater(t, scores[snippet2.SHA()], 0.0)
}

func TestService_SearchBM25(t *testing.T) {
	snippets := []indexing.Snippet{
		indexing.NewSnippet("func main() {}", ".go", nil),
	}

	bm25Repo := &FakeBM25Repository{
		searchFn: func(_ context.Context, req domain.SearchRequest) ([]domain.SearchResult, error) {
			assert.Equal(t, "main", req.Query())
			assert.Equal(t, 5, req.TopK())
			return []domain.SearchResult{
				domain.NewSearchResult(snippets[0].SHA(), 1.0),
			}, nil
		},
	}

	snippetRepo := &FakeSnippetRepository{
		byIDsFn: func(_ context.Context, _ []string) ([]indexing.Snippet, error) {
			return snippets, nil
		},
	}

	svc := NewService(
		bm25Repo,
		&FakeVectorRepository{},
		snippetRepo,
		&FakeEnrichmentRepository{},
		nil,
	)

	result, err := svc.SearchBM25(context.Background(), "main", 5)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestService_SearchVector(t *testing.T) {
	snippets := []indexing.Snippet{
		indexing.NewSnippet("func main() {}", ".go", nil),
	}

	vectorRepo := &FakeVectorRepository{
		searchFn: func(_ context.Context, req domain.SearchRequest) ([]domain.SearchResult, error) {
			return []domain.SearchResult{
				domain.NewSearchResult(snippets[0].SHA(), 0.95),
			}, nil
		},
	}

	snippetRepo := &FakeSnippetRepository{
		byIDsFn: func(_ context.Context, _ []string) ([]indexing.Snippet, error) {
			return snippets, nil
		},
	}

	svc := NewService(
		&FakeBM25Repository{},
		vectorRepo,
		snippetRepo,
		&FakeEnrichmentRepository{},
		nil,
	)

	result, err := svc.SearchVector(context.Background(), "main function", 5)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestService_SearchBM25_DefaultTopK(t *testing.T) {
	bm25Repo := &FakeBM25Repository{
		searchFn: func(_ context.Context, req domain.SearchRequest) ([]domain.SearchResult, error) {
			assert.Equal(t, 10, req.TopK())
			return nil, nil
		},
	}

	snippetRepo := &FakeSnippetRepository{}

	svc := NewService(
		bm25Repo,
		&FakeVectorRepository{},
		snippetRepo,
		&FakeEnrichmentRepository{},
		nil,
	)

	_, _ = svc.SearchBM25(context.Background(), "query", 0)
}

func TestMultiSearchResult(t *testing.T) {
	snippets := []indexing.Snippet{
		indexing.NewSnippet("func main() {}", ".go", nil),
	}
	enrichments := []enrichment.Enrichment{
		enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippetSummary, enrichment.EntityTypeSnippet, "A main function"),
	}
	scores := map[string]float64{
		snippets[0].SHA(): 0.95,
	}

	result := NewMultiSearchResult(snippets, enrichments, scores)

	assert.Equal(t, 1, result.Count())
	assert.Len(t, result.Snippets(), 1)
	assert.Len(t, result.Enrichments(), 1)
	assert.Len(t, result.FusedScores(), 1)

	// Verify immutability - modifying returned slices shouldn't affect original
	returnedSnippets := result.Snippets()
	_ = append(returnedSnippets, indexing.NewSnippet("extra", ".go", nil))
	assert.Equal(t, 1, result.Count())
}
