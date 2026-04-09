package rasterization

import (
	"fmt"
	"image"
	// Decoders registered in office.go cover gif, jpeg, png, bmp, tiff, webp.
	"os"
	"path/filepath"
	"strings" // used for traversal check in Render
)

// StandaloneImage is a Rasterizer for plain image files.
// Each file is treated as a single "page". Paths are validated
// against a base directory to prevent path traversal.
type StandaloneImage struct {
	baseDir string
}

// NewStandaloneImage creates a StandaloneImage rasterizer.
// All rendered paths must resolve within baseDir.
func NewStandaloneImage(baseDir string) *StandaloneImage {
	return &StandaloneImage{baseDir: filepath.Clean(baseDir)}
}

// PageCount always returns 1 — a standalone image is a single page.
func (s *StandaloneImage) PageCount(_ string) (int, error) {
	return 1, nil
}

// Render decodes the image file and returns it.
// The path is validated against the base directory to prevent traversal.
func (s *StandaloneImage) Render(path string, page int) (image.Image, error) {
	if page != 1 {
		return nil, fmt.Errorf("page %d out of range [1, 1]", page)
	}

	// Compute a relative path from the base directory and reject traversal.
	rel, err := filepath.Rel(s.baseDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("path %s is outside base directory", path)
	}

	// Re-join to produce a clean, verified absolute path.
	safe := filepath.Join(s.baseDir, rel)

	f, err := os.Open(safe)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", safe, err)
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", safe, err)
	}
	return img, nil
}

// Close is a no-op.
func (s *StandaloneImage) Close() error { return nil }
