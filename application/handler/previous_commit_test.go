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
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

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

func newPreviousCommit(
	t *testing.T,
) (
	*handler.PreviousCommit,
	enrichment.EnrichmentStore,
	enrichment.AssociationStore,
	repository.CommitStore,
	repository.FileStore,
) {
	t.Helper()
	db := testdb.New(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	commitStore := persistence.NewCommitStore(db)
	fileStore := persistence.NewFileStore(db)

	enrichSvc := service.NewEnrichment(enrichmentStore, associationStore, nil, nil, nil, nil)

	pc := handler.NewPreviousCommit(enrichSvc, commitStore, fileStore)
	return pc, enrichmentStore, associationStore, commitStore, fileStore
}

func TestDeleteEnrichments(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes old enrichments by type and subtype", func(t *testing.T) {
		pc, enrichmentStore, associationStore, commitStore, _ := newPreviousCommit(t)

		repoID := int64(1)
		seedCommit(t, ctx, commitStore, "old-sha", repoID)
		seedCommit(t, ctx, commitStore, "current-sha", repoID)

		// Seed enrichment on old commit.
		seedEnrichmentForCommit(t, ctx, enrichmentStore, associationStore,
			enrichment.NewSnippetEnrichment("old snippet"), "old-sha")

		// Seed enrichment on current commit (should remain).
		seedEnrichmentForCommit(t, ctx, enrichmentStore, associationStore,
			enrichment.NewSnippetEnrichment("current snippet"), "current-sha")

		err := pc.DeleteEnrichments(ctx, repoID, "current-sha",
			enrichment.TypeDevelopment, enrichment.SubtypeSnippet)
		require.NoError(t, err)

		// Old commit enrichment is gone.
		old, err := enrichmentStore.Find(ctx,
			enrichment.WithCommitSHA("old-sha"),
			enrichment.WithType(enrichment.TypeDevelopment),
			enrichment.WithSubtype(enrichment.SubtypeSnippet),
		)
		require.NoError(t, err)
		assert.Empty(t, old)

		// Current commit enrichment remains.
		current, err := enrichmentStore.Find(ctx,
			enrichment.WithCommitSHA("current-sha"),
			enrichment.WithType(enrichment.TypeDevelopment),
			enrichment.WithSubtype(enrichment.SubtypeSnippet),
		)
		require.NoError(t, err)
		assert.Len(t, current, 1)
	})

	t.Run("no-op on first sync", func(t *testing.T) {
		pc, _, _, commitStore, _ := newPreviousCommit(t)

		repoID := int64(1)
		// Only the current commit exists — no old commits.
		seedCommit(t, ctx, commitStore, "first-sha", repoID)

		err := pc.DeleteEnrichments(ctx, repoID, "first-sha",
			enrichment.TypeDevelopment, enrichment.SubtypeSnippet)
		require.NoError(t, err)
	})

	t.Run("leaves enrichments of other types untouched", func(t *testing.T) {
		pc, enrichmentStore, associationStore, commitStore, _ := newPreviousCommit(t)

		repoID := int64(1)
		seedCommit(t, ctx, commitStore, "old-sha", repoID)
		seedCommit(t, ctx, commitStore, "current-sha", repoID)

		// Seed a snippet (TypeDevelopment/SubtypeSnippet) on old commit.
		seedEnrichmentForCommit(t, ctx, enrichmentStore, associationStore,
			enrichment.NewSnippetEnrichment("old snippet"), "old-sha")

		// Seed a commit description (TypeHistory/SubtypeCommitDescription) on old commit.
		seedEnrichmentForCommit(t, ctx, enrichmentStore, associationStore,
			enrichment.NewEnrichment(enrichment.TypeHistory, enrichment.SubtypeCommitDescription, enrichment.EntityTypeCommit, "desc"),
			"old-sha")

		// Delete only snippets.
		err := pc.DeleteEnrichments(ctx, repoID, "current-sha",
			enrichment.TypeDevelopment, enrichment.SubtypeSnippet)
		require.NoError(t, err)

		// Commit description on old commit remains.
		remaining, err := enrichmentStore.Find(ctx,
			enrichment.WithCommitSHA("old-sha"),
			enrichment.WithType(enrichment.TypeHistory),
		)
		require.NoError(t, err)
		assert.Len(t, remaining, 1)
	})
}

func TestDeleteFiles(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes files from old commits", func(t *testing.T) {
		pc, _, _, commitStore, fileStore := newPreviousCommit(t)

		repoID := int64(1)
		seedCommit(t, ctx, commitStore, "old-sha", repoID)
		seedCommit(t, ctx, commitStore, "current-sha", repoID)

		// Seed files on old and current commits.
		_, err := fileStore.Save(ctx, repository.NewFile("old-sha", "main.go", "go", 100))
		require.NoError(t, err)
		_, err = fileStore.Save(ctx, repository.NewFile("current-sha", "main.go", "go", 100))
		require.NoError(t, err)

		err = pc.DeleteFiles(ctx, repoID, "current-sha")
		require.NoError(t, err)

		// Old files deleted.
		oldFiles, err := fileStore.Find(ctx, repository.WithCommitSHA("old-sha"))
		require.NoError(t, err)
		assert.Empty(t, oldFiles)

		// Current files remain.
		currentFiles, err := fileStore.Find(ctx, repository.WithCommitSHA("current-sha"))
		require.NoError(t, err)
		assert.Len(t, currentFiles, 1)
	})

	t.Run("no-op on first sync", func(t *testing.T) {
		pc, _, _, commitStore, fileStore := newPreviousCommit(t)

		repoID := int64(1)
		seedCommit(t, ctx, commitStore, "first-sha", repoID)

		_, err := fileStore.Save(ctx, repository.NewFile("first-sha", "main.go", "go", 100))
		require.NoError(t, err)

		err = pc.DeleteFiles(ctx, repoID, "first-sha")
		require.NoError(t, err)

		// File still exists.
		files, err := fileStore.Find(ctx, repository.WithCommitSHA("first-sha"))
		require.NoError(t, err)
		assert.Len(t, files, 1)
	})
}

func TestPreviousCommitHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("calls inner then deletes old enrichments", func(t *testing.T) {
		pc, enrichmentStore, associationStore, commitStore, _ := newPreviousCommit(t)

		repoID := int64(1)
		seedCommit(t, ctx, commitStore, "old-sha", repoID)
		seedCommit(t, ctx, commitStore, "current-sha", repoID)

		seedEnrichmentForCommit(t, ctx, enrichmentStore, associationStore,
			enrichment.NewSnippetEnrichment("old"), "old-sha")

		inner := &fakeHandler{}
		wrapped := pc.Handler(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, inner)

		payload := map[string]any{
			"repository_id": repoID,
			"commit_sha":    "current-sha",
		}
		err := wrapped.Execute(ctx, payload)
		require.NoError(t, err)
		assert.True(t, inner.called)

		old, err := enrichmentStore.Find(ctx,
			enrichment.WithCommitSHA("old-sha"),
			enrichment.WithType(enrichment.TypeDevelopment),
			enrichment.WithSubtype(enrichment.SubtypeSnippet),
		)
		require.NoError(t, err)
		assert.Empty(t, old)
	})

	t.Run("propagates inner error without deleting", func(t *testing.T) {
		pc, enrichmentStore, associationStore, commitStore, _ := newPreviousCommit(t)

		repoID := int64(1)
		seedCommit(t, ctx, commitStore, "old-sha", repoID)
		seedCommit(t, ctx, commitStore, "current-sha", repoID)

		seedEnrichmentForCommit(t, ctx, enrichmentStore, associationStore,
			enrichment.NewSnippetEnrichment("old"), "old-sha")

		inner := &fakeHandler{err: errors.New("inner failed")}
		wrapped := pc.Handler(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, inner)

		payload := map[string]any{
			"repository_id": repoID,
			"commit_sha":    "current-sha",
		}
		err := wrapped.Execute(ctx, payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inner failed")

		// Old enrichment should still exist.
		old, err := enrichmentStore.Find(ctx,
			enrichment.WithCommitSHA("old-sha"),
			enrichment.WithType(enrichment.TypeDevelopment),
			enrichment.WithSubtype(enrichment.SubtypeSnippet),
		)
		require.NoError(t, err)
		assert.Len(t, old, 1)
	})
}
