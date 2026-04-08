package rasterization

import (
	"crypto/sha256"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
)

const jpegQuality = 80

// Cache renders document pages on demand and caches the resulting JPEGs
// on the filesystem. The cache is ephemeral — lost on container restart,
// recomputed when accessed again.
type Cache struct {
	registry *Registry
	cacheDir string
}

// NewCache creates a Cache backed by the given Registry.
// The cacheDir is created if it does not exist.
func NewCache(registry *Registry, cacheDir string) (*Cache, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &Cache{registry: registry, cacheDir: cacheDir}, nil
}

// Supports returns true if the registry can rasterize files with the given extension.
func (c *Cache) Supports(ext string) bool {
	return c.registry.Supports(ext)
}

// Image returns JPEG bytes for the given page of the document at path.
// Results are cached on disk; a cache miss triggers rendering.
func (c *Cache) Image(path string, page int) ([]byte, error) {
	cachePath := c.cachePath(path, page)

	data, err := os.ReadFile(cachePath)
	if err == nil {
		return data, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	rast, ok := c.registry.For(ext)
	if !ok {
		return nil, fmt.Errorf("no rasterizer for extension %q", ext)
	}

	img, err := rast.Render(path, page)
	if err != nil {
		return nil, fmt.Errorf("render page %d of %s: %w", page, filepath.Base(path), err)
	}

	f, err := os.Create(cachePath)
	if err != nil {
		return nil, fmt.Errorf("create cache file: %w", err)
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}

	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("close cache file: %w", err)
	}

	return os.ReadFile(cachePath)
}

// cachePath returns the filesystem path for a cached page image.
func (c *Cache) cachePath(path string, page int) string {
	h := sha256.Sum256([]byte(path))
	name := fmt.Sprintf("%x-page%d.jpg", h[:8], page)
	return filepath.Join(c.cacheDir, name)
}
