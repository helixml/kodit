package commit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
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

	enrichmentSvc := service.NewEnrichment(stores.enrichments, stores.associations, nil, nil, nil, nil)
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
