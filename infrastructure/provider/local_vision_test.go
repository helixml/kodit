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

	vision := emb.VisionEmbedder()

	imgData := testPNG(t, 64, 64, color.RGBA{R: 255, A: 255})
	req := NewEmbeddingRequest([][]byte{imgData})

	resp, err := vision.Embed(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Len(t, embeddings, 1, "expected one embedding for one image")
	require.Equal(t, 768, len(embeddings[0]), "siglip2-base produces 768 dimensions")
}

func TestLocalVisionEmbedding_EmbedQuery(t *testing.T) {
	modelDir := localVisionModelPath(t)
	emb := NewLocalVisionEmbedding(SigLIP2BaseConfig, modelDir)

	text := emb.TextEmbedder()

	req := NewTextEmbeddingRequest([]string{"a photo of a cat"})
	resp, err := text.Embed(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Len(t, embeddings, 1, "expected one embedding for one query")
	require.Equal(t, 768, len(embeddings[0]), "siglip2-base produces 768 dimensions")
}

func TestLocalVisionEmbedding_EmbedEmpty(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewLocalVisionEmbedding(SigLIP2BaseConfig, modelDir)

	vision := emb.VisionEmbedder()
	text := emb.TextEmbedder()

	resp, err := vision.Embed(context.Background(), NewEmbeddingRequest(nil))
	require.NoError(t, err)
	require.Empty(t, resp.Embeddings())

	textResp, err := text.Embed(context.Background(), NewEmbeddingRequest(nil))
	require.NoError(t, err)
	require.Empty(t, textResp.Embeddings())
}

func TestLocalVisionEmbedding_CancelledContext(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewLocalVisionEmbedding(SigLIP2BaseConfig, modelDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	vision := emb.VisionEmbedder()
	imgData := testPNG(t, 8, 8, color.White)
	_, err := vision.Embed(ctx, NewEmbeddingRequest([][]byte{imgData}))
	require.Error(t, err)
}
