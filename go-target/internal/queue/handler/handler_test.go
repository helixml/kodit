package handler

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/queue"
	"github.com/helixml/kodit/internal/tracking"
)

type fakeTrackerFactory struct {
	logger *slog.Logger
}

func (f *fakeTrackerFactory) ForOperation(operation queue.TaskOperation, trackableType domain.TrackableType, trackableID int64) *tracking.Tracker {
	return tracking.TrackerForOperation(operation, f.logger, trackableType, trackableID)
}

type fakeSnippetRepository struct {
	snippetsByCommit map[string][]indexing.Snippet
}

func newFakeSnippetRepository() *fakeSnippetRepository {
	return &fakeSnippetRepository{
		snippetsByCommit: make(map[string][]indexing.Snippet),
	}
}

func (f *fakeSnippetRepository) Save(_ context.Context, commitSHA string, snippets []indexing.Snippet) error {
	f.snippetsByCommit[commitSHA] = snippets
	return nil
}

func (f *fakeSnippetRepository) SnippetsForCommit(_ context.Context, commitSHA string) ([]indexing.Snippet, error) {
	return f.snippetsByCommit[commitSHA], nil
}

func (f *fakeSnippetRepository) DeleteForCommit(_ context.Context, commitSHA string) error {
	delete(f.snippetsByCommit, commitSHA)
	return nil
}

func (f *fakeSnippetRepository) Search(_ context.Context, _ domain.MultiSearchRequest) ([]indexing.Snippet, error) {
	return nil, nil
}

func (f *fakeSnippetRepository) ByIDs(_ context.Context, _ []string) ([]indexing.Snippet, error) {
	return nil, nil
}

type fakeBM25Repository struct {
	indexed []domain.IndexRequest
}

func newFakeBM25Repository() *fakeBM25Repository {
	return &fakeBM25Repository{indexed: make([]domain.IndexRequest, 0)}
}

func (f *fakeBM25Repository) Index(_ context.Context, request domain.IndexRequest) error {
	f.indexed = append(f.indexed, request)
	return nil
}

func (f *fakeBM25Repository) Search(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
	return nil, nil
}

func (f *fakeBM25Repository) Delete(_ context.Context, _ domain.DeleteRequest) error {
	return nil
}

type fakeVectorRepository struct {
	indexed      []domain.IndexRequest
	hasEmbedding map[string]bool
}

func newFakeVectorRepository() *fakeVectorRepository {
	return &fakeVectorRepository{
		indexed:      make([]domain.IndexRequest, 0),
		hasEmbedding: make(map[string]bool),
	}
}

func (f *fakeVectorRepository) Index(_ context.Context, request domain.IndexRequest) error {
	f.indexed = append(f.indexed, request)
	for _, doc := range request.Documents() {
		f.hasEmbedding[doc.SnippetID()] = true
	}
	return nil
}

func (f *fakeVectorRepository) Search(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
	return nil, nil
}

func (f *fakeVectorRepository) HasEmbedding(_ context.Context, snippetID string, _ indexing.EmbeddingType) (bool, error) {
	return f.hasEmbedding[snippetID], nil
}

func (f *fakeVectorRepository) Delete(_ context.Context, _ domain.DeleteRequest) error {
	return nil
}

func (f *fakeVectorRepository) EmbeddingsForSnippets(_ context.Context, _ []string) ([]indexing.EmbeddingInfo, error) {
	return []indexing.EmbeddingInfo{}, nil
}

func TestExtractInt64(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		key     string
		want    int64
		wantErr bool
	}{
		{
			name:    "int64 value",
			payload: map[string]any{"id": int64(42)},
			key:     "id",
			want:    42,
			wantErr: false,
		},
		{
			name:    "int value",
			payload: map[string]any{"id": 42},
			key:     "id",
			want:    42,
			wantErr: false,
		},
		{
			name:    "float64 value",
			payload: map[string]any{"id": float64(42)},
			key:     "id",
			want:    42,
			wantErr: false,
		},
		{
			name:    "missing key",
			payload: map[string]any{},
			key:     "id",
			want:    0,
			wantErr: true,
		},
		{
			name:    "wrong type",
			payload: map[string]any{"id": "not a number"},
			key:     "id",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractInt64(tt.payload, tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExtractString(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		key     string
		want    string
		wantErr bool
	}{
		{
			name:    "string value",
			payload: map[string]any{"sha": "abc123"},
			key:     "sha",
			want:    "abc123",
			wantErr: false,
		},
		{
			name:    "missing key",
			payload: map[string]any{},
			key:     "sha",
			want:    "",
			wantErr: true,
		},
		{
			name:    "wrong type",
			payload: map[string]any{"sha": 123},
			key:     "sha",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractString(tt.payload, tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCreateBM25Index_Execute_NoSnippets(t *testing.T) {
	logger := slog.Default()
	bm25Repo := newFakeBM25Repository()
	bm25Service := indexing.NewBM25Service(bm25Repo)
	snippetRepo := newFakeSnippetRepository()
	trackerFactory := &fakeTrackerFactory{logger: logger}

	handler := NewCreateBM25Index(bm25Service, snippetRepo, trackerFactory, logger)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    "abc123def456789012345678901234567890abcd",
	}

	err := handler.Execute(context.Background(), payload)
	require.NoError(t, err)

	assert.Empty(t, bm25Repo.indexed)
}

func TestCreateBM25Index_Execute_WithSnippets(t *testing.T) {
	logger := slog.Default()
	bm25Repo := newFakeBM25Repository()
	bm25Service := indexing.NewBM25Service(bm25Repo)
	snippetRepo := newFakeSnippetRepository()
	trackerFactory := &fakeTrackerFactory{logger: logger}

	commitSHA := "abc123def456789012345678901234567890abcd"
	snippet := indexing.NewSnippet("func main() {}", ".go", nil)
	snippetRepo.snippetsByCommit[commitSHA] = []indexing.Snippet{snippet}

	handler := NewCreateBM25Index(bm25Service, snippetRepo, trackerFactory, logger)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    commitSHA,
	}

	err := handler.Execute(context.Background(), payload)
	require.NoError(t, err)

	require.Len(t, bm25Repo.indexed, 1)
	assert.Len(t, bm25Repo.indexed[0].Documents(), 1)
}

func TestCreateCodeEmbeddings_Execute_NoSnippets(t *testing.T) {
	logger := slog.Default()
	vectorRepo := newFakeVectorRepository()
	embeddingService := indexing.NewEmbeddingService(nil, vectorRepo)
	snippetRepo := newFakeSnippetRepository()
	trackerFactory := &fakeTrackerFactory{logger: logger}

	handler := NewCreateCodeEmbeddings(embeddingService, snippetRepo, vectorRepo, trackerFactory, logger)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    "abc123def456789012345678901234567890abcd",
	}

	err := handler.Execute(context.Background(), payload)
	require.NoError(t, err)

	assert.Empty(t, vectorRepo.indexed)
}

func TestCreateCodeEmbeddings_Execute_WithSnippets(t *testing.T) {
	logger := slog.Default()
	vectorRepo := newFakeVectorRepository()
	embeddingService := indexing.NewEmbeddingService(nil, vectorRepo)
	snippetRepo := newFakeSnippetRepository()
	trackerFactory := &fakeTrackerFactory{logger: logger}

	commitSHA := "abc123def456789012345678901234567890abcd"
	snippet := indexing.NewSnippet("func main() {}", ".go", nil)
	snippetRepo.snippetsByCommit[commitSHA] = []indexing.Snippet{snippet}

	handler := NewCreateCodeEmbeddings(embeddingService, snippetRepo, vectorRepo, trackerFactory, logger)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    commitSHA,
	}

	err := handler.Execute(context.Background(), payload)
	require.NoError(t, err)

	require.Len(t, vectorRepo.indexed, 1)
	assert.Len(t, vectorRepo.indexed[0].Documents(), 1)
}

func TestCreateCodeEmbeddings_Execute_AlreadyHasEmbeddings(t *testing.T) {
	logger := slog.Default()
	vectorRepo := newFakeVectorRepository()
	embeddingService := indexing.NewEmbeddingService(nil, vectorRepo)
	snippetRepo := newFakeSnippetRepository()
	trackerFactory := &fakeTrackerFactory{logger: logger}

	commitSHA := "abc123def456789012345678901234567890abcd"
	snippet := indexing.NewSnippet("func main() {}", ".go", nil)
	snippetRepo.snippetsByCommit[commitSHA] = []indexing.Snippet{snippet}

	vectorRepo.hasEmbedding[snippet.SHA()] = true

	handler := NewCreateCodeEmbeddings(embeddingService, snippetRepo, vectorRepo, trackerFactory, logger)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    commitSHA,
	}

	err := handler.Execute(context.Background(), payload)
	require.NoError(t, err)

	assert.Empty(t, vectorRepo.indexed)
}

func TestCreateBM25Index_Execute_MissingPayload(t *testing.T) {
	logger := slog.Default()
	bm25Repo := newFakeBM25Repository()
	bm25Service := indexing.NewBM25Service(bm25Repo)
	snippetRepo := newFakeSnippetRepository()
	trackerFactory := &fakeTrackerFactory{logger: logger}

	handler := NewCreateBM25Index(bm25Service, snippetRepo, trackerFactory, logger)

	err := handler.Execute(context.Background(), map[string]any{})
	assert.Error(t, err)
}

func TestCreateCodeEmbeddings_Execute_MissingPayload(t *testing.T) {
	logger := slog.Default()
	vectorRepo := newFakeVectorRepository()
	embeddingService := indexing.NewEmbeddingService(nil, vectorRepo)
	snippetRepo := newFakeSnippetRepository()
	trackerFactory := &fakeTrackerFactory{logger: logger}

	handler := NewCreateCodeEmbeddings(embeddingService, snippetRepo, vectorRepo, trackerFactory, logger)

	err := handler.Execute(context.Background(), map[string]any{})
	assert.Error(t, err)
}
