package indexing

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

// dedupingBM25Store is a fake search.BM25Store that simulates the
// production stores: it tracks already-indexed snippet IDs and reports
// them via ExistingIDs.
type dedupingBM25Store struct {
	mu       sync.Mutex
	existing map[string]struct{}
	indexed  []search.Document
}

func (s *dedupingBM25Store) Index(_ context.Context, req search.IndexRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, doc := range req.Documents() {
		if _, ok := s.existing[doc.SnippetID()]; ok {
			continue
		}
		s.indexed = append(s.indexed, doc)
		s.existing[doc.SnippetID()] = struct{}{}
	}
	return nil
}

func (s *dedupingBM25Store) Find(_ context.Context, _ ...repository.Option) ([]search.Result, error) {
	return nil, nil
}

func (s *dedupingBM25Store) ExistingIDs(_ context.Context, ids []string) (map[string]struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := s.existing[id]; ok {
			result[id] = struct{}{}
		}
	}
	return result, nil
}

func (s *dedupingBM25Store) DeleteBy(_ context.Context, _ ...repository.Option) error {
	return nil
}

// recordingTracker captures Skip and SetCurrent calls so tests can
// verify that the handler emits the right state transition.
type recordingTracker struct {
	skipped     bool
	skipMessage string
	currentSet  bool
	currentN    int
}

func (r *recordingTracker) SetTotal(_ context.Context, _ int) {}
func (r *recordingTracker) SetCurrent(_ context.Context, n int, _ string) {
	r.currentSet = true
	r.currentN = n
}
func (r *recordingTracker) Skip(_ context.Context, msg string) {
	r.skipped = true
	r.skipMessage = msg
}
func (r *recordingTracker) Fail(_ context.Context, _ string) {}
func (r *recordingTracker) Complete(_ context.Context)       {}

type recordingTrackerFactory struct {
	tracker *recordingTracker
}

func (f *recordingTrackerFactory) ForOperation(_ task.Operation, _ map[string]any) handler.Tracker {
	return f.tracker
}

func TestCreateBM25Index_SkipsWhenAllEnrichmentsAlreadyIndexed(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.ErrorLevel)

	db := testdb.New(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)

	commitSHA := "stable000111"

	// Create chunk enrichments attached to the commit, and pre-populate the
	// BM25 store so every snippet is already indexed. A real instance hitting
	// a stable commit on every cycle is the scenario from issue #555.
	existing := make(map[string]struct{})
	for i := range 5 {
		saved, err := enrichmentStore.Save(ctx, enrichment.NewChunkEnrichment("chunk "+strconv.Itoa(i)))
		require.NoError(t, err)
		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA))
		require.NoError(t, err)
		existing[strconv.FormatInt(saved.ID(), 10)] = struct{}{}
	}

	store := &dedupingBM25Store{existing: existing}
	bm25Service, err := domainservice.NewBM25(store)
	require.NoError(t, err)

	tracker := &recordingTracker{}
	h := NewCreateBM25Index(
		bm25Service,
		enrichmentStore,
		&recordingTrackerFactory{tracker: tracker},
		logger,
		enrichment.SubtypeChunk,
	)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    commitSHA,
	}
	require.NoError(t, h.Execute(ctx, payload))

	assert.True(t, tracker.skipped,
		"handler must emit Skip when every enrichment already has a BM25 entry")
	assert.False(t, tracker.currentSet,
		"handler must not emit SetCurrent (which marks the operation completed) when nothing was indexed")
	assert.Empty(t, store.indexed,
		"no new documents should be sent to the BM25 store when all are duplicates")
}

func TestCreateBM25Index_OnlyIndexesNewEnrichments(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.ErrorLevel)

	db := testdb.New(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)

	commitSHA := "mixedcafebab1"

	// Five enrichments: the first three are already in the BM25 store,
	// the last two are new and must be indexed.
	existing := make(map[string]struct{})
	allIDs := make([]string, 0, 5)
	for i := range 5 {
		saved, err := enrichmentStore.Save(ctx, enrichment.NewChunkEnrichment("chunk "+strconv.Itoa(i)))
		require.NoError(t, err)
		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA))
		require.NoError(t, err)
		id := strconv.FormatInt(saved.ID(), 10)
		allIDs = append(allIDs, id)
		if i < 3 {
			existing[id] = struct{}{}
		}
	}

	store := &dedupingBM25Store{existing: existing}
	bm25Service, err := domainservice.NewBM25(store)
	require.NoError(t, err)

	tracker := &recordingTracker{}
	h := NewCreateBM25Index(
		bm25Service,
		enrichmentStore,
		&recordingTrackerFactory{tracker: tracker},
		logger,
		enrichment.SubtypeChunk,
	)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    commitSHA,
	}
	require.NoError(t, h.Execute(ctx, payload))

	assert.False(t, tracker.skipped,
		"handler must not Skip when there are new enrichments to index")
	assert.Len(t, store.indexed, 2,
		"only the two new enrichments should reach the BM25 store")

	indexedIDs := make(map[string]struct{}, len(store.indexed))
	for _, doc := range store.indexed {
		indexedIDs[doc.SnippetID()] = struct{}{}
	}
	assert.Contains(t, indexedIDs, allIDs[3])
	assert.Contains(t, indexedIDs, allIDs[4])
}
