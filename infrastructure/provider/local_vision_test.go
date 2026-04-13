package provider

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/search"
)

// testPNG generates a simple solid-color PNG image.
func testPNG(t *testing.T, width, height int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// localVisionModelPath returns the models directory if the SigLIP2 model has
// been downloaded, or skips the test (run `make download-siglip2`).
func localVisionModelPath(t *testing.T) string {
	t.Helper()
	cfg := SigLIP2BaseConfig
	candidate := filepath.Join("models", cfg.ModelDir, "onnx", cfg.VisionOnnx)
	if _, err := os.Stat(candidate); err != nil {
		t.Skipf("skipping: vision model not on disk (run make download-siglip2): %v", err)
	}
	return "models"
}

func TestLocalVisionEmbedding_EmbedImage(t *testing.T) {
	modelDir := localVisionModelPath(t)
	emb := NewLocalVisionEmbedding(SigLIP2BaseConfig, modelDir)

	imgData := testPNG(t, 64, 64, color.RGBA{R: 255, A: 255})

	embeddings, err := emb.Embed(context.Background(), search.NewImageItems([][]byte{imgData}))
	require.NoError(t, err)

	require.Len(t, embeddings, 1, "expected one embedding for one image")
	require.Equal(t, 768, len(embeddings[0]), "siglip2-base produces 768 dimensions")
}

func TestLocalVisionEmbedding_EmbedQuery(t *testing.T) {
	modelDir := localVisionModelPath(t)
	emb := NewLocalVisionEmbedding(SigLIP2BaseConfig, modelDir)

	embeddings, err := emb.Embed(context.Background(), search.NewTextItems([]string{"a photo of a cat"}))
	require.NoError(t, err)

	require.Len(t, embeddings, 1, "expected one embedding for one query")
	require.Equal(t, 768, len(embeddings[0]), "siglip2-base produces 768 dimensions")
}

func TestLocalVisionEmbedding_EmbedEmpty(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewLocalVisionEmbedding(SigLIP2BaseConfig, modelDir)

	embeddings, err := emb.Embed(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, embeddings)
}

func TestLocalVisionEmbedding_CancelledContext(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewLocalVisionEmbedding(SigLIP2BaseConfig, modelDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	imgData := testPNG(t, 8, 8, color.White)
	_, err := emb.Embed(ctx, []search.EmbeddingItem{search.NewImageItem(imgData)})
	require.Error(t, err)
}
