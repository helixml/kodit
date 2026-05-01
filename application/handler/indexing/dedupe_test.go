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
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

// dedupingEmbeddingStore is a fake search.Store that simulates
// PostgreSQL semantics: it returns matches for snippet IDs in the IN
// condition, and honours the LIMIT option by truncating the result set.
// This mirrors how the production store responds, so tests can exercise
// the dedupe logic for repos with more than MaxSnippetIDsPerFind snippets.
type dedupingEmbeddingStore struct {
	mu       sync.Mutex
	existing map[string]struct{}
	saved    []search.Document
}

func (s *dedupingEmbeddingStore) Index(_ context.Context, docs []search.Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = append(s.saved, docs...)
	for _, d := range docs {
		s.existing[d.SnippetID()] = struct{}{}
	}
	return nil
}

func (s *dedupingEmbeddingStore) Find(_ context.Context, options ...repository.Option) ([]search.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := repository.Build(options...)
	ids := search.SnippetIDsFrom(q)
	var result []search.Result
	for _, id := range ids {
		if _, ok := s.existing[id]; ok {
			result = append(result, search.NewResult(id, 0))
		}
	}
	if limit := q.LimitValue(); limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *dedupingEmbeddingStore) Count(_ context.Context, _ ...repository.Option) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int64(len(s.existing)), nil
}

func (s *dedupingEmbeddingStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.existing) > 0, nil
}

func (s *dedupingEmbeddingStore) DeleteBy(_ context.Context, _ ...repository.Option) error {
	return nil
}

func TestCreateCodeEmbeddings_DeduplicatesBeyondMaxSnippetIDsPerFind(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.ErrorLevel)

	db := testdb.New(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)

	commitSHA := "fff666aaa777"

	// Create more chunk enrichments than the per-Find cap, all attached to
	// the same commit. The point is to exercise dedupe across a number of
	// snippets that exceeds MaxSnippetIDsPerFind in a single round.
	total := search.MaxSnippetIDsPerFind + 25
	existing := make(map[string]struct{}, total)
	for i := range total {
		saved, err := enrichmentStore.Save(ctx, enrichment.NewChunkEnrichment("chunk "+strconv.Itoa(i)))
		require.NoError(t, err)
		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA))
		require.NoError(t, err)
		existing[strconv.FormatInt(saved.ID(), 10)] = struct{}{}
	}

	store := &dedupingEmbeddingStore{existing: existing}
	rec := &recordingEmbedding{}
	h, err := NewCreateCodeEmbeddings(
		handler.VectorIndex{Embedding: rec, Store: store},
		enrichmentStore,
		&fakeTrackerFactory{},
		logger,
		enrichment.SubtypeChunk,
	)
	require.NoError(t, err)

	payload := map[string]any{
		"repository_id": int64(1),
		"commit_sha":    commitSHA,
	}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	assert.Empty(t, rec.documents(),
		"all %d enrichments already have embeddings; nothing should be re-embedded", total)
	assert.Empty(t, store.saved,
		"no new embeddings should be saved when all enrichments already exist")
}

func TestCreateSummaryEmbeddings_DeduplicatesBeyondMaxSnippetIDsPerFind(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(zerolog.NewTestWriter(t)).Level(zerolog.ErrorLevel)

	db := testdb.New(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)

	commitSHA := "ggg777bbb888"

	total := search.MaxSnippetIDsPerFind + 25
	existing := make(map[string]struct{}, total)
	for i := range total {
		// Create snippet to act as the snippet ID target.
		snippet, err := enrichmentStore.Save(ctx, enrichment.NewSnippetEnrichment("snippet "+strconv.Itoa(i)))
		require.NoError(t, err)
		snippetSHA := strconv.FormatInt(snippet.ID(), 10)

		summary, err := enrichmentStore.Save(ctx, enrichment.NewSnippetSummary("summary "+strconv.Itoa(i)))
		require.NoError(t, err)
		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(summary.ID(), commitSHA))
		require.NoError(t, err)
		_, err = associationStore.Save(ctx, enrichment.SnippetAssociation(summary.ID(), snippetSHA))
		require.NoError(t, err)

		existing[snippetSHA] = struct{}{}
	}

	store := &dedupingEmbeddingStore{existing: existing}
	rec := &recordingEmbedding{}
	h, err := NewCreateSummaryEmbeddings(
		handler.VectorIndex{Embedding: rec, Store: store},
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

	assert.Empty(t, rec.documents(),
		"all %d summary enrichments already have embeddings; nothing should be re-embedded", total)
	assert.Empty(t, store.saved,
		"no new summary embeddings should be saved when all enrichments already exist")
}
