package indexing

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"testing"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/chunking"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGitAdapter implements only the FileContent method we need for chunk tests.
// All other methods panic if called.
type fakeGitAdapter struct {
	files map[string][]byte
	err   error
}

func (f *fakeGitAdapter) FileContent(_ context.Context, _ string, _ string, filePath string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	content, ok := f.files[filePath]
	if !ok {
		return nil, os.ErrNotExist
	}
	return content, nil
}

func TestChunkFiles_SkipsWhenEnrichmentsExist(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "aaa111bbb222"

	// Pre-create a chunk enrichment for the commit.
	saved, err := enrichmentStore.Save(ctx, enrichment.NewChunkEnrichment("existing chunk"))
	require.NoError(t, err)
	_, err = associationStore.Save(ctx, enrichment.CommitAssociation(saved.ID(), commitSHA))
	require.NoError(t, err)

	// Create a repo so payload extraction doesn't fail.
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	tmpDir := t.TempDir()
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		&fakeGitAdapter{},
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	// Should return nil (skip) without creating new enrichments.
	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	all, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	assert.Len(t, all, 1, "no new enrichments should be created")
}

func TestChunkFiles_CreatesEnrichmentsForTextFiles(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "bbb222ccc333"
	tmpDir := t.TempDir()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	// Create a 200-char text file.
	content := make([]byte, 200)
	for i := range content {
		content[i] = 'A'
	}

	f := repository.NewFileWithDetails(commitSHA, "main.go", "abc123", "text/x-go", ".go", 200)
	savedFile, err := fileStore.Save(ctx, f)
	require.NoError(t, err)

	adapter := &fakeGitAdapter{files: map[string][]byte{"main.go": content}}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		adapter,
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	// Should create 2 chunk enrichments (200 chars / 100 chunk size).
	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	require.Len(t, chunks, 2)

	// Each chunk should have content of length 100.
	for _, c := range chunks {
		assert.Len(t, c.Content(), 100)
	}

	// Verify commit association exists for each chunk.
	for _, c := range chunks {
		assocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(c.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeCommit),
		)
		require.NoError(t, err)
		assert.Len(t, assocs, 1, "chunk should have commit association")
	}

	// Verify file association exists for each chunk.
	for _, c := range chunks {
		assocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(c.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeFile),
		)
		require.NoError(t, err)
		assert.Len(t, assocs, 1, "chunk should have file association")
		assert.Equal(t, strconv.FormatInt(savedFile.ID(), 10), assocs[0].EntityID())
	}

	// Verify repo association exists for each chunk.
	for _, c := range chunks {
		assocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(c.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeRepository),
		)
		require.NoError(t, err)
		assert.Len(t, assocs, 1, "chunk should have repository association")
		assert.Equal(t, strconv.FormatInt(savedRepo.ID(), 10), assocs[0].EntityID())
	}
}

func TestChunkFiles_SkipsBinaryFiles(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "ccc333ddd444"
	tmpDir := t.TempDir()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	// Create binary content (contains null byte).
	binaryContent := []byte("hello\x00world")
	f := repository.NewFileWithDetails(commitSHA, "image.png", "def456", "image/png", ".png", int64(len(binaryContent)))
	_, err = fileStore.Save(ctx, f)
	require.NoError(t, err)

	adapter := &fakeGitAdapter{files: map[string][]byte{"image.png": binaryContent}}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		adapter,
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	assert.Empty(t, chunks, "binary files should not produce chunks")
}

func TestChunkFiles_ContinuesOnFileContentError(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "ddd444eee555"
	tmpDir := t.TempDir()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	// Create two files: first will error, second will succeed.
	f1 := repository.NewFileWithDetails(commitSHA, "bad.go", "aaa", "text/x-go", ".go", 100)
	_, err = fileStore.Save(ctx, f1)
	require.NoError(t, err)
	f2 := repository.NewFileWithDetails(commitSHA, "good.go", "bbb", "text/x-go", ".go", 100)
	_, err = fileStore.Save(ctx, f2)
	require.NoError(t, err)

	goodContent := make([]byte, 100)
	for i := range goodContent {
		goodContent[i] = 'B'
	}
	// Only "good.go" has content; "bad.go" will get os.ErrNotExist
	adapter := &fakeGitAdapter{files: map[string][]byte{"good.go": goodContent}}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		adapter,
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	assert.Len(t, chunks, 1, "should create chunks for the successful file only")
}

func TestRelativeFilePath(t *testing.T) {
	tests := []struct {
		name      string
		filePath  string
		clonePath string
		want      string
	}{
		{
			name:      "already relative",
			filePath:  "src/main.go",
			clonePath: "/root/.kodit/repos/github.com_test_repo",
			want:      "src/main.go",
		},
		{
			name:      "absolute matching current clone dir",
			filePath:  "/root/.kodit/repos/github.com_test_repo/src/main.go",
			clonePath: "/root/.kodit/repos/github.com_test_repo",
			want:      "src/main.go",
		},
		{
			name:      "absolute with legacy clones prefix",
			filePath:  "/root/.kodit/clones/91983239377d5cbb-ent-demo/bigquery/main.py",
			clonePath: "/root/.kodit/repos/github.com_winderai_analytics-ai-agent-demo",
			want:      "bigquery/main.py",
		},
		{
			name:      "absolute with repos prefix different repo dir",
			filePath:  "/data/repos/old-name/lib/utils.rb",
			clonePath: "/data/repos/new-name",
			want:      "lib/utils.rb",
		},
		{
			name:      "unknown absolute path returned as-is",
			filePath:  "/some/random/absolute/path.txt",
			clonePath: "/root/.kodit/repos/myrepo",
			want:      "/some/random/absolute/path.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeFilePath(tt.filePath, tt.clonePath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestChunkFiles_HandlesAbsoluteFilePaths(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "fff666ggg777"
	tmpDir := t.TempDir()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	content := make([]byte, 100)
	for i := range content {
		content[i] = 'Z'
	}

	// File record has an absolute path (Python-era legacy data).
	absPath := "/root/.kodit/clones/91983239377d5cbb-ent-demo/bigquery/main.py"
	f := repository.NewFileWithDetails(commitSHA, absPath, "abc123", "text/x-python", ".py", 100)
	_, err = fileStore.Save(ctx, f)
	require.NoError(t, err)

	// The adapter expects the RELATIVE path since git show works with repo-relative paths.
	adapter := &fakeGitAdapter{files: map[string][]byte{"bigquery/main.py": content}}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		adapter,
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	assert.Len(t, chunks, 1, "should create chunk from file with absolute path")
}

func TestChunkFiles_OnlyIndexesSourceAndDocFiles(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "meta111meta222"
	tmpDir := t.TempDir()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	textContent := make([]byte, 100)
	for i := range textContent {
		textContent[i] = 'M'
	}

	// Files that should NOT be indexed (metadata, lock files, unknown extensions).
	skipped := []struct {
		path string
		ext  string
	}{
		{"go.mod", ".mod"},
		{"go.sum", ".sum"},
		{"package-lock.json", ".json"},
		{"yarn.lock", ".lock"},
		{"pnpm-lock.yaml", ".yaml"},
		{"Cargo.lock", ".lock"},
		{"composer.lock", ".lock"},
		{"data.csv", ".csv"},
		{"image.png", ".png"},
		{"nested/dir/model.pkl", ".pkl"},
	}

	// Files that SHOULD be indexed (source code and documentation).
	indexed := []struct {
		path string
		ext  string
	}{
		{"main.go", ".go"},
		{"app.py", ".py"},
		{"index.ts", ".ts"},
		{"component.tsx", ".tsx"},
		{"lib.rs", ".rs"},
		{"README.md", ".md"},
		{"setup.sh", ".sh"},
		{"style.css", ".css"},
		{"page.html", ".html"},
		{"query.sql", ".sql"},
	}

	adapterFiles := make(map[string][]byte)
	for _, sf := range skipped {
		f := repository.NewFileWithDetails(commitSHA, sf.path, "abc", "text/plain", sf.ext, 100)
		_, err = fileStore.Save(ctx, f)
		require.NoError(t, err)
		adapterFiles[sf.path] = textContent
	}
	for _, sf := range indexed {
		f := repository.NewFileWithDetails(commitSHA, sf.path, "abc", "text/plain", sf.ext, 100)
		_, err = fileStore.Save(ctx, f)
		require.NoError(t, err)
		adapterFiles[sf.path] = textContent
	}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		&fakeGitAdapter{files: adapterFiles},
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	assert.Len(t, chunks, len(indexed), "only source and documentation files should be indexed")
}

func TestChunkFiles_SetsLanguageFromExtension(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "eee555fff666"
	tmpDir := t.TempDir()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	content := make([]byte, 100)
	for i := range content {
		content[i] = 'X'
	}

	f := repository.NewFileWithDetails(commitSHA, "script.py", "abc123", "text/x-python", ".py", 100)
	_, err = fileStore.Save(ctx, f)
	require.NoError(t, err)

	adapter := &fakeGitAdapter{files: map[string][]byte{"script.py": content}}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		adapter,
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, ".py", chunks[0].Language())
}

func TestChunkFiles_PersistsLineRanges(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "linerange111222"
	tmpDir := t.TempDir()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	// 3 lines of 10 chars each → with Size=15 we get 2 chunks:
	//   chunk 1: lines 1-2 ("aaaaaaaaaa\nbbbbbbbbbb\n")
	//   chunk 2: line 3  ("cccccccccc\n")
	content := []byte("aaaaaaaaaa\nbbbbbbbbbb\ncccccccccc\n")

	f := repository.NewFileWithDetails(commitSHA, "lines.go", "abc123", "text/x-go", ".go", int64(len(content)))
	_, err = fileStore.Save(ctx, f)
	require.NoError(t, err)

	adapter := &fakeGitAdapter{files: map[string][]byte{"lines.go": content}}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		adapter,
		chunking.ChunkParams{Size: 25, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.NoError(t, err)

	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	require.Len(t, chunks, 2)

	// Verify a line range was persisted for each chunk enrichment.
	for _, c := range chunks {
		ranges, err := lineRangeStore.Find(ctx, repository.WithCondition("enrichment_id", c.ID()))
		require.NoError(t, err)
		require.Len(t, ranges, 1, "each chunk should have exactly one line range")
		assert.Greater(t, ranges[0].StartLine(), 0, "start line must be positive")
		assert.GreaterOrEqual(t, ranges[0].EndLine(), ranges[0].StartLine(), "end line >= start line")
		assert.Equal(t, c.ID(), ranges[0].EnrichmentID())
	}

	// First chunk should cover lines 1-2, second chunk should cover line 3.
	r1, err := lineRangeStore.Find(ctx, repository.WithCondition("enrichment_id", chunks[0].ID()))
	require.NoError(t, err)
	assert.Equal(t, 1, r1[0].StartLine())
	assert.Equal(t, 2, r1[0].EndLine())

	r2, err := lineRangeStore.Find(ctx, repository.WithCondition("enrichment_id", chunks[1].ID()))
	require.NoError(t, err)
	assert.Equal(t, 3, r2[0].StartLine())
	assert.Equal(t, 3, r2[0].EndLine())
}
