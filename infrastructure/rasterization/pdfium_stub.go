//go:build !pdfium

package rasterization

import "image"

// NewPdfiumRasterizer returns nil when the pdfium build tag is absent.
// The registry will have no PDF rasterizer, so PDF files are skipped.
func NewPdfiumRasterizer() (*PdfiumRasterizer, error) {
	return nil, nil
}

// PdfiumRasterizer is a placeholder when pdfium is not available.
type PdfiumRasterizer struct{}

func (p *PdfiumRasterizer) PageCount(_ string) (int, error)             { return 0, nil }
func (p *PdfiumRasterizer) Render(_ string, _ int) (image.Image, error) { return nil, nil }
func (p *PdfiumRasterizer) Close() error                                { return nil }
