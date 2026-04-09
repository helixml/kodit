package extraction

import (
	"fmt"
	"io"
	"strings"
)

// TextRenderer extracts text from individual document pages.
// For PDFs this means pages; for spreadsheets, sheets; for presentations, slides.
type TextRenderer interface {
	io.Closer

	// PageCount returns the number of extractable pages in the document.
	PageCount(path string) (int, error)

	// Render returns the text content of the given 1-based page.
	Render(path string, page int) (string, error)
}

// TextRendererRegistry maps file extensions to TextRenderer implementations.
type TextRendererRegistry struct {
	renderers map[string]TextRenderer
}

// NewTextRendererRegistry creates an empty TextRendererRegistry.
func NewTextRendererRegistry() *TextRendererRegistry {
	return &TextRendererRegistry{renderers: make(map[string]TextRenderer)}
}

// Register associates a file extension (e.g. ".pdf") with a TextRenderer.
func (r *TextRendererRegistry) Register(ext string, renderer TextRenderer) {
	r.renderers[strings.ToLower(ext)] = renderer
}

// For returns the TextRenderer for the given extension, or nil and false if none is registered.
func (r *TextRendererRegistry) For(ext string) (TextRenderer, bool) {
	renderer, ok := r.renderers[strings.ToLower(ext)]
	return renderer, ok
}

// Supports returns true if a TextRenderer is registered for the given extension.
func (r *TextRendererRegistry) Supports(ext string) bool {
	_, ok := r.renderers[strings.ToLower(ext)]
	return ok
}

// Close closes all registered text renderers, deduplicating shared instances.
func (r *TextRendererRegistry) Close() error {
	seen := make(map[TextRenderer]bool)
	var errs []error
	for ext, renderer := range r.renderers {
		if seen[renderer] {
			continue
		}
		seen[renderer] = true
		if err := renderer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s text renderer: %w", ext, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("closing text renderers: %v", errs)
	}
	return nil
}
