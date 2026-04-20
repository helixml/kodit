package rasterization

import (
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// renderDPI is the resolution used when rendering document pages to images.
const renderDPI = 150

// PdfiumRasterizer renders PDF pages using the PDFium library via WebAssembly
// (Wazero). No CGO or system libraries are required — the PDFium WASM binary
// is embedded in the go-pdfium module.
//
// The underlying pdfium.Pdfium instance wraps a single WASM module whose
// internal state is not safe for concurrent use: parallel calls corrupt the
// module's memory and function tables, after which every subsequent call
// fails until the process restarts. mu serialises all calls into the instance.
type PdfiumRasterizer struct {
	pool     pdfium.Pool
	instance pdfium.Pdfium
	mu       sync.Mutex
}

// NewPdfiumRasterizer initialises PDFium via the Wazero WebAssembly runtime
// and returns a Rasterizer for PDF files.
func NewPdfiumRasterizer() (*PdfiumRasterizer, error) {
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
	return &PdfiumRasterizer{pool: pool, instance: inst}, nil
}

// PageCount returns the number of pages in the PDF at the given path.
func (r *PdfiumRasterizer) PageCount(path string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, err := r.instance.OpenDocument(&requests.OpenDocument{
		FilePath: &path,
	})
	if err != nil {
		return 0, fmt.Errorf("open pdf %s: %w", path, err)
	}
	defer func() {
		_, _ = r.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
			Document: doc.Document,
		})
	}()

	resp, err := r.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return 0, fmt.Errorf("get page count: %w", err)
	}
	return resp.PageCount, nil
}

// Render returns the given 1-based page of the PDF as an image.
func (r *PdfiumRasterizer) Render(path string, page int) (image.Image, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, err := r.instance.OpenDocument(&requests.OpenDocument{
		FilePath: &path,
	})
	if err != nil {
		return nil, fmt.Errorf("open pdf %s: %w", path, err)
	}
	defer func() {
		_, _ = r.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
			Document: doc.Document,
		})
	}()

	pageIndex := page - 1 // convert 1-based to 0-based
	resp, err := r.instance.RenderPageInDPI(&requests.RenderPageInDPI{
		DPI: renderDPI,
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{
				Document: doc.Document,
				Index:    pageIndex,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("render page %d: %w", page, err)
	}
	defer resp.Cleanup()

	src := resp.Result.Image
	pix := make([]uint8, len(src.Pix))
	copy(pix, src.Pix)
	return &image.RGBA{
		Pix:    pix,
		Stride: src.Stride,
		Rect:   src.Rect,
	}, nil
}

// Close releases all PDFium resources.
func (r *PdfiumRasterizer) Close() error {
	if err := r.instance.Close(); err != nil {
		return err
	}
	return r.pool.Close()
}
