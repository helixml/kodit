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

// siglip2ModelPath returns the models directory if the SigLIP2 model has been
// downloaded, or skips the test otherwise (run `make download-siglip2`).
func siglip2ModelPath(t *testing.T) string {
	t.Helper()
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

	vision := emb.VisionEmbedder()

	imgData := testPNG(t, 64, 64, color.RGBA{R: 255, A: 255})
	req := NewEmbeddingRequest([][]byte{imgData})

	resp, err := vision.Embed(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Len(t, embeddings, 1, "expected one embedding for one image")
	require.Equal(t, 768, len(embeddings[0]), "siglip2-base produces 768 dimensions")
}

func TestSigLIP2Embedding_EmbedQuery(t *testing.T) {
	modelDir := siglip2ModelPath(t)
	emb := NewSigLIP2Embedding(modelDir)
	require.True(t, emb.Available())

	text := emb.TextEmbedder()

	req := NewTextEmbeddingRequest([]string{"a photo of a cat"})
	resp, err := text.Embed(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Len(t, embeddings, 1, "expected one embedding for one query")
	require.Equal(t, 768, len(embeddings[0]), "siglip2-base produces 768 dimensions")
}

func TestSigLIP2Embedding_EmbedEmpty(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewSigLIP2Embedding(modelDir)

	vision := emb.VisionEmbedder()
	text := emb.TextEmbedder()

	// Empty image request should return empty without initialization.
	resp, err := vision.Embed(context.Background(), NewEmbeddingRequest(nil))
	require.NoError(t, err)
	require.Empty(t, resp.Embeddings())

	// Empty text request too.
	textResp, err := text.Embed(context.Background(), NewEmbeddingRequest(nil))
	require.NoError(t, err)
	require.Empty(t, textResp.Embeddings())
}

func TestSigLIP2Embedding_CancelledContext(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewSigLIP2Embedding(modelDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	vision := emb.VisionEmbedder()
	imgData := testPNG(t, 8, 8, color.White)
	_, err := vision.Embed(ctx, NewEmbeddingRequest([][]byte{imgData}))
	require.Error(t, err)
}
