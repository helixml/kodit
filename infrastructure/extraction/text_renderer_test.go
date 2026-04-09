package extraction

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- TextRendererRegistry ---

func TestTextRendererRegistry_ForUnregistered(t *testing.T) {
	reg := NewTextRendererRegistry()
	_, ok := reg.For(".pdf")
	assert.False(t, ok)
}

func TestTextRendererRegistry_RegisterAndRetrieve(t *testing.T) {
	reg := NewTextRendererRegistry()
	renderer := NewPDFTextRenderer()
	reg.Register(".pdf", renderer)

	got, ok := reg.For(".pdf")
	assert.True(t, ok)
	assert.Equal(t, renderer, got)
}

func TestTextRendererRegistry_CaseInsensitive(t *testing.T) {
	reg := NewTextRendererRegistry()
	reg.Register(".PDF", NewPDFTextRenderer())

	_, ok := reg.For(".pdf")
	assert.True(t, ok)
}

func TestTextRendererRegistry_Supports(t *testing.T) {
	reg := NewTextRendererRegistry()
	reg.Register(".xlsx", NewXLSXTextRenderer())

	assert.True(t, reg.Supports(".xlsx"))
	assert.False(t, reg.Supports(".pdf"))
}

func TestTextRendererRegistry_Close(t *testing.T) {
	reg := NewTextRendererRegistry()
	reg.Register(".pdf", NewPDFTextRenderer())
	reg.Register(".xlsx", NewXLSXTextRenderer())
	assert.NoError(t, reg.Close())
}

// --- PDFTextRenderer ---

func TestPDFTextRenderer_ErrorOnMissingFile(t *testing.T) {
	r := NewPDFTextRenderer()
	_, err := r.PageCount("/nonexistent/file.pdf")
	assert.Error(t, err)

	_, err = r.Render("/nonexistent/file.pdf", 1)
	assert.Error(t, err)
}

func TestPDFTextRenderer_ErrorOnOversizedFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "huge.pdf")
	f, err := os.Create(tmp)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(maxDocumentSize+1))
	require.NoError(t, f.Close())

	r := NewPDFTextRenderer()
	_, err = r.PageCount(tmp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum document size")
}

// --- XLSXTextRenderer ---

func TestXLSXTextRenderer_ErrorOnMissingFile(t *testing.T) {
	r := NewXLSXTextRenderer()
	_, err := r.PageCount("/nonexistent/file.xlsx")
	assert.Error(t, err)

	_, err = r.Render("/nonexistent/file.xlsx", 1)
	assert.Error(t, err)
}

func TestXLSXTextRenderer_ErrorOnOversizedFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "huge.xlsx")
	f, err := os.Create(tmp)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(maxDocumentSize+1))
	require.NoError(t, f.Close())

	r := NewXLSXTextRenderer()
	_, err = r.PageCount(tmp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum document size")
}

// --- PPTXTextRenderer ---

func TestPPTXTextRenderer_ErrorOnMissingFile(t *testing.T) {
	r := NewPPTXTextRenderer()
	_, err := r.PageCount("/nonexistent/file.pptx")
	assert.Error(t, err)

	_, err = r.Render("/nonexistent/file.pptx", 1)
	assert.Error(t, err)
}

func TestPPTXTextRenderer_ErrorOnOversizedFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "huge.pptx")
	f, err := os.Create(tmp)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(maxDocumentSize+1))
	require.NoError(t, f.Close())

	r := NewPPTXTextRenderer()
	_, err = r.PageCount(tmp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum document size")
}

// --- SinglePageTextRenderer ---

func TestSinglePageTextRenderer_ErrorOnMissingFile(t *testing.T) {
	r := NewSinglePageTextRenderer()
	_, err := r.PageCount("/nonexistent/file.docx")
	assert.Error(t, err)

	_, err = r.Render("/nonexistent/file.docx", 1)
	assert.Error(t, err)
}

func TestSinglePageTextRenderer_ErrorOnOversizedFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "huge.docx")
	f, err := os.Create(tmp)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(maxDocumentSize+1))
	require.NoError(t, f.Close())

	r := NewSinglePageTextRenderer()
	_, err = r.PageCount(tmp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum document size")
}

func TestSinglePageTextRenderer_RejectsInvalidPage(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.docx")
	require.NoError(t, os.WriteFile(tmp, []byte("dummy"), 0644))

	r := NewSinglePageTextRenderer()
	_, err := r.Render(tmp, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "page 2 out of range (1-1)")
}
