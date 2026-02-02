package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIgnorePattern(t *testing.T) {
	t.Run("valid directory", func(t *testing.T) {
		dir := t.TempDir()
		pattern, err := NewIgnorePattern(dir)
		require.NoError(t, err)
		assert.Equal(t, dir, pattern.base)
	})

	t.Run("non-existent path", func(t *testing.T) {
		_, err := NewIgnorePattern("/nonexistent/path")
		assert.Error(t, err)
	})

	t.Run("file instead of directory", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "file.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

		_, err := NewIgnorePattern(filePath)
		assert.Error(t, err)
		assert.IsType(t, &NotDirectoryError{}, err)
	})
}

func TestIgnorePattern_ShouldIgnore(t *testing.T) {
	t.Run("directories are never ignored", func(t *testing.T) {
		dir := t.TempDir()
		subdir := filepath.Join(dir, "subdir")
		require.NoError(t, os.Mkdir(subdir, 0755))

		pattern, err := NewIgnorePattern(dir)
		require.NoError(t, err)

		assert.False(t, pattern.ShouldIgnore(subdir))
	})

	t.Run("files in .git directory are ignored", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		require.NoError(t, os.Mkdir(gitDir, 0755))
		gitFile := filepath.Join(gitDir, "config")
		require.NoError(t, os.WriteFile(gitFile, []byte("content"), 0644))

		pattern, err := NewIgnorePattern(dir)
		require.NoError(t, err)

		assert.True(t, pattern.ShouldIgnore(gitFile))
	})

	t.Run("regular files are not ignored by default", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "regular.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

		pattern, err := NewIgnorePattern(dir)
		require.NoError(t, err)

		assert.False(t, pattern.ShouldIgnore(filePath))
	})

	t.Run("non-existent files return false", func(t *testing.T) {
		dir := t.TempDir()

		pattern, err := NewIgnorePattern(dir)
		require.NoError(t, err)

		assert.False(t, pattern.ShouldIgnore(filepath.Join(dir, "nonexistent.txt")))
	})
}

func TestIgnorePattern_NoIndexRules(t *testing.T) {
	t.Run("matches .noindex patterns", func(t *testing.T) {
		dir := t.TempDir()

		// Create .noindex file
		noindexContent := "*.log\ntemp/\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, ".noindex"),
			[]byte(noindexContent),
			0644,
		))

		// Create files
		logFile := filepath.Join(dir, "app.log")
		txtFile := filepath.Join(dir, "readme.txt")
		require.NoError(t, os.WriteFile(logFile, []byte("log"), 0644))
		require.NoError(t, os.WriteFile(txtFile, []byte("text"), 0644))

		pattern, err := NewIgnorePattern(dir)
		require.NoError(t, err)

		assert.True(t, pattern.ShouldIgnore(logFile), "*.log should be ignored")
		assert.False(t, pattern.ShouldIgnore(txtFile), "*.txt should not be ignored")
	})

	t.Run("handles empty lines and comments in .noindex", func(t *testing.T) {
		dir := t.TempDir()

		noindexContent := "# Comment line\n\n*.tmp\n  \n"
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, ".noindex"),
			[]byte(noindexContent),
			0644,
		))

		tmpFile := filepath.Join(dir, "cache.tmp")
		require.NoError(t, os.WriteFile(tmpFile, []byte("temp"), 0644))

		pattern, err := NewIgnorePattern(dir)
		require.NoError(t, err)

		assert.True(t, pattern.ShouldIgnore(tmpFile))
	})
}

func TestNotDirectoryError(t *testing.T) {
	err := &NotDirectoryError{Path: "/some/path"}
	assert.Contains(t, err.Error(), "/some/path")
	assert.Contains(t, err.Error(), "not a directory")
}
