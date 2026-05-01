package extraction

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inlineTrailerPDF returns a minimal valid 1-page PDF whose content stream
// displays the literal text "Hello PDFium 553". The trailer keyword is on the
// same line as its dictionary (`trailer << ... >>`) — this is the structure
// that breaks tabula's xref parser on the arxiv PDF reported in issue #553,
// while remaining valid per the PDF spec.
func inlineTrailerPDF(t *testing.T) []byte {
	t.Helper()
	stream := "BT /F1 12 Tf 100 700 Td (Hello PDFium 553) Tj ET\n"

	var body bytes.Buffer
	body.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	offsets := []int{0}
	for i, obj := range objs {
		offsets = append(offsets, body.Len())
		fmt.Fprintf(&body, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	offsets = append(offsets, body.Len())
	fmt.Fprintf(&body, "5 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n", len(stream), stream)

	xrefOffset := body.Len()
	body.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&body, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&body, "trailer << /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)
	return body.Bytes()
}

func TestPDFiumTextRenderer_PageCount(t *testing.T) {
	r, err := NewPDFiumTextRenderer()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	path := filepath.Join(t.TempDir(), "hello.pdf")
	require.NoError(t, os.WriteFile(path, inlineTrailerPDF(t), 0644))

	count, err := r.PageCount(path)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestPDFiumTextRenderer_Render(t *testing.T) {
	r, err := NewPDFiumTextRenderer()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	path := filepath.Join(t.TempDir(), "hello.pdf")
	require.NoError(t, os.WriteFile(path, inlineTrailerPDF(t), 0644))

	text, err := r.Render(path, 1)
	require.NoError(t, err)
	assert.Contains(t, text, "Hello PDFium 553", "expected the literal text from the content stream")
}

func TestPDFiumTextRenderer_RenderRejectsOutOfRangePage(t *testing.T) {
	r, err := NewPDFiumTextRenderer()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	path := filepath.Join(t.TempDir(), "hello.pdf")
	require.NoError(t, os.WriteFile(path, inlineTrailerPDF(t), 0644))

	_, err = r.Render(path, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestPDFiumTextRenderer_PageCountErrorOnMissingFile(t *testing.T) {
	r, err := NewPDFiumTextRenderer()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	_, err = r.PageCount("/nonexistent/file.pdf")
	require.Error(t, err)
}

// TestPDFiumTextRenderer_Issue553_InlineTrailer is a regression test for
// issue #553: the previous tabula-based renderer rejected PDFs whose xref
// section ended with `trailer << ... >>` on a single line (its parser only
// recognised the `trailer` keyword when it was the entire line). This format
// is valid per the PDF spec and is emitted by mainstream tooling — for example
// arxiv preprints. PDFium handles it without complaint.
func TestPDFiumTextRenderer_Issue553_InlineTrailer(t *testing.T) {
	r, err := NewPDFiumTextRenderer()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	path := filepath.Join(t.TempDir(), "issue553.pdf")
	pdf := inlineTrailerPDF(t)
	require.Contains(t, string(pdf), "trailer << /Size", "fixture must keep the inline-trailer pattern that triggered issue #553")
	require.NoError(t, os.WriteFile(path, pdf, 0644))

	text, err := r.Render(path, 1)
	require.NoError(t, err, "PDFium must not regress to tabula's xref-parser failure on inline trailers")
	assert.NotEmpty(t, text, "extracted text must be non-empty so search hits do not have empty Content")
}
