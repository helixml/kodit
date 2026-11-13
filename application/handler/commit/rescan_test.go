package commit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTracker struct{}

func (f *fakeTracker) SetTotal(_ context.Context, _ int)             {}
func (f *fakeTracker) SetCurrent(_ context.Context, _ int, _ string) {}
func (f *fakeTracker) Skip(_ context.Context, _ string)              {}
func (f *fakeTracker) Fail(_ context.Context, _ string)              {}
func (f *fakeTracker) Complete(_ context.Context)                    {}

type fakeTrackerFactory struct{}

func (f *fakeTrackerFactory) ForOperation(_ task.Operation, _ map[string]any) handler.Tracker {
	return &fakeTracker{}
}

func newRescanHandler(t *testing.T) (*Rescan, rescanStores) {
	t.Helper()
	db := testdb.New(t)
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)

	stores := rescanStores{
		enrichments:  persistence.NewEnrichmentStore(db),
		associations: persistence.NewAssociationStore(db),
		commits:      persistence.NewCommitStore(db),
		files:        persistence.NewFileStore(db),
		statuses:     persistence.NewStatusStore(db),
		repos:        persistence.NewRepositoryStore(db),
	}

	enrichmentSvc := service.NewEnrichment(stores.enrichments, stores.associations, nil, nil, nil, nil, nil)
	h := NewRescan(enrichmentSvc, stores.associations, stores.commits, stores.files, stores.statuses, &fakeTrackerFactory{}, logger)

	return h, stores
}

type rescanStores struct {
	enrichments  persistence.EnrichmentStore
	associations persistence.AssociationStore
	commits      persistence.CommitStore
	files        persistence.FileStore
	statuses     persistence.StatusStore
	repos        persistence.RepositoryStore
}

func seedCommit(t *testing.T, ctx context.Context, stores rescanStores, repoID int64, commitSHA string) {
	t.Helper()

	repo, err := repository.NewRepository("https://example.com/test/repo.git")
	require.NoError(t, err)
	repo = repo.WithID(repoID)
	_, err = stores.repos.Save(ctx, repo)
	require.NoError(t, err)

	author := repository.NewAuthor("test", "test@example.com")
	now := time.Now()
	commit := repository.NewCommit(commitSHA, repoID, "test commit", author, author, now, now)
	_, err = stores.commits.Save(ctx, commit)
	require.NoError(t, err)
}

func TestRescan_DeletesOldStatuses(t *testing.T) {
	ctx := context.Background()

	h, stores := newRescanHandler(t)

	repoID := int64(42)
	commitSHA := "abc123def456"

	seedCommit(t, ctx, stores, repoID, commitSHA)

	// Seed a failed status for this repository.
	failedStatus := task.NewStatus(
		task.OperationScanCommit,
		nil,
		task.TrackableTypeRepository,
		repoID,
	)
	failedStatus = failedStatus.Fail("something went wrong")
	_, err := stores.statuses.Save(ctx, failedStatus)
	require.NoError(t, err)

	// Verify the status exists.
	statuses, err := stores.statuses.Find(ctx, task.WithTrackable(task.TrackableTypeRepository, repoID)...)
	require.NoError(t, err)
	assert.Len(t, statuses, 1)

	payload := map[string]any{
		"repository_id": repoID,
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Old statuses should be gone.
	statuses, err = stores.statuses.Find(ctx, task.WithTrackable(task.TrackableTypeRepository, repoID)...)
	require.NoError(t, err)
	assert.Empty(t, statuses)

	// Commit should be deleted.
	exists, err := stores.commits.Exists(ctx, repository.WithRepoID(repoID), repository.WithSHA(commitSHA))
	require.NoError(t, err)
	assert.False(t, exists)
}

func seedChunk(t *testing.T, ctx context.Context, stores rescanStores, commitSHA string, repoIDs ...string) int64 {
	t.Helper()
	e, err := stores.enrichments.Save(ctx, enrichment.NewEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeChunk, enrichment.EntityTypeCommit, "chunk"))
	require.NoError(t, err)
	_, err = stores.associations.Save(ctx, enrichment.CommitAssociation(e.ID(), commitSHA))
	require.NoError(t, err)
	for _, repoID := range repoIDs {
		_, err = stores.associations.Save(ctx, enrichment.RepositoryAssociation(e.ID(), repoID))
		require.NoError(t, err)
	}
	return e.ID()
}

func repositoryOwners(t *testing.T, ctx context.Context, stores rescanStores, enrichmentID int64) []string {
	t.Helper()
	assocs, err := stores.associations.Find(ctx,
		enrichment.WithEnrichmentIDIn([]int64{enrichmentID}),
		enrichment.WithEntityType(enrichment.EntityTypeRepository),
	)
	require.NoError(t, err)
	owners := make([]string, 0, len(assocs))
	for _, a := range assocs {
		owners = append(owners, a.EntityID())
	}
	return owners
}

// TestRescan_RefusesCommitOfAnotherRepository checks that a repository cannot
// clear a commit recorded for another repository.
func TestRescan_RefusesCommitOfAnotherRepository(t *testing.T) {
	ctx := context.Background()
	h, stores := newRescanHandler(t)

	seedCommit(t, ctx, stores, 1, "shared123")
	chunk := seedChunk(t, ctx, stores, "shared123", "1")

	err := h.Execute(ctx, map[string]any{"repository_id": int64(2), "commit_sha": "shared123"})
	require.Error(t, err)

	exists, err := stores.enrichments.Exists(ctx, repository.WithID(chunk))
	require.NoError(t, err)
	assert.True(t, exists, "repository 1's chunk must survive repository 2's rescan")
	exists, err = stores.commits.Exists(ctx, repository.WithRepoID(1), repository.WithSHA("shared123"))
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestRescan_KeepsEnrichmentsSharedWithAnotherRepository checks that a
// rescan deletes only what belongs to the rescanning repository alone.
func TestRescan_KeepsEnrichmentsSharedWithAnotherRepository(t *testing.T) {
	ctx := context.Background()
	h, stores := newRescanHandler(t)

	seedCommit(t, ctx, stores, 1, "shared123")
	shared := seedChunk(t, ctx, stores, "shared123", "1", "2")
	own := seedChunk(t, ctx, stores, "shared123", "1")

	require.NoError(t, h.Execute(ctx, map[string]any{"repository_id": int64(1), "commit_sha": "shared123"}))

	exists, err := stores.enrichments.Exists(ctx, repository.WithID(own))
	require.NoError(t, err)
	assert.False(t, exists, "a chunk only repository 1 owns is deleted")

	exists, err = stores.enrichments.Exists(ctx, repository.WithID(shared))
	require.NoError(t, err)
	assert.True(t, exists, "a chunk repository 2 also owns is kept")
	assert.Equal(t, []string{"2"}, repositoryOwners(t, ctx, stores, shared))

	exists, err = stores.commits.Exists(ctx, repository.WithSHA("shared123"))
	require.NoError(t, err)
	assert.True(t, exists, "the commit row stays while another repository shares the commit")
}
