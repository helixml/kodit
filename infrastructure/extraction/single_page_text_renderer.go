package extraction

import (
	"fmt"
	"path/filepath"

	"github.com/tsawler/tabula"
)

// SinglePageTextRenderer extracts text from document formats that are treated
// as a single page (DOCX, ODT, EPUB). PageCount always returns 1 and only
// page 1 can be rendered.
type SinglePageTextRenderer struct{}

// NewSinglePageTextRenderer creates a new SinglePageTextRenderer.
func NewSinglePageTextRenderer() *SinglePageTextRenderer {
	return &SinglePageTextRenderer{}
}

// PageCount always returns 1 for single-page document formats.
func (r *SinglePageTextRenderer) PageCount(path string) (int, error) {
	if err := validateDocumentPath(path); err != nil {
		return 0, err
	}
	return 1, nil
}

// Render returns the full document text. Only page 1 is valid.
func (r *SinglePageTextRenderer) Render(path string, page int) (string, error) {
	if err := validateDocumentPath(path); err != nil {
		return "", err
	}
	if page != 1 {
		return "", fmt.Errorf("page %d out of range (1-1)", page)
	}
	text, _, err := tabula.Open(path).
		ExcludeHeadersAndFooters().
		JoinParagraphs().
		Text()
	if err != nil {
		return "", fmt.Errorf("extract text from %s: %w", filepath.Base(path), err)
	}
	return text, nil
}

// Close is a no-op; SinglePageTextRenderer holds no persistent resources.
func (r *SinglePageTextRenderer) Close() error { return nil }
