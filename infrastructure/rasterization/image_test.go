package rasterization

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTestImage(t *testing.T, dir, name string, c color.Color) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
	return path
}

func TestStandaloneImage_PageCount(t *testing.T) {
	rast := NewStandaloneImage("/any")
	count, err := rast.PageCount("/any/path.png")
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestStandaloneImage_Render(t *testing.T) {
	dir := t.TempDir()
	path := writeTestImage(t, dir, "photo.png", color.RGBA{R: 255, A: 255})

	rast := NewStandaloneImage(dir)
	img, err := rast.Render(path, 1)
	require.NoError(t, err)
	require.NotNil(t, img)

	bounds := img.Bounds()
	require.Equal(t, 4, bounds.Dx())
	require.Equal(t, 4, bounds.Dy())

	r, g, b, a := img.At(0, 0).RGBA()
	require.Equal(t, uint32(0xffff), r)
	require.Equal(t, uint32(0), g)
	require.Equal(t, uint32(0), b)
	require.Equal(t, uint32(0xffff), a)
}

func TestStandaloneImage_Render_OutOfRange(t *testing.T) {
	dir := t.TempDir()
	path := writeTestImage(t, dir, "photo.png", color.RGBA{R: 255, A: 255})

	rast := NewStandaloneImage(dir)

	_, err := rast.Render(path, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")

	_, err = rast.Render(path, 2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "out of range")
}

func TestStandaloneImage_Render_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	rast := NewStandaloneImage(dir)
	_, err := rast.Render(filepath.Join(dir, "nonexistent.png"), 1)
	require.Error(t, err)
}

func TestStandaloneImage_Render_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	rast := NewStandaloneImage(dir)

	_, err := rast.Render(filepath.Join(dir, "..", "etc", "passwd"), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside base directory")
}

func TestStandaloneImage_Close(t *testing.T) {
	rast := NewStandaloneImage("/any")
	require.NoError(t, rast.Close())
}
