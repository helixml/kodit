package extraction

import (
	"fmt"
	"path/filepath"

	"github.com/tsawler/tabula/xlsx"
)

// XLSXTextRenderer extracts text from individual spreadsheet sheets using tabula.
type XLSXTextRenderer struct{}

// NewXLSXTextRenderer creates a new XLSXTextRenderer.
func NewXLSXTextRenderer() *XLSXTextRenderer {
	return &XLSXTextRenderer{}
}

// PageCount returns the number of sheets in the workbook.
func (r *XLSXTextRenderer) PageCount(path string) (int, error) {
	if err := validateDocumentPath(path); err != nil {
		return 0, err
	}
	xr, err := xlsx.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open xlsx %s: %w", filepath.Base(path), err)
	}
	defer xr.Close()
	return xr.PageCount()
}

// Render returns the text content of the given 1-based sheet.
func (r *XLSXTextRenderer) Render(path string, page int) (string, error) {
	if err := validateDocumentPath(path); err != nil {
		return "", err
	}
	xr, err := xlsx.Open(path)
	if err != nil {
		return "", fmt.Errorf("open xlsx %s: %w", filepath.Base(path), err)
	}
	defer xr.Close()

	count, err := xr.PageCount()
	if err != nil {
		return "", fmt.Errorf("get sheet count for %s: %w", filepath.Base(path), err)
	}
	if page < 1 || page > count {
		return "", fmt.Errorf("page %d out of range (1-%d)", page, count)
	}

	text, err := xr.TextWithOptions(xlsx.ExtractOptions{
		Sheets:         []int{page - 1},
		IncludeHeaders: true,
	})
	if err != nil {
		return "", fmt.Errorf("extract text from sheet %d of %s: %w", page, filepath.Base(path), err)
	}
	return text, nil
}

// Close is a no-op; XLSXTextRenderer holds no persistent resources.
func (r *XLSXTextRenderer) Close() error { return nil }
