package indexing

import (
	"context"
	"image"
	"image/color"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/sourcelocation"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/infrastructure/rasterization"
	"github.com/helixml/kodit/internal/testdb"
)

// renderableRasterizer is a fake rasterizer that returns a real image.
type renderableRasterizer struct {
	pages map[string]int
}

func (r *renderableRasterizer) PageCount(path string) (int, error) {
	n := r.pages[path]
	return n, nil
}

func (r *renderableRasterizer) Render(_ string, _ int) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.White)
	return img, nil
}

func (r *renderableRasterizer) Close() error { return nil }

// fakeVisionEmbedder is a fake search.Embedder that returns deterministic vectors.
type fakeVisionEmbedder struct {
	called int
}

func (f *fakeVisionEmbedder) Embed(_ context.Context, items []search.EmbeddingItem) ([][]float64, error) {
	f.called += len(items)
	result := make([][]float64, len(items))
	for i := range items {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}

// fakeVisionStore is a fake search.EmbeddingStore that tracks saved embeddings.
type fakeVisionStore struct {
	saved []search.Embedding
}

func (s *fakeVisionStore) SaveAll(_ context.Context, embeddings []search.Embedding) error {
	s.saved = append(s.saved, embeddings...)
	return nil
}

func (s *fakeVisionStore) Find(_ context.Context, _ ...repository.Option) ([]search.Embedding, error) {
	return nil, nil
}

func (s *fakeVisionStore) Search(_ context.Context, _ ...repository.Option) ([]search.Result, error) {
	return nil, nil
}

func (s *fakeVisionStore) Exists(_ context.Context, _ ...repository.Option) (bool, error) {
	return false, nil
}

func (s *fakeVisionStore) DeleteBy(_ context.Context, _ ...repository.Option) error {
	return nil
}

// setupEmbeddingTest creates the stores and seeds data as the extract handler would.
func setupEmbeddingTest(t *testing.T) (
	context.Context,
	persistence.EnrichmentStore,
	persistence.AssociationStore,
	persistence.SourceLocationStore,
	persistence.RepositoryStore,
	persistence.FileStore,
	repository.Repository,
	string, // commitSHA
) {
	t.Helper()
	db := testdb.New(t)
	ctx := context.Background()

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	sourceLocStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	// Create a repo with a working copy.
	repo, err := repository.NewRepository("https://github.com/test/pdf-repo")
	require.NoError(t, err)
	cloneDir := t.TempDir()
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(cloneDir, "https://github.com/test/pdf-repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	commitSHA := "abc123def456"

	// Create a commit.
	now := time.Now()
	author := repository.NewAuthor("test", "test@example.com")
	commit := repository.NewCommit(commitSHA, savedRepo.ID(), "add pdfs", author, author, now, now)
	_, err = persistence.NewCommitStore(db).Save(ctx, commit)
	require.NoError(t, err)

	// Create a PDF file record.
	f := repository.NewFileWithDetails(commitSHA, "docs/report.pdf", "blobsha1", "application/pdf", ".pdf", 5000)
	savedFile, err := fileStore.Save(ctx, f)
	require.NoError(t, err)

	// Simulate what the extract handler does: create page image enrichments
	// with commit, file, and repository associations + source locations.
	for page := 1; page <= 3; page++ {
		e := enrichment.NewPageImage()
		saved, saveErr := enrichmentStore.Save(ctx, e)
		require.NoError(t, saveErr)

		sl := sourcelocation.NewPage(saved.ID(), page)
		_, err = sourceLocStore.Save(ctx, sl)
		require.NoError(t, err)

		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA))
		require.NoError(t, err)

		_, err = associationStore.Save(ctx, enrichment.FileAssociation(saved.ID(), strconv.FormatInt(savedFile.ID(), 10)))
		require.NoError(t, err)

		_, err = associationStore.Save(ctx, enrichment.RepositoryAssociation(saved.ID(), strconv.FormatInt(savedRepo.ID(), 10)))
		require.NoError(t, err)
	}

	return ctx, enrichmentStore, associationStore, sourceLocStore, repoStore, fileStore, savedRepo, commitSHA
}

func TestCreatePageImageEmbeddings_ProcessesEnrichments(t *testing.T) {
	ctx, enrichmentStore, associationStore, sourceLocStore, repoStore, fileStore, savedRepo, commitSHA := setupEmbeddingTest(t)

	embedder := &fakeVisionEmbedder{}
	store := &fakeVisionStore{}
	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)

	reg := rasterization.NewRegistry()
	reg.Register(".pdf", &renderableRasterizer{pages: map[string]int{}})

	h, err := NewCreatePageImageEmbeddings(
		repoStore,
		enrichmentStore,
		associationStore,
		sourceLocStore,
		fileStore,
		reg,
		embedder,
		store,
		&fakeTrackerFactory{},
		logger,
	)
	require.NoError(t, err)

	err = h.Execute(ctx, map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	})
	require.NoError(t, err)

	// The handler should have found 3 page image enrichments and embedded them.
	assert.Equal(t, 3, embedder.called, "expected 3 images embedded")
	assert.Len(t, store.saved, 3, "expected 3 embeddings saved")
}
