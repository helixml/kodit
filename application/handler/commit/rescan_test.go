package commit

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/database"
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

func (f *fakeTrackerFactory) ForOperation(_ task.Operation, _ task.TrackableType, _ int64) handler.Tracker {
	return &fakeTracker{}
}

func openTestDB(t *testing.T) database.Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()
	db, err := database.NewDatabase(ctx, "sqlite:///"+dbPath)
	require.NoError(t, err)
	require.NoError(t, persistence.AutoMigrate(db))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRescan_DeletesOldStatuses(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	db := openTestDB(t)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	statusStore := persistence.NewStatusStore(db)

	repoID := int64(42)

	// Seed a failed status for this repository.
	failedStatus := task.NewStatus(
		task.OperationScanCommit,
		nil,
		task.TrackableTypeRepository,
		repoID,
	)
	failedStatus = failedStatus.Fail("something went wrong")
	_, err := statusStore.Save(ctx, failedStatus)
	require.NoError(t, err)

	// Verify the status exists.
	statuses, err := statusStore.FindByTrackable(ctx, task.TrackableTypeRepository, repoID)
	require.NoError(t, err)
	assert.Len(t, statuses, 1)

	h := NewRescan(enrichmentStore, associationStore, statusStore, &fakeTrackerFactory{}, logger)

	payload := map[string]any{
		"repository_id": repoID,
		"commit_sha":    "abc123def456",
	}

	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Old statuses should be gone.
	statuses, err = statusStore.FindByTrackable(ctx, task.TrackableTypeRepository, repoID)
	require.NoError(t, err)
	assert.Empty(t, statuses)

	// Statuses for other repositories should be unaffected.
	otherStatus := task.NewStatus(
		task.OperationScanCommit,
		nil,
		task.TrackableTypeRepository,
		int64(99),
	)
	_, err = statusStore.Save(ctx, otherStatus)
	require.NoError(t, err)

	count, err := statusStore.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
