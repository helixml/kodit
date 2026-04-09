package extraction

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tsawler/tabula"
)

// PDFTextRenderer extracts text from individual PDF pages using tabula.
type PDFTextRenderer struct{}

// NewPDFTextRenderer creates a new PDFTextRenderer.
func NewPDFTextRenderer() *PDFTextRenderer {
	return &PDFTextRenderer{}
}

// PageCount returns the number of pages in the PDF.
func (r *PDFTextRenderer) PageCount(path string) (int, error) {
	if err := validateDocumentPath(path); err != nil {
		return 0, err
	}
	ext := tabula.Open(path)
	defer ext.Close()
	return ext.PageCount()
}

// Render returns the text content of the given 1-based page.
func (r *PDFTextRenderer) Render(path string, page int) (string, error) {
	if err := validateDocumentPath(path); err != nil {
		return "", err
	}
	ext := tabula.Open(path)
	defer ext.Close()

	count, err := ext.PageCount()
	if err != nil {
		return "", fmt.Errorf("get page count for %s: %w", filepath.Base(path), err)
	}
	if page < 1 || page > count {
		return "", fmt.Errorf("page %d out of range (1-%d)", page, count)
	}

	text, _, err := ext.
		Pages(page).
		ExcludeHeadersAndFooters().
		JoinParagraphs().
		Text()
	if err != nil {
		return "", fmt.Errorf("extract text from page %d of %s: %w", page, filepath.Base(path), err)
	}
	return text, nil
}

// Close is a no-op; PDFTextRenderer holds no persistent resources.
func (r *PDFTextRenderer) Close() error { return nil }

// validateDocumentPath checks that the file exists and is within size limits.
func validateDocumentPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", filepath.Base(path), err)
	}
	if info.Size() > maxDocumentSize {
		return fmt.Errorf("%s exceeds maximum document size (%d bytes)", filepath.Base(path), maxDocumentSize)
	}
	return nil
}
