package extraction

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
