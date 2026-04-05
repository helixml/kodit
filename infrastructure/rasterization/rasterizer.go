// Package rasterization converts document pages to images.
// It provides a generic Rasterizer interface and a Registry that dispatches
// by file extension, supporting PDFs now and extensible to spreadsheets,
// presentations, and other document types.
package rasterization

import (
	"fmt"
	"image"
	"io"
	"strings"
)

// Rasterizer converts document pages to images.
// For PDFs this means pages; for spreadsheets, sheets; for presentations, slides.
type Rasterizer interface {
	io.Closer

	// PageCount returns the number of renderable pages in the document.
	PageCount(path string) (int, error)

	// Render returns the given 1-based page as an image.
	Render(path string, page int) (image.Image, error)
}

// Registry maps file extensions to Rasterizer implementations.
type Registry struct {
	rasterizers map[string]Rasterizer
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{rasterizers: make(map[string]Rasterizer)}
}

// Register associates a file extension (e.g. ".pdf") with a Rasterizer.
func (r *Registry) Register(ext string, rasterizer Rasterizer) {
	r.rasterizers[strings.ToLower(ext)] = rasterizer
}

// For returns the Rasterizer for the given extension, or nil and false if none is registered.
func (r *Registry) For(ext string) (Rasterizer, bool) {
	rast, ok := r.rasterizers[strings.ToLower(ext)]
	return rast, ok
}

// Supports returns true if a Rasterizer is registered for the given extension.
func (r *Registry) Supports(ext string) bool {
	_, ok := r.rasterizers[strings.ToLower(ext)]
	return ok
}

// Close closes all registered rasterizers.
func (r *Registry) Close() error {
	var errs []error
	for ext, rast := range r.rasterizers {
		if err := rast.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s rasterizer: %w", ext, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("closing rasterizers: %v", errs)
	}
	return nil
}
