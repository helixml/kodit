package rasterization

import (
	"fmt"
	"image"
	// Decoders registered in office.go cover gif, jpeg, png, bmp, tiff, webp.
	"os"
	"path/filepath"
	"strings"
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
func (s *StandaloneImage) Render(path string, page int) (image.Image, error) {
	if page != 1 {
		return nil, fmt.Errorf("page %d out of range [1, 1]", page)
	}

	clean := filepath.Clean(path)
	if !strings.HasPrefix(clean, s.baseDir+string(filepath.Separator)) && clean != s.baseDir {
		return nil, fmt.Errorf("path %s is outside base directory", path)
	}

	f, err := os.Open(clean)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", clean, err)
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", clean, err)
	}
	return img, nil
}

// Close is a no-op.
func (s *StandaloneImage) Close() error { return nil }
