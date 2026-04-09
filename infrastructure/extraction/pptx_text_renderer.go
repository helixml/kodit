package extraction

import (
	"fmt"
	"path/filepath"

	"github.com/tsawler/tabula/pptx"
)

// PPTXTextRenderer extracts text from individual presentation slides using tabula.
type PPTXTextRenderer struct{}

// NewPPTXTextRenderer creates a new PPTXTextRenderer.
func NewPPTXTextRenderer() *PPTXTextRenderer {
	return &PPTXTextRenderer{}
}

// PageCount returns the number of slides in the presentation.
func (r *PPTXTextRenderer) PageCount(path string) (int, error) {
	if err := validateDocumentPath(path); err != nil {
		return 0, err
	}
	pr, err := pptx.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open pptx %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = pr.Close() }()
	return pr.PageCount()
}

// Render returns the text content of the given 1-based slide.
func (r *PPTXTextRenderer) Render(path string, page int) (string, error) {
	if err := validateDocumentPath(path); err != nil {
		return "", err
	}
	pr, err := pptx.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pptx %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = pr.Close() }()

	count, err := pr.PageCount()
	if err != nil {
		return "", fmt.Errorf("get slide count for %s: %w", filepath.Base(path), err)
	}
	if page < 1 || page > count {
		return "", fmt.Errorf("page %d out of range (1-%d)", page, count)
	}

	text, err := pr.TextWithOptions(pptx.ExtractOptions{
		SlideNumbers:   []int{page - 1},
		IncludeNotes:   true,
		IncludeTitles:  true,
		ExcludeHeaders: true,
		ExcludeFooters: true,
	})
	if err != nil {
		return "", fmt.Errorf("extract text from slide %d of %s: %w", page, filepath.Base(path), err)
	}
	return text, nil
}

// Close is a no-op; PPTXTextRenderer holds no persistent resources.
func (r *PPTXTextRenderer) Close() error { return nil }
