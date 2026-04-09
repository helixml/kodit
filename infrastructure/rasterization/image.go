package rasterization

import (
	"fmt"
	"image"
	// Decoders registered in office.go cover gif, jpeg, png, bmp, tiff, webp.
	"os"
)

// StandaloneImage is a Rasterizer for plain image files.
// Each file is treated as a single "page".
type StandaloneImage struct{}

// NewStandaloneImage creates a StandaloneImage rasterizer.
func NewStandaloneImage() *StandaloneImage {
	return &StandaloneImage{}
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

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}

// Close is a no-op.
func (s *StandaloneImage) Close() error { return nil }
