package kodit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/helixml/kodit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_RequiresStorage(t *testing.T) {
	_, err := kodit.New()
	assert.ErrorIs(t, err, kodit.ErrNoStorage)
}

func TestNew_WithSQLite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
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
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	repos, err := client.Repositories().List(ctx)
	require.NoError(t, err)
	assert.Empty(t, repos)
}

func TestClient_Tasks_List_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	tasks, err := client.Tasks().List(ctx)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestClient_Search_ReturnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	result, err := client.Search(ctx, "test query",
		kodit.WithLimit(10),
		kodit.WithSemanticWeight(0.5),
	)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Count())
}

func TestClient_Search_AfterClose_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
	)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Search(ctx, "test query")
	assert.ErrorIs(t, err, kodit.ErrClientClosed)
}

func TestWithDataDir_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "custom_data")
	dbPath := filepath.Join(dataDir, "test.db")

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(dataDir),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	// Verify data directory was created
	info, err := os.Stat(dataDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
