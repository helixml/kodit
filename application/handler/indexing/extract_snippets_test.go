package indexing

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	domainservice "github.com/helixml/kodit/domain/service"
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

func (f *fakeTrackerFactory) ForOperation(_ task.Operation, _ task.TrackableType, _ int64) handler.Tracker {
	return &fakeTracker{}
}

func TestExtractSnippets(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("chunks files into snippet enrichments", func(t *testing.T) {
		db := testdb.New(t)
		repoStore := persistence.NewRepositoryStore(db)
		enrichmentStore := persistence.NewEnrichmentStore(db)
		associationStore := persistence.NewAssociationStore(db)
		fileStore := persistence.NewFileStore(db)

		// Create a temp directory with source files
		tmpDir := t.TempDir()
		goFile := filepath.Join(tmpDir, "main.go")
		goContent := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
		require.NoError(t, os.WriteFile(goFile, []byte(goContent), 0644))

		// Create repository pointing to the temp dir
		repo, err := repository.NewRepository("https://github.com/test/repo")
		require.NoError(t, err)
		repo = repo.
			WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
			WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
		savedRepo, err := repoStore.Save(ctx, repo)
		require.NoError(t, err)

		// Create file record in DB
		f := repository.NewFile("abc123", "main.go", "go", 100)
		savedFile, err := fileStore.Save(ctx, f)
		require.NoError(t, err)

		h := NewExtractSnippets(repoStore, enrichmentStore, associationStore, fileStore, &fakeTrackerFactory{}, logger)

		payload := map[string]any{
			"repository_id": savedRepo.ID(),
			"commit_sha":    "abc123",
		}

		err = h.Execute(ctx, payload)
		require.NoError(t, err)

		// Verify snippet enrichments were created
		snippets, err := enrichmentStore.Find(ctx, enrichment.WithCommitSHA("abc123"), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
		require.NoError(t, err)
		assert.Equal(t, 1, len(snippets))
		assert.Contains(t, snippets[0].Content(), "package main")
		assert.Equal(t, "go", snippets[0].Language())

		// Verify commit association
		commitAssocs, err := associationStore.Find(ctx, enrichment.WithEntityID("abc123"), enrichment.WithEntityType(enrichment.EntityTypeCommit))
		require.NoError(t, err)
		assert.Equal(t, 1, len(commitAssocs))

		// Verify file association
		fileAssocs, err := associationStore.Find(ctx, enrichment.WithEnrichmentID(snippets[0].ID()), enrichment.WithEntityType(enrichment.EntityTypeFile))
		require.NoError(t, err)
		assert.Equal(t, 1, len(fileAssocs))
		_ = savedFile
	})

	t.Run("chunks large files into multiple snippets", func(t *testing.T) {
		db := testdb.New(t)
		repoStore := persistence.NewRepositoryStore(db)
		enrichmentStore := persistence.NewEnrichmentStore(db)
		associationStore := persistence.NewAssociationStore(db)
		fileStore := persistence.NewFileStore(db)

		tmpDir := t.TempDir()

		// Create a large file with more than chunkLines lines
		var lines []string
		for i := 0; i < 150; i++ {
			lines = append(lines, "line "+strings.Repeat("x", 10))
		}
		largeContent := strings.Join(lines, "\n")
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "large.go"), []byte(largeContent), 0644))

		repo, err := repository.NewRepository("https://github.com/test/large")
		require.NoError(t, err)
		repo = repo.
			WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/large")).
			WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
		savedRepo, err := repoStore.Save(ctx, repo)
		require.NoError(t, err)

		f := repository.NewFile("sha456", "large.go", "go", 10000)
		_, err = fileStore.Save(ctx, f)
		require.NoError(t, err)

		h := NewExtractSnippets(repoStore, enrichmentStore, associationStore, fileStore, &fakeTrackerFactory{}, logger)

		payload := map[string]any{
			"repository_id": savedRepo.ID(),
			"commit_sha":    "sha456",
		}

		err = h.Execute(ctx, payload)
		require.NoError(t, err)

		snippets, err := enrichmentStore.Find(ctx, enrichment.WithCommitSHA("sha456"), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
		require.NoError(t, err)
		// 150 lines with chunkSize=60, overlap=10, step=50 → ceil(150/50)=3 chunks
		assert.Equal(t, 3, len(snippets))
	})

	t.Run("skips when snippets already exist", func(t *testing.T) {
		db := testdb.New(t)
		repoStore := persistence.NewRepositoryStore(db)
		enrichmentStore := persistence.NewEnrichmentStore(db)
		associationStore := persistence.NewAssociationStore(db)
		fileStore := persistence.NewFileStore(db)

		// Seed an existing snippet for the commit
		snip := enrichment.NewSnippetEnrichmentWithLanguage("existing code", "go")
		saved, err := enrichmentStore.Save(ctx, snip)
		require.NoError(t, err)
		_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), "existing123"))
		require.NoError(t, err)

		h := NewExtractSnippets(repoStore, enrichmentStore, associationStore, fileStore, &fakeTrackerFactory{}, logger)

		payload := map[string]any{
			"repository_id": int64(1),
			"commit_sha":    "existing123",
		}

		err = h.Execute(ctx, payload)
		require.NoError(t, err)

		// Count should still be 1 (no new snippets created)
		count, err := enrichmentStore.Count(ctx, enrichment.WithCommitSHA("existing123"), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("skips when no files found", func(t *testing.T) {
		db := testdb.New(t)
		repoStore := persistence.NewRepositoryStore(db)
		enrichmentStore := persistence.NewEnrichmentStore(db)
		associationStore := persistence.NewAssociationStore(db)
		fileStore := persistence.NewFileStore(db)

		tmpDir := t.TempDir()
		repo, err := repository.NewRepository("https://github.com/test/empty")
		require.NoError(t, err)
		repo = repo.
			WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/empty")).
			WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
		savedRepo, err := repoStore.Save(ctx, repo)
		require.NoError(t, err)

		h := NewExtractSnippets(repoStore, enrichmentStore, associationStore, fileStore, &fakeTrackerFactory{}, logger)

		payload := map[string]any{
			"repository_id": savedRepo.ID(),
			"commit_sha":    "nope123",
		}

		err = h.Execute(ctx, payload)
		require.NoError(t, err)

		count, err := enrichmentStore.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}

func TestExtractSnippetsAndBM25Index(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	db := testdb.New(t)
	repoStore := persistence.NewRepositoryStore(db)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	fileStore := persistence.NewFileStore(db)

	bm25Store, err := persistence.NewSQLiteBM25Store(db, logger)
	require.NoError(t, err)
	bm25Service, err := domainservice.NewBM25(bm25Store)
	require.NoError(t, err)

	// Create temp files
	tmpDir := t.TempDir()
	goContent := `package calculator

func Add(a, b int) int {
	return a + b
}

func Subtract(a, b int) int {
	return a - b
}

func Multiply(a, b int) int {
	return a * b
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "calc.go"), []byte(goContent), 0644))

	// Set up repository and file records
	repo, err := repository.NewRepository("https://github.com/test/calc")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/calc")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	f := repository.NewFile("commit789", "calc.go", "go", 200)
	_, err = fileStore.Save(ctx, f)
	require.NoError(t, err)

	// Step 1: Extract snippets
	extractHandler := NewExtractSnippets(repoStore, enrichmentStore, associationStore, fileStore, &fakeTrackerFactory{}, logger)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    "commit789",
	}

	err = extractHandler.Execute(ctx, payload)
	require.NoError(t, err)

	// Verify snippets were extracted
	snippets, err := enrichmentStore.Find(ctx, enrichment.WithCommitSHA("commit789"), enrichment.WithType(enrichment.TypeDevelopment), enrichment.WithSubtype(enrichment.SubtypeSnippet))
	require.NoError(t, err)
	require.NotEmpty(t, snippets, "expected at least one snippet")

	for _, s := range snippets {
		assert.NotEmpty(t, s.Content())
		assert.Equal(t, "go", s.Language())
	}

	// Step 2: Create BM25 index from the snippets
	bm25Handler := NewCreateBM25Index(bm25Service, enrichmentStore, &fakeTrackerFactory{}, logger)

	err = bm25Handler.Execute(ctx, payload)
	require.NoError(t, err)

	// Step 3: Search the BM25 index
	results, err := bm25Service.Find(ctx, "Add Subtract calculator")
	require.NoError(t, err)
	assert.NotEmpty(t, results, "expected BM25 results for calculator query")
}

func TestSplitLines(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := splitLines(nil, 10, 2)
		assert.Nil(t, result)
	})

	t.Run("single line", func(t *testing.T) {
		result := splitLines([]string{"hello"}, 10, 2)
		assert.Equal(t, []string{"hello"}, result)
	})

	t.Run("fits in single chunk", func(t *testing.T) {
		lines := []string{"a", "b", "c"}
		result := splitLines(lines, 10, 2)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, "a\nb\nc", result[0])
	})

	t.Run("splits with overlap", func(t *testing.T) {
		lines := make([]string, 10)
		for i := range lines {
			lines[i] = "line"
		}
		// size=5, overlap=2, step=3 → chunks start at 0,3,6
		result := splitLines(lines, 5, 2)
		assert.Equal(t, 3, len(result))
	})

	t.Run("skips blank chunks", func(t *testing.T) {
		lines := []string{"content", "", "", "", "", "", "", "", "", "", ""}
		result := splitLines(lines, 5, 1)
		// All chunks should have at least some non-blank content
		for _, chunk := range result {
			assert.NotEqual(t, "", strings.TrimSpace(chunk))
		}
	})
}

func TestFileLanguage(t *testing.T) {
	t.Run("uses file language field", func(t *testing.T) {
		f := repository.NewFile("sha", "test.go", "golang", 100)
		assert.Equal(t, "golang", fileLanguage(f))
	})

	t.Run("derives from extension", func(t *testing.T) {
		f := repository.NewFile("sha", "test.py", "", 100)
		assert.Equal(t, "python", fileLanguage(f))
	})

	t.Run("returns empty for unknown extension", func(t *testing.T) {
		f := repository.NewFile("sha", "test.xyz", "", 100)
		assert.Equal(t, "", fileLanguage(f))
	})
}
