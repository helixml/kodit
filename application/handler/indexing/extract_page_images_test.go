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

func TestExtractPageImages_DataAccessibleAfterIndexing(t *testing.T) {
	ctx, enrichmentStore, associationStore, sourceLocStore, repoStore, fileStore := setupPageImageTest(t)
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	commitSHA := "fff666aaa777"

	savedRepo := saveTestRepo(t, ctx, repoStore)
	clonedPath := savedRepo.WorkingCopy().Path()

	// Two PDF files: 2 pages and 1 page.
	pdf1 := repository.NewFileWithDetails(commitSHA, "docs/manual.pdf", "hash1", "application/pdf", ".pdf", 2000)
	savedPDF1, err := fileStore.Save(ctx, pdf1)
	require.NoError(t, err)

	pdf2 := repository.NewFileWithDetails(commitSHA, "slides/deck.pdf", "hash2", "application/pdf", ".pdf", 3000)
	savedPDF2, err := fileStore.Save(ctx, pdf2)
	require.NoError(t, err)

	rast := &fakeRasterizer{pages: map[string]int{
		clonedPath + "/docs/manual.pdf": 2,
		clonedPath + "/slides/deck.pdf": 1,
	}}
	reg := newTestRegistry(rast, ".pdf")

	h := NewExtractPageImages(repoStore, enrichmentStore, associationStore, sourceLocStore, fileStore, reg, &fakeTrackerFactory{}, logger)
	err = h.Execute(ctx, map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	})
	require.NoError(t, err)

	// Query all page image enrichments through the store.
	images, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypePageImage),
	)
	require.NoError(t, err)
	require.Len(t, images, 3, "2 pages from manual.pdf + 1 page from deck.pdf")

	// Collect enrichment IDs for bulk queries.
	ids := make([]int64, len(images))
	for i, img := range images {
		ids[i] = img.ID()
	}

	// Verify all source locations are accessible and have correct page numbers.
	pagesByFile := map[int64][]int{} // fileID → pages
	for _, img := range images {
		locs, err := sourceLocStore.Find(ctx, sourcelocation.WithEnrichmentID(img.ID()))
		require.NoError(t, err)
		require.Len(t, locs, 1, "each enrichment should have exactly one source location")
		assert.Greater(t, locs[0].Page(), 0, "page should be 1-based")
		assert.Zero(t, locs[0].StartLine(), "page image should not have line ranges")
		assert.Zero(t, locs[0].EndLine(), "page image should not have line ranges")

		// Find the file association to group pages by file.
		fileAssocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(img.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeFile),
		)
		require.NoError(t, err)
		require.Len(t, fileAssocs, 1)
		fileID, _ := strconv.ParseInt(fileAssocs[0].EntityID(), 10, 64)
		pagesByFile[fileID] = append(pagesByFile[fileID], locs[0].Page())
	}

	// manual.pdf (2 pages) should have pages [1, 2].
	manualPages := pagesByFile[savedPDF1.ID()]
	assert.ElementsMatch(t, []int{1, 2}, manualPages, "manual.pdf should have pages 1 and 2")

	// deck.pdf (1 page) should have page [1].
	deckPages := pagesByFile[savedPDF2.ID()]
	assert.ElementsMatch(t, []int{1}, deckPages, "deck.pdf should have page 1")

	// Verify commit associations exist for all enrichments.
	for _, img := range images {
		commitAssocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(img.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeCommit),
		)
		require.NoError(t, err)
		require.Len(t, commitAssocs, 1)
		assert.Equal(t, commitSHA, commitAssocs[0].EntityID())
	}

	// Verify repo associations exist for all enrichments.
	for _, img := range images {
		repoAssocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(img.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeRepository),
		)
		require.NoError(t, err)
		require.Len(t, repoAssocs, 1)
		assert.Equal(t, strconv.FormatInt(savedRepo.ID(), 10), repoAssocs[0].EntityID())
	}

	// Verify enrichment content is empty (images rendered on demand).
	for _, img := range images {
		assert.Empty(t, img.Content(), "page image content should be empty")
		assert.Equal(t, enrichment.SubtypePageImage, img.Subtype())
		assert.Equal(t, enrichment.TypeDevelopment, img.Type())
	}
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
