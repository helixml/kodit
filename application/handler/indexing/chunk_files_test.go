package indexing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/chunking"
	"github.com/helixml/kodit/infrastructure/extraction"
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
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
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
		&fakeGitAdapter{}, nil, extraction.NewExtractors(), nil,
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
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "bbb222ccc333"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
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
		adapter, nil, extraction.NewExtractors(), nil,
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
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
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
		adapter, nil, extraction.NewExtractors(), nil,
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
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "ddd444eee555"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
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
		adapter, nil, extraction.NewExtractors(), nil,
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

func TestSafeDiskPath(t *testing.T) {
	tests := []struct {
		name      string
		clonePath string
		relPath   string
		wantSafe  bool
	}{
		{
			name:      "normal relative path",
			clonePath: "/data/repos/myrepo",
			relPath:   "src/main.go",
			wantSafe:  true,
		},
		{
			name:      "traversal escapes clone dir",
			clonePath: "/data/repos/myrepo",
			relPath:   "../../etc/passwd",
			wantSafe:  false,
		},
		{
			name:      "dot-dot in middle that stays inside",
			clonePath: "/data/repos/myrepo",
			relPath:   "src/../lib/util.go",
			wantSafe:  true,
		},
		{
			name:      "dot-dot that escapes via nested traversal",
			clonePath: "/data/repos/myrepo",
			relPath:   "src/../../../../etc/shadow",
			wantSafe:  false,
		},
		{
			name:      "single dot stays inside",
			clonePath: "/data/repos/myrepo",
			relPath:   ".",
			wantSafe:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, safe := safeDiskPath(tt.clonePath, tt.relPath)
			assert.Equal(t, tt.wantSafe, safe)
			if safe {
				assert.NotEmpty(t, path)
			}
		})
	}
}

func TestChunkFiles_HandlesAbsoluteFilePaths(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "fff666ggg777"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
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
		adapter, nil, extraction.NewExtractors(), nil,
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
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "meta111meta222"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
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
		{"data.csv", ".csv"},
	}

	csvContent := []byte("name,city\nalice,london\n")

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
		if sf.ext == ".csv" {
			adapterFiles[sf.path] = csvContent
		} else {
			adapterFiles[sf.path] = textContent
		}
	}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		&fakeGitAdapter{files: adapterFiles}, nil, extraction.NewExtractors(), nil,
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
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "eee555fff666"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
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
		adapter, nil, extraction.NewExtractors(), nil,
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
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "linerange111222"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(25, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
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
		adapter, nil, extraction.NewExtractors(), nil,
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

// fakeDocumentText implements DocumentTextSource for testing.
type fakeDocumentText struct {
	texts map[string]string
	err   error
}

func (f *fakeDocumentText) Text(path string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	text, ok := f.texts[path]
	if !ok {
		return "", fmt.Errorf("no text for %s", path)
	}
	return text, nil
}

// fakeFailingRenderer is an extraction.TextRenderer whose PageCount and Render
// always return the configured error — used to model PDF parsers that bail at
// the cross-reference / content-stream layer. The error echoes the path it was
// given, modelling parsers that embed local paths in their messages (like
// os.Stat's PathError does).
type fakeFailingRenderer struct {
	err error
}

func (f *fakeFailingRenderer) PageCount(path string) (int, error) {
	return 0, fmt.Errorf("open pdf %s: %w", path, f.err)
}

func (f *fakeFailingRenderer) Render(path string, _ int) (string, error) {
	return "", fmt.Errorf("open pdf %s: %w", path, f.err)
}

func (f *fakeFailingRenderer) Close() error { return nil }

func TestChunkFiles_ExtractsDocumentFiles(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "doc111doc222"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	f := repository.NewFileWithDetails(commitSHA, "manual.pdf", "abc123", "application/pdf", ".pdf", 5000)
	savedFile, err := fileStore.Save(ctx, f)
	require.NoError(t, err)

	extractedText := strings.Repeat("Document content. ", 10)
	diskPath := filepath.Join(tmpDir, "manual.pdf")
	docText := &fakeDocumentText{texts: map[string]string{diskPath: extractedText}}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		&fakeGitAdapter{}, docText, extraction.NewExtractors(), nil,
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
	assert.NotEmpty(t, chunks, "document files should produce chunks")

	// Verify associations exist for each chunk.
	for _, c := range chunks {
		commitAssocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(c.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeCommit),
		)
		require.NoError(t, err)
		assert.Len(t, commitAssocs, 1)

		fileAssocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(c.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeFile),
		)
		require.NoError(t, err)
		assert.Len(t, fileAssocs, 1)
		assert.Equal(t, strconv.FormatInt(savedFile.ID(), 10), fileAssocs[0].EntityID())

		repoAssocs, err := associationStore.Find(ctx,
			enrichment.WithEnrichmentID(c.ID()),
			enrichment.WithEntityType(enrichment.EntityTypeRepository),
		)
		require.NoError(t, err)
		assert.Len(t, repoAssocs, 1)
		assert.Equal(t, strconv.FormatInt(savedRepo.ID(), 10), repoAssocs[0].EntityID())
	}
}

func TestChunkFiles_SkipsDocumentsWhenExtractorNil(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "nodoc111222"
	tmpDir := t.TempDir()

	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", ""))
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	f := repository.NewFileWithDetails(commitSHA, "report.pdf", "abc123", "application/pdf", ".pdf", 5000)
	_, err = fileStore.Save(ctx, f)
	require.NoError(t, err)

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		&fakeGitAdapter{}, nil, extraction.NewExtractors(), nil,
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
	assert.Empty(t, chunks, "document files should be skipped when extractor is nil")
}

func TestChunkFiles_ErrorsWhenOnlyUnreadableDocumentsExist(t *testing.T) {
	// A commit whose only files are unreadable documents must fail rather than
	// report success with no chunks (issue #553): the BM25 and embedding steps
	// would otherwise run on empty input and the commit would look text-indexed
	// while storing nothing. Issue #576 keeps this failure loud without letting
	// one bad document block the rest of a commit that has readable files.
	ctx := context.Background()
	var logBuf bytes.Buffer
	logger := zerolog.New(&logBuf).Level(zerolog.WarnLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "docerr111222"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	f := repository.NewFileWithDetails(commitSHA, "bad.pdf", "aaa", "application/pdf", ".pdf", 5000)
	_, err = fileStore.Save(ctx, f)
	require.NoError(t, err)

	docText := &fakeDocumentText{err: fmt.Errorf("invalid xref subsection header")}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		&fakeGitAdapter{}, docText, extraction.NewExtractors(), nil,
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.Error(t, err, "a commit with only unreadable documents must fail, not report success with no chunks")
	assert.Contains(t, err.Error(), "bad.pdf", "the error should identify the failing file")
	assert.Contains(t, err.Error(), "invalid xref subsection header", "the error should include the parser error")
	assert.NotContains(t, err.Error(), tmpDir, "the persisted error must not expose local clone paths")

	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	assert.Empty(t, chunks, "no chunks should be created for the unreadable document")

	logs := logBuf.String()
	assert.Contains(t, logs, "bad.pdf", "the warning should identify the failing file")
	assert.Contains(t, logs, "invalid xref subsection header", "the warning should include the parser error")
}

func TestChunkFiles_ErrorsWhenPerPageAndFallbackExtractionFail(t *testing.T) {
	ctx := context.Background()
	var logBuf bytes.Buffer
	logger := zerolog.New(&logBuf).Level(zerolog.WarnLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "perpageerr111"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	f := repository.NewFileWithDetails(commitSHA, "broken.pdf", "aaa", "application/pdf", ".pdf", 5000)
	_, err = fileStore.Save(ctx, f)
	require.NoError(t, err)

	// Both per-page and whole-document extractors fail with the same xref error
	// — this is the real-world case from issue #553.
	registry := extraction.NewTextRendererRegistry()
	registry.Register(".pdf", &fakeFailingRenderer{err: fmt.Errorf("invalid xref subsection header")})
	docText := &fakeDocumentText{err: fmt.Errorf("invalid xref subsection header")}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		&fakeGitAdapter{}, docText, extraction.NewExtractors(), registry,
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.Error(t, err, "a commit with only unreadable documents must fail, not report success with no chunks")
	assert.Contains(t, err.Error(), "broken.pdf", "the error should identify the failing file")
	assert.Contains(t, err.Error(), "invalid xref subsection header", "the error should include the parser error")
	assert.NotContains(t, err.Error(), tmpDir, "the persisted error must not expose local clone paths")

	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	assert.Empty(t, chunks, "no chunks should be created for the unreadable document")

	logs := logBuf.String()
	assert.Contains(t, logs, "broken.pdf", "the warning should identify the failing file")
	assert.Contains(t, logs, "invalid xref subsection header", "the warning should include the parser error")
}

func TestChunkFiles_SkipsUnreadableDocumentAndChunksRemainingFiles(t *testing.T) {
	// Issue #576: one unreadable document (here: a password-protected PDF) used
	// to abort chunking at the first failing file. Chunks written before it were
	// kept, so the next run skipped the commit as "already chunked" and every
	// file whose path sorted after the document was never indexed.
	//
	// The fix keeps chunking the remaining files and reports every failure in
	// an aggregated error after the loop, so the failure is visible in task
	// status without dropping the rest of the commit.
	ctx := context.Background()
	var logBuf bytes.Buffer
	logger := zerolog.New(&logBuf).Level(zerolog.WarnLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "skip111doc222"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	// The unreadable document sorts BEFORE the Go file, mirroring the issue:
	// one unreadable document left every file whose path sorted after it
	// unindexed.
	pdf := repository.NewFileWithDetails(commitSHA, "0-notes.pdf", "aaa", "application/pdf", ".pdf", 5000)
	_, err = fileStore.Save(ctx, pdf)
	require.NoError(t, err)

	goContent := make([]byte, 100)
	for i := range goContent {
		goContent[i] = 'G'
	}
	gof := repository.NewFileWithDetails(commitSHA, "1-main.go", "bbb", "text/x-go", ".go", 100)
	_, err = fileStore.Save(ctx, gof)
	require.NoError(t, err)

	// Both per-page and whole-document extraction fail, like an encrypted PDF:
	// per-page fails with "invalid password", the fallback with a decompression
	// error.
	registry := extraction.NewTextRendererRegistry()
	registry.Register(".pdf", &fakeFailingRenderer{err: fmt.Errorf("invalid password")})
	docText := &fakeDocumentText{err: fmt.Errorf("zlib decompression failed")}

	adapter := &fakeGitAdapter{files: map[string][]byte{"1-main.go": goContent}}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		adapter, docText, extraction.NewExtractors(), registry,
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	// The task still fails loudly so unreadable documents are visible in task
	// status (issue #553) — but only after the whole commit was processed.
	require.Error(t, err, "unreadable documents must fail the task")
	assert.Contains(t, err.Error(), "0-notes.pdf", "the error should name the unreadable document")
	assert.Contains(t, err.Error(), "invalid password", "the error should include the per-page parser error")
	assert.Contains(t, err.Error(), "zlib decompression failed", "the error should include the fallback parser error")
	assert.NotContains(t, err.Error(), tmpDir, "the persisted error must not expose local clone paths")

	chunks, err := enrichmentStore.Find(ctx,
		enrichment.WithCommitSHA(commitSHA),
		enrichment.WithType(enrichment.TypeDevelopment),
		enrichment.WithSubtype(enrichment.SubtypeChunk),
	)
	require.NoError(t, err)
	require.Len(t, chunks, 1, "the Go file sorted after the unreadable document must still be chunked")
	assert.Equal(t, ".go", chunks[0].Language())

	// The skip is visible: a warning names the unreadable file and both errors.
	logs := logBuf.String()
	assert.Contains(t, logs, "0-notes.pdf", "the warning should identify the unreadable document")
	assert.Contains(t, logs, "invalid password", "the warning should include the per-page error")
	assert.Contains(t, logs, "zlib decompression failed", "the warning should include the fallback error")
}

func TestChunkFiles_AggregatedDocumentErrorIsBounded(t *testing.T) {
	// A repository can contain many unreadable documents. The aggregated error
	// includes only the first failures plus a count of the rest (each failure
	// is still logged per file), so the error persisted in task status cannot
	// grow without bound when a whole repository of documents is unreadable.
	ctx := context.Background()
	var logBuf bytes.Buffer
	logger := zerolog.New(&logBuf).Level(zerolog.WarnLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "bound111doc222"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(100, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	const total = 12
	for i := 0; i < total; i++ {
		f := repository.NewFileWithDetails(commitSHA, fmt.Sprintf("%02d-doc.pdf", i), "aaa", "application/pdf", ".pdf", 5000)
		_, err = fileStore.Save(ctx, f)
		require.NoError(t, err)
	}

	// The fake document source fails with an error that embeds the local disk
	// path ("no text for <diskPath>"), like os.Stat errors do, to prove local
	// paths are stripped from the aggregated error.
	docText := &fakeDocumentText{}

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		&fakeGitAdapter{}, docText, extraction.NewExtractors(), nil,
		chunking.ChunkParams{Size: 100, Overlap: 0, MinSize: 1},
		&fakeTrackerFactory{},
		logger,
	)

	payload := map[string]any{
		"repository_id": savedRepo.ID(),
		"commit_sha":    commitSHA,
	}

	err = h.Execute(ctx, payload)
	require.Error(t, err)
	msg := err.Error()
	assert.Equal(t, 10, strings.Count(msg, "extract document text from"), "only the first 10 document errors are included")
	assert.Contains(t, msg, "00-doc.pdf")
	assert.Contains(t, msg, "09-doc.pdf")
	assert.NotContains(t, msg, "10-doc.pdf", "errors beyond the cap are summarized, not listed")
	assert.NotContains(t, msg, "11-doc.pdf")
	assert.Contains(t, msg, "and 2 more documents failed to extract text")
	assert.NotContains(t, msg, tmpDir, "no local clone paths may leak even with many failures")

	// Every failure is still logged individually for diagnosis.
	for i := 0; i < total; i++ {
		assert.Contains(t, logBuf.String(), fmt.Sprintf("%02d-doc.pdf", i))
	}
}

func TestChunkFiles_ParsesCSVFiles(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	db := testdb.New(t)

	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewSourceLocationStore(db)
	repoStore := persistence.NewRepositoryStore(db)
	fileStore := persistence.NewFileStore(db)

	commitSHA := "csv111aaa222"
	tmpDir := t.TempDir()

	cc, err := repository.NewChunkingConfig(1500, 0, 1)
	require.NoError(t, err)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	require.NoError(t, err)
	repo = repo.
		WithWorkingCopy(repository.NewWorkingCopy(tmpDir, "https://github.com/test/repo")).
		WithTrackingConfig(repository.NewTrackingConfig("main", "", "")).
		WithChunkingConfig(cc)
	savedRepo, err := repoStore.Save(ctx, repo)
	require.NoError(t, err)

	// CSV with string and numeric columns.
	// "name" and "city" are strings; "age" and "score" are numeric.
	csvContent := []byte("name,age,city,score\nalice,30,london,9.5\nbob,25,paris,8.1\ncarol,35,berlin,7.9\n")

	f := repository.NewFileWithDetails(commitSHA, "data.csv", "abc123", "text/csv", ".csv", int64(len(csvContent)))
	_, err = fileStore.Save(ctx, f)
	require.NoError(t, err)

	h := NewChunkFiles(
		repoStore, enrichmentStore, associationStore, lineRangeStore, fileStore,
		&fakeGitAdapter{files: map[string][]byte{"data.csv": csvContent}}, nil, extraction.NewExtractors(), nil,
		chunking.ChunkParams{Size: 1500, Overlap: 0, MinSize: 1},
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
	require.NotEmpty(t, chunks, "CSV file should produce at least one chunk")

	// All chunk content combined.
	combined := ""
	for _, ch := range chunks {
		combined += ch.Content()
	}

	// String column values must appear.
	assert.Contains(t, combined, "alice")
	assert.Contains(t, combined, "bob")
	assert.Contains(t, combined, "carol")
	assert.Contains(t, combined, "london")
	assert.Contains(t, combined, "paris")
	assert.Contains(t, combined, "berlin")

	// Column headers must appear.
	assert.Contains(t, combined, "name")
	assert.Contains(t, combined, "city")

	// Numeric values must NOT appear in the Values section.
	// They may appear in the top-rows verbatim section, so check the Values line only.
	for _, ch := range chunks {
		for _, line := range strings.Split(ch.Content(), "\n") {
			if strings.HasPrefix(line, "Values:") {
				assert.NotContains(t, line, "30", "age values should not appear in Values line")
				assert.NotContains(t, line, "9.5", "score values should not appear in Values line")
			}
		}
	}

	// Language must be set to .csv.
	for _, ch := range chunks {
		assert.Equal(t, ".csv", ch.Language(), "chunk language should be .csv")
	}
}
