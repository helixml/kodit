package rasterization

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPdfiumRasterizer_Available(t *testing.T) {
	rast, err := NewPdfiumRasterizer()
	require.NoError(t, err)
	require.NotNil(t, rast, "pdfium rasterizer must be available")
	defer func() { _ = rast.Close() }()
}

// whitePDF returns a minimal valid PDF with a single white 200x200 page.
// Built from raw bytes so no PDF-generation library is needed.
func whitePDF() []byte {
	return []byte(`%PDF-1.0
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]>>endobj
xref
0 4
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000107 00000 n
trailer<</Size 4/Root 1 0 R>>
startxref
178
%%EOF`)
}

func TestPdfiumRasterizer_RenderWhitePage(t *testing.T) {
	rast, err := NewPdfiumRasterizer()
	require.NoError(t, err)
	defer func() { _ = rast.Close() }()

	path := filepath.Join(t.TempDir(), "white.pdf")
	require.NoError(t, os.WriteFile(path, whitePDF(), 0o644))

	img, err := rast.Render(path, 1)
	require.NoError(t, err)

	rgba, ok := img.(*image.RGBA)
	require.True(t, ok, "expected *image.RGBA, got %T", img)

	bounds := rgba.Bounds()
	require.True(t, bounds.Dx() > 0 && bounds.Dy() > 0, "image must not be empty")

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := (y-bounds.Min.Y)*rgba.Stride + (x-bounds.Min.X)*4
			r, g, b, a := rgba.Pix[off], rgba.Pix[off+1], rgba.Pix[off+2], rgba.Pix[off+3]
			if r < 250 || g < 250 || b < 250 || a < 250 {
				t.Fatalf("pixel (%d,%d) is not white: RGBA(%d,%d,%d,%d) — possible corrupted WASM memory", x, y, r, g, b, a)
			}
		}
	}
}
