package rasterization

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeTestOffice creates a minimal ZIP file (simulating an Office Open XML
// document) with the given entries. Each entry maps a ZIP path to raw bytes.
func writeTestOffice(t *testing.T, name string, entries map[string][]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	require.NoError(t, err)

	w := zip.NewWriter(f)
	for k, v := range entries {
		fw, werr := w.Create(k)
		require.NoError(t, werr)
		_, werr = fw.Write(v)
		require.NoError(t, werr)
	}
	require.NoError(t, w.Close())
	require.NoError(t, f.Close())
	return path
}

// solidPNG returns a 2x2 PNG image filled with the given color.
func solidPNG(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestOfficeImageExtractor_PageCount(t *testing.T) {
	img := solidPNG(t, color.RGBA{R: 255, A: 255})
	path := writeTestOffice(t, "slides.pptx", map[string][]byte{
		"ppt/media/image1.png":  img,
		"ppt/media/image2.png":  img,
		"ppt/media/image3.jpeg": img, // wrong extension for content, but counts
		"ppt/slides/slide1.xml": []byte("<xml/>"),
		"ppt/media/chart1.emf":  []byte("not an image"), // unsupported ext
	})

	ext := NewOfficeImageExtractor()
	count, err := ext.PageCount(path)
	require.NoError(t, err)
	require.Equal(t, 3, count) // image1.png, image2.png, image3.jpeg — emf excluded
}

func TestOfficeImageExtractor_PageCount_Docx(t *testing.T) {
	img := solidPNG(t, color.RGBA{R: 255, A: 255})
	path := writeTestOffice(t, "doc.docx", map[string][]byte{
		"word/media/photo.png": img,
		"word/document.xml":    []byte("<xml/>"),
	})

	ext := NewOfficeImageExtractor()
	count, err := ext.PageCount(path)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestOfficeImageExtractor_PageCount_Xlsx(t *testing.T) {
	img := solidPNG(t, color.RGBA{R: 255, A: 255})
	path := writeTestOffice(t, "sheet.xlsx", map[string][]byte{
		"xl/media/logo.png":   img,
		"xl/media/chart.jpg":  img,
		"xl/worksheets/s.xml": []byte("<xml/>"),
	})

	ext := NewOfficeImageExtractor()
	count, err := ext.PageCount(path)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestOfficeImageExtractor_PageCount_NoImages(t *testing.T) {
	path := writeTestOffice(t, "empty.docx", map[string][]byte{
		"word/document.xml": []byte("<xml/>"),
	})

	ext := NewOfficeImageExtractor()
	count, err := ext.PageCount(path)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestOfficeImageExtractor_Render(t *testing.T) {
	img := solidPNG(t, color.RGBA{R: 255, A: 255})
	path := writeTestOffice(t, "slides.pptx", map[string][]byte{
		"ppt/media/image1.png": img,
		"ppt/media/image2.png": img,
	})

	ext := NewOfficeImageExtractor()
	result, err := ext.Render(path, 1)
	require.NoError(t, err)
	require.NotNil(t, result)

	bounds := result.Bounds()
	require.Equal(t, 2, bounds.Dx())
	require.Equal(t, 2, bounds.Dy())

	// Verify the pixel is red.
	r, g, b, a := result.At(0, 0).RGBA()
	require.Equal(t, uint32(0xffff), r)
	require.Equal(t, uint32(0), g)
	require.Equal(t, uint32(0), b)
	require.Equal(t, uint32(0xffff), a)
}

func TestOfficeImageExtractor_Render_Deterministic(t *testing.T) {
	red := solidPNG(t, color.RGBA{R: 255, A: 255})
	blue := solidPNG(t, color.RGBA{B: 255, A: 255})
	path := writeTestOffice(t, "slides.pptx", map[string][]byte{
		"ppt/media/b_second.png": blue,
		"ppt/media/a_first.png":  red,
	})

	ext := NewOfficeImageExtractor()

	// Pages are sorted by path — a_first (red) comes before b_second (blue).
	r1, err := ext.Render(path, 1)
	require.NoError(t, err)
	r, _, _, _ := r1.At(0, 0).RGBA()
	require.Equal(t, uint32(0xffff), r, "page 1 should be red (a_first)")

	r2, err := ext.Render(path, 2)
	require.NoError(t, err)
	_, _, b, _ := r2.At(0, 0).RGBA()
	require.Equal(t, uint32(0xffff), b, "page 2 should be blue (b_second)")
}

func TestOfficeImageExtractor_Render_OutOfRange(t *testing.T) {
	img := solidPNG(t, color.RGBA{R: 255, A: 255})
	path := writeTestOffice(t, "slides.pptx", map[string][]byte{
		"ppt/media/image1.png": img,
	})

	ext := NewOfficeImageExtractor()

	_, err := ext.Render(path, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")

	_, err = ext.Render(path, 2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")
}

func TestOfficeImageExtractor_NotAZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notazip.docx")
	require.NoError(t, os.WriteFile(path, []byte("not a zip file"), 0o644))

	ext := NewOfficeImageExtractor()

	_, err := ext.PageCount(path)
	require.Error(t, err)

	_, err = ext.Render(path, 1)
	require.Error(t, err)
}

func TestOfficeImageExtractor_EMFExcluded(t *testing.T) {
	img := solidPNG(t, color.RGBA{R: 255, A: 255})
	path := writeTestOffice(t, "slides.pptx", map[string][]byte{
		"ppt/media/image1.png":  img,
		"ppt/media/vector.emf":  []byte("emf data"),
		"ppt/media/vector2.wmf": []byte("wmf data"),
		"ppt/media/icon.svg":    []byte("<svg/>"),
	})

	ext := NewOfficeImageExtractor()
	count, err := ext.PageCount(path)
	require.NoError(t, err)
	require.Equal(t, 1, count) // only image1.png
}

func TestOfficeImageExtractor_Close(t *testing.T) {
	ext := NewOfficeImageExtractor()
	require.NoError(t, ext.Close())
}
