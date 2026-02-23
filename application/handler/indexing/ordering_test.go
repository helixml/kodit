package indexing

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingEmbedding captures the IndexRequest it receives so tests can
// inspect the document order that was sent to the embedding service.
type recordingEmbedding struct {
	requests []search.IndexRequest
}

func (r *recordingEmbedding) Index(_ context.Context, req search.IndexRequest, _ ...search.IndexOption) error {
	r.requests = append(r.requests, req)
	return nil
}

func (r *recordingEmbedding) Find(_ context.Context, _ string, _ ...repository.Option) ([]search.Result, error) {
	return nil, nil
}

func (r *recordingEmbedding) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return false, nil
}

func (r *recordingEmbedding) documents() []search.Document {
	var docs []search.Document
	for _, req := range r.requests {
		docs = append(docs, req.Documents()...)
	}
	return docs
}

// emptyEmbeddingStore returns no existing embeddings so every enrichment
// appears "new" and reaches the embedding service.
type emptyEmbeddingStore struct{}

func (e *emptyEmbeddingStore) SaveAll(_ context.Context, _ []search.Embedding) error {
	return nil
}

func (e *emptyEmbeddingStore) Find(_ context.Context, _ ...repository.Option) ([]search.Embedding, error) {
	return nil, nil
}

func (e *emptyEmbeddingStore) Search(_ context.Context, _ ...repository.Option) ([]search.Result, error) {
	return nil, nil
}

func (e *emptyEmbeddingStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return false, nil
}

func (e *emptyEmbeddingStore) DeleteBy(_ context.Context, _ ...repository.Option) error {
	return nil
}

type fakeTracker struct{}

func (f *fakeTracker) SetTotal(_ context.Context, _ int)             {}
func (f *fakeTracker) SetCurrent(_ context.Context, _ int, _ string) {}
func (f *fakeTracker) Skip(_ context.Context, _ string)              {}
func (f *fakeTracker) Fail(_ context.Context, _ string)              {}
func (f *fakeTracker) Complete(_ context.Context)                    {}

type fakeTrackerFactory struct{}

func (f *fakeTrackerFactory) ForOperation(_ task.Operation, _ task.TrackableType, _ int64) handler.Tracker {
	return &fakeTracker{}
}

// failingAssociationStore wraps a real AssociationStore and returns an error
// from Find after a configured number of successful calls. This simulates a
// transient failure that only surfaces in the second loop of filterNewEnrichments.
type failingAssociationStore struct {
	enrichment.AssociationStore
	failAfter int32
	calls     atomic.Int32
	err       error
}

func (f *failingAssociationStore) Find(ctx context.Context, opts ...repository.Option) ([]enrichment.Association, error) {
	if f.calls.Add(1) > f.failAfter {
		return nil, f.err
	}
	return f.AssociationStore.Find(ctx, opts...)
}

func TestCreateSummaryEmbeddings_FilterPropagatesError(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	db := testdb.New(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	realAssocStore := persistence.NewAssociationStore(db)

	commitSHA := "eee555fff666"

	// Create one summary enrichment with a snippet association.
	snippet, err := enrichmentStore.Save(ctx, enrichment.NewSnippetEnrichment("snippet content"))
	require.NoError(t, err)
	snippetSHA := strconv.FormatInt(snippet.ID(), 10)

	summary, err := enrichmentStore.Save(ctx, enrichment.NewSnippetSummary("summary content"))
	require.NoError(t, err)

	_, err = realAssocStore.Save(ctx, enrichment.CommitAssociation(summary.ID(), commitSHA))
	require.NoError(t, err)
	_, err = realAssocStore.Save(ctx, enrichment.SnippetAssociation(summary.ID(), snippetSHA))
	require.NoError(t, err)

	// The first loop in filterNewEnrichments calls Find once (succeeds).
	// The second loop calls Find again — this time it must fail.
	injectedErr := errors.New("transient association lookup failure")
	fakeAssocStore := &failingAssociationStore{
		AssociationStore: realAssocStore,
		failAfter:        1,
		err:              injectedErr,
	}

	rec := &recordingEmbedding{}
	h, err := NewCreateSummaryEmbeddings(
		handler.VectorIndex{Embedding: rec, Store: &emptyEmbeddingStore{}},
		enrichmentStore,
		fakeAssocStore,
		&fakeTrackerFactory{},
		logger,
	)
	require.NoError(t, err)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    commitSHA,
	}
	err = h.Execute(ctx, payload)
	require.Error(t, err, "error from findSnippetSHA in the filter's second loop must propagate")
	assert.ErrorIs(t, err, injectedErr)
}

func TestCreateCodeEmbeddings_OrdersByID(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	db := testdb.New(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)

	// Create enrichments; auto-increment IDs guarantee ascending order.
	commitSHA := "aaa111bbb222"
	contents := []string{"third item", "first item", "second item"}
	ids := make([]int64, len(contents))
	for i, c := range contents {
		saved, err := enrichmentStore.Save(ctx, enrichment.NewSnippetEnrichment(c))
		require.NoError(t, err)
		ids[i] = saved.ID()
		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA))
		require.NoError(t, err)
	}

	rec := &recordingEmbedding{}
	h, err := NewCreateCodeEmbeddings(
		handler.VectorIndex{Embedding: rec, Store: &emptyEmbeddingStore{}},
		enrichmentStore,
		&fakeTrackerFactory{},
		logger,
	)
	require.NoError(t, err)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    commitSHA,
	}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	docs := rec.documents()
	require.Len(t, docs, len(contents))

	// Documents must arrive in ascending enrichment ID order.
	for i, doc := range docs {
		assert.Equal(t, strconv.FormatInt(ids[i], 10), doc.SnippetID(),
			"document %d should have enrichment ID %d", i, ids[i])
	}
}

func TestCreateSummaryEmbeddings_OrdersByID(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	db := testdb.New(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)

	commitSHA := "ccc333ddd444"

	// Create snippet enrichments and their summaries.
	// Each summary is linked to a snippet via a SnippetAssociation, which
	// is how the handler resolves the snippet SHA for the embedding document.
	snippetSHAs := make([]string, 3)
	summaryIDs := make([]int64, 3)
	for i := 0; i < 3; i++ {
		// Create the snippet first (gives us an ID to use as the "snippet SHA").
		snippet, err := enrichmentStore.Save(ctx, enrichment.NewSnippetEnrichment("snippet "+strconv.Itoa(i)))
		require.NoError(t, err)
		snippetSHAs[i] = strconv.FormatInt(snippet.ID(), 10)

		// Create the summary enrichment.
		summary, err := enrichmentStore.Save(ctx, enrichment.NewSnippetSummary("summary "+strconv.Itoa(i)))
		require.NoError(t, err)
		summaryIDs[i] = summary.ID()

		// Link summary → commit.
		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(summary.ID(), commitSHA))
		require.NoError(t, err)

		// Link summary → snippet (this is what findSnippetSHA looks up).
		_, err = associationStore.Save(ctx, enrichment.SnippetAssociation(summary.ID(), snippetSHAs[i]))
		require.NoError(t, err)
	}

	rec := &recordingEmbedding{}
	h, err := NewCreateSummaryEmbeddings(
		handler.VectorIndex{Embedding: rec, Store: &emptyEmbeddingStore{}},
		enrichmentStore,
		associationStore,
		&fakeTrackerFactory{},
		logger,
	)
	require.NoError(t, err)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    commitSHA,
	}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	docs := rec.documents()
	require.Len(t, docs, 3)

	// Documents must arrive in ascending summary enrichment ID order,
	// keyed by their associated snippet SHA.
	for i, doc := range docs {
		assert.Equal(t, snippetSHAs[i], doc.SnippetID(),
			"document %d should reference snippet SHA %s", i, snippetSHAs[i])
	}
}
