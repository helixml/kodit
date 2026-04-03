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

// siglip2ModelDir returns the path to the on-disk SigLIP2 model, or skips the
// test if the model has not been downloaded (run `make download-siglip2`).
func siglip2ModelPath(t *testing.T) string {
	t.Helper()
	// The model is downloaded to infrastructure/provider/models/ relative to
	// the repo root. In tests the working directory is the package directory,
	// so we go up two levels.
	candidate := filepath.Join("models", siglip2ModelDir, "onnx", siglip2VisionOnnx)
	if _, err := os.Stat(candidate); err != nil {
		t.Skipf("skipping: siglip2 model not on disk (run make download-siglip2): %v", err)
	}
	return "models"
}

func TestSigLIP2Embedding_EmbedImage(t *testing.T) {
	modelDir := siglip2ModelPath(t)
	emb := NewSigLIP2Embedding(modelDir)
	require.True(t, emb.Available())

	// Embed a simple red 64x64 PNG.
	imgData := testPNG(t, 64, 64, color.RGBA{R: 255, A: 255})
	req := NewVisionEmbeddingRequest([]VisionImage{NewVisionImage(imgData)})

	resp, err := emb.EmbedImages(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Len(t, embeddings, 1, "expected one embedding for one image")
	require.Equal(t, 768, len(embeddings[0]), "siglip2-base produces 768 dimensions")
}

func TestSigLIP2Embedding_EmbedQuery(t *testing.T) {
	modelDir := siglip2ModelPath(t)
	emb := NewSigLIP2Embedding(modelDir)
	require.True(t, emb.Available())

	req := NewEmbeddingRequest([]string{"a photo of a cat"})
	resp, err := emb.EmbedQuery(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Len(t, embeddings, 1, "expected one embedding for one query")
	require.Equal(t, 768, len(embeddings[0]), "siglip2-base produces 768 dimensions")
}

func TestSigLIP2Embedding_EmbedEmpty(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewSigLIP2Embedding(modelDir)

	// Empty image request should return empty without initialization.
	req := NewVisionEmbeddingRequest(nil)
	resp, err := emb.EmbedImages(context.Background(), req)
	require.NoError(t, err)
	require.Empty(t, resp.Embeddings())

	// Empty text request too.
	textReq := NewEmbeddingRequest(nil)
	textResp, err := emb.EmbedQuery(context.Background(), textReq)
	require.NoError(t, err)
	require.Empty(t, textResp.Embeddings())
}

func TestSigLIP2Embedding_CancelledContext(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewSigLIP2Embedding(modelDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	imgData := testPNG(t, 8, 8, color.White)
	req := NewVisionEmbeddingRequest([]VisionImage{NewVisionImage(imgData)})
	_, err := emb.EmbedImages(ctx, req)
	require.Error(t, err)
}
