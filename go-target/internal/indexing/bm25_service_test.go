package indexing

import (
	"context"
	"testing"

	"github.com/helixml/kodit/internal/domain"
	"github.com/stretchr/testify/assert"
)

// FakeBM25Repository is a test double for BM25Repository.
type FakeBM25Repository struct {
	indexed  []domain.IndexRequest
	deleted  []domain.DeleteRequest
	results  []domain.SearchResult
	indexErr error
	searchErr error
	deleteErr error
}

func (f *FakeBM25Repository) Index(_ context.Context, request domain.IndexRequest) error {
	if f.indexErr != nil {
		return f.indexErr
	}
	f.indexed = append(f.indexed, request)
	return nil
}

func (f *FakeBM25Repository) Search(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.results, nil
}

func (f *FakeBM25Repository) Delete(_ context.Context, request domain.DeleteRequest) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, request)
	return nil
}

func TestBM25ServiceIndex(t *testing.T) {
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
			expectIndexed: 1, // One call with both docs
		},
		{
			name: "filters invalid documents",
			documents: []domain.Document{
				domain.NewDocument("", "no id"),         // Invalid: empty ID
				domain.NewDocument("id1", ""),           // Invalid: empty text
				domain.NewDocument("id2", "   "),        // Invalid: whitespace only
				domain.NewDocument("id3", "valid text"), // Valid
			},
			expectIndexed: 1,
		},
		{
			name: "all invalid",
			documents: []domain.Document{
				domain.NewDocument("", "no id"),
				domain.NewDocument("id1", ""),
			},
			expectIndexed: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeBM25Repository{}
			service := NewBM25Service(repo)

			request := domain.NewIndexRequest(tt.documents)
			err := service.Index(context.Background(), request)

			assert.NoError(t, err)
			assert.Len(t, repo.indexed, tt.expectIndexed)
		})
	}
}

func TestBM25ServiceSearch(t *testing.T) {
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
			name:        "whitespace query",
			query:       "   ",
			topK:        10,
			expectError: ErrEmptyQuery,
		},
		{
			name:        "invalid topK",
			query:       "search",
			topK:        0,
			expectError: ErrInvalidTopK,
		},
		{
			name:        "negative topK",
			query:       "search",
			topK:        -1,
			expectError: ErrInvalidTopK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeBM25Repository{
				results: []domain.SearchResult{
					domain.NewSearchResult("id1", 0.9),
				},
			}
			service := NewBM25Service(repo)

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

func TestBM25ServiceDelete(t *testing.T) {
	tests := []struct {
		name          string
		ids           []string
		expectDeleted int
	}{
		{
			name:          "empty ids",
			ids:           []string{},
			expectDeleted: 0,
		},
		{
			name:          "valid ids",
			ids:           []string{"id1", "id2"},
			expectDeleted: 1,
		},
		{
			name:          "filters invalid ids",
			ids:           []string{"", "0", "-1", "valid"},
			expectDeleted: 1,
		},
		{
			name:          "all invalid",
			ids:           []string{"", "0", "-1"},
			expectDeleted: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeBM25Repository{}
			service := NewBM25Service(repo)

			request := domain.NewDeleteRequest(tt.ids)
			err := service.Delete(context.Background(), request)

			assert.NoError(t, err)
			assert.Len(t, repo.deleted, tt.expectDeleted)
		})
	}
}
