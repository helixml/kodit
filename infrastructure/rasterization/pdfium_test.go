package rasterization

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPdfiumRasterizer_Available(t *testing.T) {
	rast, err := NewPdfiumRasterizer()
	require.NoError(t, err)
	require.NotNil(t, rast, "pdfium rasterizer must be available")
	defer func() { _ = rast.Close() }()
}
