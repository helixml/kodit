package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
)

func newDeleteHandler(t *testing.T) (*Delete, persistence.RepositoryStore) {
	t.Helper()
	db := testdb.New(t)
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)

	repoStore := persistence.NewRepositoryStore(db)
	commitStore := persistence.NewCommitStore(db)
	branchStore := persistence.NewBranchStore(db)
	tagStore := persistence.NewTagStore(db)
	fileStore := persistence.NewFileStore(db)
	taskStore := persistence.NewTaskStore(db)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)

	enrichments := service.NewEnrichment(
		enrichmentStore,
		associationStore,
		nil, // bm25 — not reached during delete
		nil, // code embeddings
		nil, // text embeddings
		nil, // vision embeddings
		nil, // line ranges
	)

	queue := service.NewQueue(taskStore, logger)

	stores := handler.RepositoryStores{
		Repositories: repoStore,
		Commits:      commitStore,
		Branches:     branchStore,
		Tags:         tagStore,
		Files:        fileStore,
	}

	h := NewDelete(stores, enrichments, queue, &fakeSyncTrackerFactory{}, logger)
	return h, repoStore
}

func TestDelete_PreservesLocalFileDirectory(t *testing.T) {
	ctx := context.Background()
	h, repoStore := newDeleteHandler(t)

	// Create a temp directory that simulates the user's filestore.
	userDir := t.TempDir()
	sentinel := filepath.Join(userDir, "important-file.pdf")
	require.NoError(t, os.WriteFile(sentinel, []byte("do not delete"), 0644))

	// Register a file:// repository pointing at the user directory.
	repo, err := repository.NewRepository("file://" + userDir)
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy(userDir, "file://"+userDir))
	repo, err = repoStore.Save(ctx, repo)
	require.NoError(t, err)

	// Delete the repository.
	payload := map[string]any{"repository_id": repo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// The user directory and its contents must still exist.
	assert.DirExists(t, userDir)
	assert.FileExists(t, sentinel)
}

func TestDelete_RemovesClonedDirectory(t *testing.T) {
	ctx := context.Background()
	h, repoStore := newDeleteHandler(t)

	// Create a temp directory that simulates a Kodit-managed clone.
	cloneDir := t.TempDir()
	sentinel := filepath.Join(cloneDir, "cloned-file.go")
	require.NoError(t, os.WriteFile(sentinel, []byte("cloned content"), 0644))

	// Register a remote (non-file://) repository with a cloned working copy.
	repo, err := repository.NewRepository("https://github.com/example/repo.git")
	require.NoError(t, err)
	repo = repo.WithWorkingCopy(repository.NewWorkingCopy(cloneDir, "https://github.com/example/repo.git"))
	repo, err = repoStore.Save(ctx, repo)
	require.NoError(t, err)

	// Delete the repository.
	payload := map[string]any{"repository_id": repo.ID()}
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// The cloned directory should have been removed.
	assert.NoDirExists(t, cloneDir)
}
