package indexing

import (
	"context"
	"errors"
	"image"
	"os"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/sourcelocation"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/infrastructure/rasterization"
	"github.com/helixml/kodit/internal/testdb"
)

// fakeRasterizer implements rasterization.Rasterizer for tests.
type fakeRasterizer struct {
	pages map[string]int
	err   error
}

func (f *fakeRasterizer) PageCount(path string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	n, ok := f.pages[path]
	if !ok {
		return 0, errors.New("file not found")
	}
	return n, nil
}

func (f *fakeRasterizer) Render(_ string, _ int) (image.Image, error) {
	return nil, errors.New("render not implemented in fake")
}

func (f *fakeRasterizer) Close() error { return nil }

func newTestRegistry(rast *fakeRasterizer, exts ...string) *rasterization.Registry {
	reg := rasterization.NewRegistry()
	for _, ext := range exts {
		reg.Register(ext, rast)
	}
	return reg
}

func setupPageImageTest(t *testing.T) (
	context.Context,
	persistence.EnrichmentStore,
	persistence.AssociationStore,
	persistence.SourceLocationStore,
	persistence.RepositoryStore,
	persistence.FileStore,
) {
	t.Helper()
	db := testdb.New(t)
	return context.Background(),
		persistence.NewEnrichmentStore(db),
		persistence.NewAssociationStore(db),
		persistence.NewSourceLocationStore(db),
		persistence.NewRepositoryStore(db),
		persistence.NewFileStore(db)
}

func saveTestRepo(t *testing.T, ctx context.Context, repoStore persistence.RepositoryStore) repository.Repository {
	t.Helper()
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(t.TempDir(), "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	saved, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)
	return saved
}

func TestExtractPageImages_NilRegistry(t *testing.T) {
	h := NewExtractPageImages(nil, nil, nil, nil, nil, nil, &fakeTrackerFactory{}, zerolog.Nop())
	err := h.Execute(context.Background(), map[string]any{
		"repository_id": int64(1),
		"commit_sha":    "abc123",
	})
	assert.NoError(t, err)
}

func TestExtractPageImages_SkipsWhenEnrichmentsExist(t *testing.T) {
	ctx, enrichmentStore, associationStore, sourceLocStore, repoStore, fileStore := setupPageImageTest(t)
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	commitSHA := "aaa111bbb222"

	saved, err := enrichmentStore.Save(ctx, enrichment.NewPageImage())
	require.NoError(t, err)
	_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA))
	require.NoError(t, err)

	savedRepo := saveTestRepo(t, ctx, repoStore)

	reg := newTestRegistry(&fakeRasterizer{pages: map[string]int{}}, ".pdf")

	h := NewExtractPageImages(repoStore, enrichmentStore, associationStore, sourceLocStore, fileStore, reg, &fakeTrackerFactory{}, logger)

	err = h.Execute(ctx, map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	})
	require.NoError(t, err)

	all, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypePageImage),
	)
	require.NoError(t, err)
	assert.Len(t, all, 1, "no new enrichments should be created")
}

func TestExtractPageImages_CreatesEnrichmentsForPDFPages(t *testing.T) {
	ctx, enrichmentStore, associationStore, sourceLocStore, repoStore, fileStore := setupPageImageTest(t)
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	commitSHA := "bbb222ccc333"

	savedRepo := saveTestRepo(t, ctx, repoStore)
	clonedPath := savedRepo.WorkingCopy().Path()

	f := repository.NewFileWithDetails(commitSHA, "docs/report.pdf", "abc123", "application/pdf", ".pdf", 1000)
	savedFile, err := fileStore.Save(ctx, f)
	require.NoError(t, err)

	diskPath := clonedPath + "/docs/report.pdf"
	rast := &fakeRasterizer{pages: map[string]int{diskPath: 3}}
	reg := newTestRegistry(rast, ".pdf")

	h := NewExtractPageImages(repoStore, enrichmentStore, associationStore, sourceLocStore, fileStore, reg, &fakeTrackerFactory{}, logger)

	err = h.Execute(ctx, map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	})
	require.NoError(t, err)

	// Should create 3 page image enrichments.
	images, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypePageImage),
	)
	require.NoError(t, err)
	require.Len(t, images, 3)

	// Verify each enrichment has empty content.
	for _, img := range images {
		assert.Empty(t, img.Content())
	}

	// Verify source locations have pages 1, 2, 3.
	for i, img := range images {
		locs, err := sourceLocStore.Find(ctx, sourcelocation.WithEnrichmentID(img.ID()))
		require.NoError(t, err)
		require.Len(t, locs, 1)
		assert.Equal(t, i+1, locs[0].Page())
		assert.Equal(t, 0, locs[0].StartLine())
		assert.Equal(t, 0, locs[0].EndLine())
	}

	// Verify file association for each enrichment.
	for _, img := range images {
		assocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(img.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeFile),
		)
		require.NoError(t, err)
		require.Len(t, assocs, 1)
		assert.Equal(t, strconv.FormatInt(savedFile.ID(), 10), assocs[0].EntityID())
	}

	// Verify repo association for each enrichment.
	for _, img := range images {
		assocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(img.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeRepository),
		)
		require.NoError(t, err)
		require.Len(t, assocs, 1)
		assert.Equal(t, strconv.FormatInt(savedRepo.ID(), 10), assocs[0].EntityID())
	}
}

func TestExtractPageImages_SkipsNonRasterizableFiles(t *testing.T) {
	ctx, enrichmentStore, associationStore, sourceLocStore, repoStore, fileStore := setupPageImageTest(t)
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	commitSHA := "ccc333ddd444"

	savedRepo := saveTestRepo(t, ctx, repoStore)

	// Create a .go file and a .pdf file.
	goFile := repository.NewFileWithDetails(commitSHA, "main.go", "abc123", "text/x-go", ".go", 500)
	_, err := fileStore.Save(ctx, goFile)
	require.NoError(t, err)

	clonedPath := savedRepo.WorkingCopy().Path()
	diskPath := clonedPath + "/report.pdf"
	pdfFile := repository.NewFileWithDetails(commitSHA, "report.pdf", "def456", "application/pdf", ".pdf", 1000)
	_, err = fileStore.Save(ctx, pdfFile)
	require.NoError(t, err)

	rast := &fakeRasterizer{pages: map[string]int{diskPath: 2}}
	reg := newTestRegistry(rast, ".pdf")

	h := NewExtractPageImages(repoStore, enrichmentStore, associationStore, sourceLocStore, fileStore, reg, &fakeTrackerFactory{}, logger)

	err = h.Execute(ctx, map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	})
	require.NoError(t, err)

	// Only 2 enrichments from the PDF, none from .go.
	images, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypePageImage),
	)
	require.NoError(t, err)
	assert.Len(t, images, 2)
}

func TestExtractPageImages_ContinuesOnPageCountError(t *testing.T) {
	ctx, enrichmentStore, associationStore, sourceLocStore, repoStore, fileStore := setupPageImageTest(t)
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	commitSHA := "ddd444eee555"

	savedRepo := saveTestRepo(t, ctx, repoStore)
	clonedPath := savedRepo.WorkingCopy().Path()

	// Two PDFs: first will error, second should succeed.
	_, err := fileStore.Save(ctx, repository.NewFileWithDetails(commitSHA, "bad.pdf", "abc123", "application/pdf", ".pdf", 500))
	require.NoError(t, err)
	_, err = fileStore.Save(ctx, repository.NewFileWithDetails(commitSHA, "good.pdf", "def456", "application/pdf", ".pdf", 1000))
	require.NoError(t, err)

	rast := &fakeRasterizer{pages: map[string]int{
		clonedPath + "/good.pdf": 2,
		// bad.pdf not in map → returns error
	}}
	reg := newTestRegistry(rast, ".pdf")

	h := NewExtractPageImages(repoStore, enrichmentStore, associationStore, sourceLocStore, fileStore, reg, &fakeTrackerFactory{}, logger)

	err = h.Execute(ctx, map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	})
	require.NoError(t, err)

	// Only 2 enrichments from good.pdf.
	images, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypePageImage),
	)
	require.NoError(t, err)
	assert.Len(t, images, 2)
}

func TestExtractPageImages_NoFiles(t *testing.T) {
	ctx, enrichmentStore, associationStore, sourceLocStore, repoStore, fileStore := setupPageImageTest(t)
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	commitSHA := "eee555fff666"

	savedRepo := saveTestRepo(t, ctx, repoStore)

	reg := newTestRegistry(&fakeRasterizer{}, ".pdf")

	h := NewExtractPageImages(repoStore, enrichmentStore, associationStore, sourceLocStore, fileStore, reg, &fakeTrackerFactory{}, logger)

	err := h.Execute(ctx, map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	})
	require.NoError(t, err)

	images, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypePageImage),
	)
	require.NoError(t, err)
	assert.Empty(t, images)
}
