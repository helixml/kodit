package extraction

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDocument(t *testing.T) {
	supported := []string{".pdf", ".docx", ".odt", ".xlsx", ".pptx", ".epub"}
	for _, ext := range supported {
		assert.True(t, IsDocument(ext), "expected %s to be a document extension", ext)
	}

	unsupported := []string{".go", ".py", ".txt", ".md", ".json", ".png", ""}
	for _, ext := range unsupported {
		assert.False(t, IsDocument(ext), "expected %s to not be a document extension", ext)
	}
}

func TestIsDocument_CaseInsensitive(t *testing.T) {
	assert.True(t, IsDocument(".PDF"))
	assert.True(t, IsDocument(".Docx"))
}

func TestDocumentText_ErrorOnMissingFile(t *testing.T) {
	d := NewDocumentText()
	_, err := d.Text("/nonexistent/file.pdf")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file.pdf")
}

func TestDocumentText_RejectsUnsupportedExtension(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(tmp, []byte("hello"), 0644))

	d := NewDocumentText()
	_, err := d.Text(tmp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported document type")
}

func TestDocumentText_RejectsOversizedFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "huge.pdf")
	// Create a sparse file that reports a size exceeding the limit.
	f, err := os.Create(tmp)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(maxDocumentSize+1))
	require.NoError(t, f.Close())

	d := NewDocumentText()
	_, err = d.Text(tmp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum document size")
}
