package indexing

import (
	"context"
	"testing"

	"github.com/helixml/kodit/internal/domain"
	"github.com/stretchr/testify/assert"
)

// FakeVectorSearchRepository is a test double for VectorSearchRepository.
type FakeVectorSearchRepository struct {
	indexed       []domain.IndexRequest
	deleted       []domain.DeleteRequest
	results       []domain.SearchResult
	hasEmbeddings map[string]bool
	indexErr      error
	searchErr     error
	deleteErr     error
	hasErr        error
}

func (f *FakeVectorSearchRepository) Index(_ context.Context, request domain.IndexRequest) error {
	if f.indexErr != nil {
		return f.indexErr
	}
	f.indexed = append(f.indexed, request)
	return nil
}

func (f *FakeVectorSearchRepository) Search(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.results, nil
}

func (f *FakeVectorSearchRepository) HasEmbedding(_ context.Context, snippetID string, _ EmbeddingType) (bool, error) {
	if f.hasErr != nil {
		return false, f.hasErr
	}
	if f.hasEmbeddings == nil {
		return false, nil
	}
	return f.hasEmbeddings[snippetID], nil
}

func (f *FakeVectorSearchRepository) Delete(_ context.Context, request domain.DeleteRequest) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, request)
	return nil
}

func (f *FakeVectorSearchRepository) EmbeddingsForSnippets(_ context.Context, _ []string) ([]EmbeddingInfo, error) {
	return []EmbeddingInfo{}, nil
}

// FakeEmbeddingProvider is a test double for provider.Provider.
type FakeEmbeddingProvider struct{}

func (f *FakeEmbeddingProvider) SupportsTextGeneration() bool {
	return false
}

func (f *FakeEmbeddingProvider) SupportsEmbedding() bool {
	return true
}

func (f *FakeEmbeddingProvider) Close() error {
	return nil
}

func TestEmbeddingServiceIndex(t *testing.T) {
	tests := []struct {
		name          string
		documents     []domain.Document
		expectIndexed int
	}{
		{
			name:          "empty documents",
			documents:     []domain.Document{},
			expectIndexed: 0,
		},
		{
			name: "valid documents",
			documents: []domain.Document{
				domain.NewDocument("id1", "text1"),
				domain.NewDocument("id2", "text2"),
			},
			expectIndexed: 1,
		},
		{
			name: "filters invalid documents",
			documents: []domain.Document{
				domain.NewDocument("", "no id"),
				domain.NewDocument("id1", ""),
				domain.NewDocument("id2", "valid"),
			},
			expectIndexed: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeVectorSearchRepository{}
			provider := &FakeEmbeddingProvider{}
			service := NewEmbeddingService(provider, repo)

			request := domain.NewIndexRequest(tt.documents)
			err := service.Index(context.Background(), request)

			assert.NoError(t, err)
			assert.Len(t, repo.indexed, tt.expectIndexed)
		})
	}
}

func TestEmbeddingServiceSearch(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		topK        int
		expectError error
	}{
		{
			name:        "valid query",
			query:       "search term",
			topK:        10,
			expectError: nil,
		},
		{
			name:        "empty query",
			query:       "",
			topK:        10,
			expectError: ErrEmptyQuery,
		},
		{
			name:        "invalid topK",
			query:       "search",
			topK:        0,
			expectError: ErrInvalidTopK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeVectorSearchRepository{
				results: []domain.SearchResult{
					domain.NewSearchResult("id1", 0.9),
				},
			}
			provider := &FakeEmbeddingProvider{}
			service := NewEmbeddingService(provider, repo)

			request := domain.NewSearchRequest(tt.query, tt.topK, nil)
			results, err := service.Search(context.Background(), request)

			if tt.expectError != nil {
				assert.ErrorIs(t, err, tt.expectError)
				assert.Nil(t, results)
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, 1)
			}
		})
	}
}

func TestEmbeddingServiceSearchDeduplicates(t *testing.T) {
	repo := &FakeVectorSearchRepository{
		results: []domain.SearchResult{
			domain.NewSearchResult("id1", 0.9),
			domain.NewSearchResult("id1", 0.8), // Duplicate
			domain.NewSearchResult("id2", 0.7),
		},
	}
	provider := &FakeEmbeddingProvider{}
	service := NewEmbeddingService(provider, repo)

	request := domain.NewSearchRequest("query", 10, nil)
	results, err := service.Search(context.Background(), request)

	assert.NoError(t, err)
	assert.Len(t, results, 2) // Deduplicated
	assert.Equal(t, "id1", results[0].SnippetID())
	assert.Equal(t, 0.9, results[0].Score()) // First occurrence wins
	assert.Equal(t, "id2", results[1].SnippetID())
}

func TestEmbeddingServiceHasEmbedding(t *testing.T) {
	tests := []struct {
		name       string
		snippetID  string
		has        bool
		expectErr  error
		expectHas  bool
	}{
		{
			name:      "has embedding",
			snippetID: "id1",
			has:       true,
			expectHas: true,
		},
		{
			name:      "no embedding",
			snippetID: "id2",
			has:       false,
			expectHas: false,
		},
		{
			name:      "empty snippet ID",
			snippetID: "",
			expectErr: ErrInvalidSnippetID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeVectorSearchRepository{
				hasEmbeddings: map[string]bool{
					"id1": true,
				},
			}
			provider := &FakeEmbeddingProvider{}
			service := NewEmbeddingService(provider, repo)

			has, err := service.HasEmbedding(context.Background(), tt.snippetID, EmbeddingTypeCode)

			if tt.expectErr != nil {
				assert.ErrorIs(t, err, tt.expectErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectHas, has)
			}
		})
	}
}
