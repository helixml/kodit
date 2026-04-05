package rasterization

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRasterizer struct {
	pages       map[string]int
	renderCalls int
}

func (f *fakeRasterizer) PageCount(path string) (int, error) {
	n, ok := f.pages[path]
	if !ok {
		return 0, os.ErrNotExist
	}
	return n, nil
}

func (f *fakeRasterizer) Render(_ string, _ int) (image.Image, error) {
	f.renderCalls++
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	return img, nil
}

func (f *fakeRasterizer) Close() error { return nil }

func TestCache_ImageCacheMiss(t *testing.T) {
	cacheDir := t.TempDir()
	rast := &fakeRasterizer{pages: map[string]int{"/test.pdf": 2}}

	reg := NewRegistry()
	reg.Register(".pdf", rast)

	cache, err := NewCache(reg, cacheDir)
	require.NoError(t, err)

	data, err := cache.Image("/test.pdf", 1)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Equal(t, 1, rast.renderCalls)

	// Verify file was written to cache.
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestCache_ImageCacheHit(t *testing.T) {
	cacheDir := t.TempDir()
	rast := &fakeRasterizer{pages: map[string]int{"/test.pdf": 1}}

	reg := NewRegistry()
	reg.Register(".pdf", rast)

	cache, err := NewCache(reg, cacheDir)
	require.NoError(t, err)

	// First call: cache miss.
	data1, err := cache.Image("/test.pdf", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, rast.renderCalls)

	// Second call: cache hit.
	data2, err := cache.Image("/test.pdf", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, rast.renderCalls, "should not render again")
	assert.Equal(t, data1, data2)
}

func TestCache_Supports(t *testing.T) {
	reg := NewRegistry()
	reg.Register(".pdf", &fakeRasterizer{})

	cache, err := NewCache(reg, t.TempDir())
	require.NoError(t, err)

	assert.True(t, cache.Supports(".pdf"))
	assert.True(t, cache.Supports(".PDF"))
	assert.False(t, cache.Supports(".xlsx"))
}

func TestCache_UnsupportedExtension(t *testing.T) {
	reg := NewRegistry()
	cache, err := NewCache(reg, t.TempDir())
	require.NoError(t, err)

	_, err = cache.Image("/test.xlsx", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rasterizer")
}

func TestCache_CreatesCacheDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "cache")
	reg := NewRegistry()
	_, err := NewCache(reg, dir)
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
