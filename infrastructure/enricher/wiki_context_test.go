package enricher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWikiContextService_Gather_Readme(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Hello"), 0644))

	svc := NewWikiContextService()
	readme, _, _, err := svc.Gather(t.Context(), dir, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "# Hello", readme)
}

func TestWikiContextService_Gather_ReadmeTruncated(t *testing.T) {
	dir := t.TempDir()
	long := make([]byte, 4000)
	for i := range long {
		long[i] = 'x'
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), long, 0644))

	svc := NewWikiContextService()
	readme, _, _, err := svc.Gather(t.Context(), dir, nil, nil)
	require.NoError(t, err)
	assert.Len(t, readme, 3000+len("\n...[truncated]"))
	assert.Contains(t, readme, "...[truncated]")
}

func TestWikiContextService_Gather_NoReadme(t *testing.T) {
	dir := t.TempDir()

	svc := NewWikiContextService()
	readme, _, _, err := svc.Gather(t.Context(), dir, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, readme)
}

func TestWikiContextService_Gather_FileTree(t *testing.T) {
	files := []repository.File{
		repository.NewFile("abc", "src/main.go", "go", 100),
		repository.NewFile("abc", "src/util.go", "go", 50),
	}

	svc := NewWikiContextService()
	_, fileTree, _, err := svc.Gather(t.Context(), t.TempDir(), files, nil)
	require.NoError(t, err)
	assert.Contains(t, fileTree, "src/main.go")
	assert.Contains(t, fileTree, "src/util.go")
}

func TestWikiContextService_Gather_FileTreeCapped(t *testing.T) {
	files := make([]repository.File, 250)
	for i := range files {
		files[i] = repository.NewFile("abc", "file"+string(rune('A'+i%26))+".go", "go", 10)
	}

	svc := NewWikiContextService()
	_, fileTree, _, err := svc.Gather(t.Context(), t.TempDir(), files, nil)
	require.NoError(t, err)
	assert.Contains(t, fileTree, "... and more files")
}

func TestWikiContextService_Gather_Enrichments(t *testing.T) {
	enrichments := []enrichment.Enrichment{
		enrichment.NewPhysicalArchitecture("architecture content"),
	}

	svc := NewWikiContextService()
	_, _, enrichmentText, err := svc.Gather(t.Context(), t.TempDir(), nil, enrichments)
	require.NoError(t, err)
	assert.Contains(t, enrichmentText, "architecture/physical")
	assert.Contains(t, enrichmentText, "architecture content")
}

func TestWikiContextService_FileContent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main"), 0644))

	svc := NewWikiContextService()
	content := svc.FileContent(dir, "test.go", 100)
	assert.Equal(t, "package main", content)
}

func TestWikiContextService_FileContent_Truncated(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.go"), []byte("abcdefghij"), 0644))

	svc := NewWikiContextService()
	content := svc.FileContent(dir, "big.go", 5)
	assert.Equal(t, "abcde\n...[truncated]", content)
}

func TestWikiContextService_FileContent_Missing(t *testing.T) {
	svc := NewWikiContextService()
	content := svc.FileContent(t.TempDir(), "nonexistent.go", 100)
	assert.Empty(t, content)
}
