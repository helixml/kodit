package kodit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const unitTestPollPeriod = 50 * time.Millisecond

func TestNew_RequiresDatabase(t *testing.T) {
	_, err := kodit.New()
	assert.ErrorIs(t, err, kodit.ErrNoDatabase)
}

func TestNew_WithSQLite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	// Verify database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestClient_Close_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)

	// First close should succeed
	err = client.Close()
	assert.NoError(t, err)

	// Second close should return ErrClientClosed
	err = client.Close()
	assert.ErrorIs(t, err, kodit.ErrClientClosed)
}

func TestClient_Repositories_List_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	repos, err := client.Repositories.Find(ctx)
	require.NoError(t, err)
	assert.Empty(t, repos)
}

func TestClient_Tasks_List_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	tasks, err := client.Tasks.List(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestClient_Search_ReturnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	result, err := client.Search.Query(ctx, "test query",
		service.WithLimit(10),
		service.WithSemanticWeight(0.5),
	)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Count())
}

func TestClient_Search_AfterClose_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Search.Query(ctx, "test query")
	assert.ErrorIs(t, err, kodit.ErrClientClosed)
}

func TestWithFullPipeline_RequiresTextProvider(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	_, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithFullPipeline(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WithFullPipeline requires a text provider")
}

func TestWithRAGPipeline_WorksWithoutTextProvider(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithRAGPipeline(),
		kodit.WithEmbeddingProvider(&stubEmbedder{}),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
}

func TestWithRAGPipeline_WorksWithTextProvider(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Even with a text provider set, RAG pipeline should not require enrichment handlers.
	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithRAGPipeline(),
		kodit.WithEmbeddingProvider(&stubEmbedder{}),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
}

func TestWithDataDir_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "custom_data")
	dbPath := filepath.Join(dataDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(dataDir),
		kodit.WithSkipProviderValidation(),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Verify data directory was created
	info, err := os.Stat(dataDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestMigrate_ThenNewWithSkipMigrations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	require.NoError(t, kodit.Migrate(context.Background(), kodit.WithSQLite(dbPath)))

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithSkipMigrations(),
		kodit.WithSkipProviderValidation(),
		kodit.WithEmbeddingProvider(&stubEmbedder{}),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	require.NoError(t, err)
	assert.NoError(t, client.Close())
}

func TestNew_WithSkipMigrations_RejectsUnmigratedDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	_, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithSkipMigrations(),
		kodit.WithSkipProviderValidation(),
		kodit.WithEmbeddingProvider(&stubEmbedder{}),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
	)
	assert.ErrorContains(t, err, "validate schema")
}

// With WithDeferredWorkers nothing takes tasks off the queue until
// StartWorkers; StopWorkers pauses it again and StartWorkers resumes it.
func TestDeferredWorkers_ProcessTasksOnlyWhileStarted(t *testing.T) {
	ctx := context.Background()
	client, err := kodit.New(
		kodit.WithSQLite(filepath.Join(t.TempDir(), "test.db")),
		kodit.WithSkipProviderValidation(),
		kodit.WithEmbeddingProvider(&stubEmbedder{}),
		kodit.WithWorkerPollPeriod(unitTestPollPeriod),
		kodit.WithDeferredWorkers(),
	)
	require.NoError(t, err)
	defer func() { assert.NoError(t, client.Close()) }()

	enqueue := func(repoID int) {
		t.Helper()
		require.NoError(t, client.Tasks.Enqueue(ctx, task.NewTask(task.OperationSyncRepository,
			int(task.PriorityNormal), map[string]any{"repository_id": repoID})))
	}
	pending := func() int64 {
		n, err := client.Tasks.Count(ctx)
		require.NoError(t, err)
		return n
	}
	assertStaysQueued := func() {
		t.Helper()
		time.Sleep(10 * unitTestPollPeriod)
		assert.Equal(t, int64(1), pending(), "a stopped worker took a task")
	}
	assertDrained := func() {
		t.Helper()
		assert.Eventually(t, func() bool { return pending() == 0 }, 5*time.Second, unitTestPollPeriod,
			"a started worker did not take the task")
	}

	enqueue(1)
	assertStaysQueued()

	require.NoError(t, client.StartWorkers(ctx))
	require.NoError(t, client.StartWorkers(ctx), "starting twice is a no-op")
	assertDrained()

	client.StopWorkers()
	enqueue(2)
	assertStaysQueued()

	require.NoError(t, client.StartWorkers(ctx))
	assertDrained()
}
