package handler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

// noopEmbeddingStore satisfies search.EmbeddingStore with no-op methods.
type noopEmbeddingStore struct{}

func (noopEmbeddingStore) SaveAll(_ context.Context, _ []search.Embedding) error { return nil }
func (noopEmbeddingStore) Find(_ context.Context, _ ...repository.Option) ([]search.Embedding, error) {
	return nil, nil
}
func (noopEmbeddingStore) Search(_ context.Context, _ ...repository.Option) ([]search.Result, error) {
	return nil, nil
}
func (noopEmbeddingStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return false, nil
}
func (noopEmbeddingStore) DeleteBy(_ context.Context, _ ...repository.Option) error { return nil }

// fakeHandler records whether Execute was called and can return an error.
type fakeHandler struct {
	called bool
	err    error
}

func (f *fakeHandler) Execute(_ context.Context, _ map[string]any) error {
	f.called = true
	return f.err
}

func seedCommit(t *testing.T, ctx context.Context, store repository.CommitStore, sha string, repoID int64) {
	t.Helper()
	now := time.Now()
	author := repository.NewAuthor("test", "test@test.com")
	_, err := store.Save(ctx, repository.NewCommit(sha, repoID, "msg", author, author, now, now))
	require.NoError(t, err)
}

func seedEnrichmentForCommit(
	t *testing.T,
	ctx context.Context,
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	e enrichment.Enrichment,
	sha string,
) enrichment.Enrichment {
	t.Helper()
	saved, err := enrichmentStore.Save(ctx, e)
	require.NoError(t, err)
	_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), sha))
	require.NoError(t, err)
	return saved
}

type testStores struct {
	enrichments  enrichment.EnrichmentStore
	associations enrichment.AssociationStore
	commits      repository.CommitStore
	files        repository.FileStore
	enrichSvc    *service.Enrichment
}

func newTestStores(t *testing.T) testStores {
	t.Helper()
	db := testdb.New(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	commitStore := persistence.NewCommitStore(db)
	fileStore := persistence.NewFileStore(db)

	enrichSvc := service.NewEnrichment(enrichmentStore, associationStore, nil, nil, nil, noopEmbeddingStore{}, nil)

	return testStores{
		enrichments:  enrichmentStore,
		associations: associationStore,
		commits:      commitStore,
		files:        fileStore,
		enrichSvc:    enrichSvc,
	}
}

func TestEnrichmentCleanup(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes old enrichments by type and subtype", func(t *testing.T) {
		s := newTestStores(t)
		cleanup := handler.NewEnrichmentCleanup(s.enrichSvc, s.commits,
			enrichment.TypeDevelopment, enrichment.SubtypeSnippet)

		repoID := int64(1)
		seedCommit(t, ctx, s.commits, "old-sha", repoID)
		seedCommit(t, ctx, s.commits, "current-sha", repoID)

		seedEnrichmentForCommit(t, ctx, s.enrichments, s.associations,
			enrichment.NewSnippetEnrichment("old snippet"), "old-sha")

		seedEnrichmentForCommit(t, ctx, s.enrichments, s.associations,
			enrichment.NewSnippetEnrichment("current snippet"), "current-sha")

		err := cleanup.Clean(ctx, repoID, "current-sha")
		require.NoError(t, err)

		old, err := s.enrichments.Find(ctx,
			enrichment.WithCommitSHA("old-sha"),
			enrichment.WithType(enrichment.TypeDevelopment),
			enrichment.WithSubtype(enrichment.SubtypeSnippet),
		)
		require.NoError(t, err)
		assert.Empty(t, old)

		current, err := s.enrichments.Find(ctx,
			enrichment.WithCommitSHA("current-sha"),
			enrichment.WithType(enrichment.TypeDevelopment),
			enrichment.WithSubtype(enrichment.SubtypeSnippet),
		)
		require.NoError(t, err)
		assert.Len(t, current, 1)
	})

	t.Run("no-op on first sync", func(t *testing.T) {
		s := newTestStores(t)
		cleanup := handler.NewEnrichmentCleanup(s.enrichSvc, s.commits,
			enrichment.TypeDevelopment, enrichment.SubtypeSnippet)

		repoID := int64(1)
		seedCommit(t, ctx, s.commits, "first-sha", repoID)

		err := cleanup.Clean(ctx, repoID, "first-sha")
		require.NoError(t, err)
	})

	t.Run("leaves enrichments of other types untouched", func(t *testing.T) {
		s := newTestStores(t)
		cleanup := handler.NewEnrichmentCleanup(s.enrichSvc, s.commits,
			enrichment.TypeDevelopment, enrichment.SubtypeSnippet)

		repoID := int64(1)
		seedCommit(t, ctx, s.commits, "old-sha", repoID)
		seedCommit(t, ctx, s.commits, "current-sha", repoID)

		seedEnrichmentForCommit(t, ctx, s.enrichments, s.associations,
			enrichment.NewSnippetEnrichment("old snippet"), "old-sha")

		seedEnrichmentForCommit(t, ctx, s.enrichments, s.associations,
			enrichment.NewEnrichment(enrichment.TypeHistory, enrichment.SubtypeCommitDescription, enrichment.EntityTypeCommit, "desc"),
			"old-sha")

		err := cleanup.Clean(ctx, repoID, "current-sha")
		require.NoError(t, err)

		remaining, err := s.enrichments.Find(ctx,
			enrichment.WithCommitSHA("old-sha"),
			enrichment.WithType(enrichment.TypeHistory),
		)
		require.NoError(t, err)
		assert.Len(t, remaining, 1)
	})
}

func TestFileCleanup(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes files from old commits", func(t *testing.T) {
		s := newTestStores(t)
		cleanup := handler.NewFileCleanup(s.commits, s.files)

		repoID := int64(1)
		seedCommit(t, ctx, s.commits, "old-sha", repoID)
		seedCommit(t, ctx, s.commits, "current-sha", repoID)

		_, err := s.files.Save(ctx, repository.NewFile("old-sha", "main.go", "go", 100))
		require.NoError(t, err)
		_, err = s.files.Save(ctx, repository.NewFile("current-sha", "main.go", "go", 100))
		require.NoError(t, err)

		err = cleanup.Clean(ctx, repoID, "current-sha")
		require.NoError(t, err)

		oldFiles, err := s.files.Find(ctx, repository.WithCommitSHA("old-sha"))
		require.NoError(t, err)
		assert.Empty(t, oldFiles)

		currentFiles, err := s.files.Find(ctx, repository.WithCommitSHA("current-sha"))
		require.NoError(t, err)
		assert.Len(t, currentFiles, 1)
	})

	t.Run("no-op on first sync", func(t *testing.T) {
		s := newTestStores(t)
		cleanup := handler.NewFileCleanup(s.commits, s.files)

		repoID := int64(1)
		seedCommit(t, ctx, s.commits, "first-sha", repoID)

		_, err := s.files.Save(ctx, repository.NewFile("first-sha", "main.go", "go", 100))
		require.NoError(t, err)

		err = cleanup.Clean(ctx, repoID, "first-sha")
		require.NoError(t, err)

		files, err := s.files.Find(ctx, repository.WithCommitSHA("first-sha"))
		require.NoError(t, err)
		assert.Len(t, files, 1)
	})
}

func TestWithCleanup(t *testing.T) {
	ctx := context.Background()

	t.Run("calls inner then cleans old enrichments", func(t *testing.T) {
		s := newTestStores(t)
		cleanup := handler.NewEnrichmentCleanup(s.enrichSvc, s.commits,
			enrichment.TypeDevelopment, enrichment.SubtypeSnippet)

		repoID := int64(1)
		seedCommit(t, ctx, s.commits, "old-sha", repoID)
		seedCommit(t, ctx, s.commits, "current-sha", repoID)

		seedEnrichmentForCommit(t, ctx, s.enrichments, s.associations,
			enrichment.NewSnippetEnrichment("old"), "old-sha")

		inner := &fakeHandler{}
		wrapped := handler.WithCleanup(inner, cleanup)

		payload := map[string]any{
			"repository_id": repoID,
			"commit_sha":    "current-sha",
		}
		err := wrapped.Execute(ctx, payload)
		require.NoError(t, err)
		assert.True(t, inner.called)

		old, err := s.enrichments.Find(ctx,
			enrichment.WithCommitSHA("old-sha"),
			enrichment.WithType(enrichment.TypeDevelopment),
			enrichment.WithSubtype(enrichment.SubtypeSnippet),
		)
		require.NoError(t, err)
		assert.Empty(t, old)
	})

	t.Run("propagates inner error without cleaning", func(t *testing.T) {
		s := newTestStores(t)
		cleanup := handler.NewEnrichmentCleanup(s.enrichSvc, s.commits,
			enrichment.TypeDevelopment, enrichment.SubtypeSnippet)

		repoID := int64(1)
		seedCommit(t, ctx, s.commits, "old-sha", repoID)
		seedCommit(t, ctx, s.commits, "current-sha", repoID)

		seedEnrichmentForCommit(t, ctx, s.enrichments, s.associations,
			enrichment.NewSnippetEnrichment("old"), "old-sha")

		inner := &fakeHandler{err: errors.New("inner failed")}
		wrapped := handler.WithCleanup(inner, cleanup)

		payload := map[string]any{
			"repository_id": repoID,
			"commit_sha":    "current-sha",
		}
		err := wrapped.Execute(ctx, payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inner failed")

		old, err := s.enrichments.Find(ctx,
			enrichment.WithCommitSHA("old-sha"),
			enrichment.WithType(enrichment.TypeDevelopment),
			enrichment.WithSubtype(enrichment.SubtypeSnippet),
		)
		require.NoError(t, err)
		assert.Len(t, old, 1)
	})
}
