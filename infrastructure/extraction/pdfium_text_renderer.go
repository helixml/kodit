package extraction

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

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

// PDFiumTextRenderer extracts text from PDF pages using PDFium via the Wazero
// WebAssembly runtime — the same engine that drives Chrome's PDF viewer. It is
// substantially more tolerant of malformed cross-reference tables and content
// streams than tabula, which fails on common real-world PDFs (issue #553).
//
// The underlying pdfium.Pdfium instance wraps a single WASM module whose
// internal state is not safe for concurrent use, so mu serialises calls.
type PDFiumTextRenderer struct {
	pool     pdfium.Pool
	instance pdfium.Pdfium
	mu       sync.Mutex
}

// NewPDFiumTextRenderer initialises a PDFium WASM pool and returns a
// TextRenderer for PDF files.
func NewPDFiumTextRenderer() (*PDFiumTextRenderer, error) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("init pdfium wasm pool: %w", err)
	}
	inst, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("get pdfium instance: %w", err)
	}
	return &PDFiumTextRenderer{pool: pool, instance: inst}, nil
}

// PageCount returns the number of pages in the PDF at the given path.
func (r *PDFiumTextRenderer) PageCount(path string) (int, error) {
	if err := validateDocumentPath(path); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, err := r.instance.OpenDocument(&requests.OpenDocument{FilePath: &path})
	if err != nil {
		return 0, fmt.Errorf("open pdf %s: %w", filepath.Base(path), err)
	}
	defer func() {
		_, _ = r.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	}()

	resp, err := r.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return 0, fmt.Errorf("get page count for %s: %w", filepath.Base(path), err)
	}
	return resp.PageCount, nil
}

// Render returns the text content of the given 1-based page.
func (r *PDFiumTextRenderer) Render(path string, page int) (string, error) {
	if err := validateDocumentPath(path); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, err := r.instance.OpenDocument(&requests.OpenDocument{FilePath: &path})
	if err != nil {
		return "", fmt.Errorf("open pdf %s: %w", filepath.Base(path), err)
	}
	defer func() {
		_, _ = r.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	}()

	count, err := r.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return "", fmt.Errorf("get page count for %s: %w", filepath.Base(path), err)
	}
	if page < 1 || page > count.PageCount {
		return "", fmt.Errorf("page %d out of range (1-%d)", page, count.PageCount)
	}

	pageIndex := page - 1
	resp, err := r.instance.GetPageText(&requests.GetPageText{
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{Document: doc.Document, Index: pageIndex},
		},
	})
	if err != nil {
		return "", fmt.Errorf("extract text from page %d of %s: %w", page, filepath.Base(path), err)
	}
	return resp.Text, nil
}

// Close releases the underlying PDFium resources.
func (r *PDFiumTextRenderer) Close() error {
	if err := r.instance.Close(); err != nil {
		return err
	}
	return r.pool.Close()
}
