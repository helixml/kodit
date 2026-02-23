package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestHugotEmbedding_Embed(t *testing.T) {
	if !hasEmbeddedModel {
		t.Skip("skipping: requires -tags embed_model")
	}

	modelDir := t.TempDir()
	emb := NewHugotEmbedding(modelDir)
	defer func() {
		require.NoError(t, emb.Close())
	}()

	req := NewEmbeddingRequest([]string{"hello world"})
	resp, err := emb.Embed(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Len(t, embeddings, 1)
	require.Equal(t, 768, len(embeddings[0]), "st-codesearch-distilroberta-base produces 768 dimensions")
}

func TestHugotEmbedding_EmbedBatch(t *testing.T) {
	if !hasEmbeddedModel {
		t.Skip("skipping: requires -tags embed_model")
	}

	modelDir := t.TempDir()
	emb := NewHugotEmbedding(modelDir)
	defer func() {
		require.NoError(t, emb.Close())
	}()

	// 50 texts should be split into 5 batches of 10
	texts := make([]string, 50)
	for i := range texts {
		texts[i] = "test sentence number"
	}

	req := NewEmbeddingRequest(texts)
	resp, err := emb.Embed(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Len(t, embeddings, 50)
	for i, vec := range embeddings {
		require.Equal(t, 768, len(vec), "embedding %d has wrong dimension", i)
	}
}

func TestHugotEmbedding_EmbedEmpty(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewHugotEmbedding(modelDir)
	defer func() {
		require.NoError(t, emb.Close())
	}()

	req := NewEmbeddingRequest([]string{})
	resp, err := emb.Embed(context.Background(), req)
	require.NoError(t, err)

	embeddings := resp.Embeddings()
	require.Empty(t, embeddings)
}

func TestHugotEmbedding_Close(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewHugotEmbedding(modelDir)

	// Close without initialization should succeed
	require.NoError(t, emb.Close())

	// Double close should also succeed
	require.NoError(t, emb.Close())
}

func TestExtractEmbeddedModel(t *testing.T) {
	// Build a fake embedded FS with the expected structure
	fakeFS := fstest.MapFS{
		"models/test-model/tokenizer.json":  {Data: []byte(`{"test": true}`)},
		"models/test-model/config.json":     {Data: []byte(`{"hidden_size": 768}`)},
		"models/test-model/onnx/model.onnx": {Data: []byte("fake-onnx-data")},
	}

	targetDir := t.TempDir()
	modelPath, err := extractEmbeddedModel(fakeFS, targetDir)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(targetDir, "test-model"), modelPath)

	// Verify files were extracted
	data, err := os.ReadFile(filepath.Join(modelPath, "tokenizer.json"))
	require.NoError(t, err)
	require.Contains(t, string(data), `"test": true`)

	data, err = os.ReadFile(filepath.Join(modelPath, "onnx", "model.onnx"))
	require.NoError(t, err)
	require.Equal(t, "fake-onnx-data", string(data))

	// Second extraction should skip (files already present)
	modelPath2, err := extractEmbeddedModel(fakeFS, targetDir)
	require.NoError(t, err)
	require.Equal(t, modelPath, modelPath2)
}

func TestExtractEmbeddedModel_NoModelDir(t *testing.T) {
	emptyFS := fstest.MapFS{
		"models/.gitkeep": {Data: []byte("")},
	}

	targetDir := t.TempDir()
	_, err := extractEmbeddedModel(emptyFS, targetDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no model directory found")
}

func TestHugotEmbedding_DiskModelPath(t *testing.T) {
	modelDir := t.TempDir()

	// No model yet — diskModelPath should fail.
	emb := NewHugotEmbedding(modelDir)
	_, err := emb.diskModelPath()
	require.Error(t, err)

	// Create a valid model subdirectory.
	subdir := filepath.Join(modelDir, "my-model")
	require.NoError(t, os.MkdirAll(subdir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subdir, "tokenizer.json"), []byte(`{}`), 0o644))

	got, err := emb.diskModelPath()
	require.NoError(t, err)
	require.Equal(t, subdir, got)
}

func TestHugotEmbedding_AvailableWithDiskModel(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewHugotEmbedding(modelDir)

	// Without embedded model and no disk model, should be unavailable.
	if !hasEmbeddedModel {
		require.False(t, emb.Available())
	}

	// Place model files on disk — should become available.
	subdir := filepath.Join(modelDir, "test-model")
	require.NoError(t, os.MkdirAll(subdir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subdir, "tokenizer.json"), []byte(`{}`), 0o644))

	require.True(t, emb.Available())
}

func TestHugotEmbedding_DiskModelPath_SkipsFiles(t *testing.T) {
	modelDir := t.TempDir()

	// A plain file (not a directory) should be skipped.
	require.NoError(t, os.WriteFile(filepath.Join(modelDir, "README.md"), []byte("readme"), 0o644))

	emb := NewHugotEmbedding(modelDir)
	_, err := emb.diskModelPath()
	require.Error(t, err)
}

func TestHugotEmbedding_DiskModelPath_SkipsDirWithoutTokenizer(t *testing.T) {
	modelDir := t.TempDir()

	// A directory without tokenizer.json should be skipped.
	require.NoError(t, os.MkdirAll(filepath.Join(modelDir, "incomplete-model"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(modelDir, "incomplete-model", "config.json"), []byte(`{}`), 0o644))

	emb := NewHugotEmbedding(modelDir)
	_, err := emb.diskModelPath()
	require.Error(t, err)
}

func TestHugotEmbedding_CancelledContext(t *testing.T) {
	modelDir := t.TempDir()
	emb := NewHugotEmbedding(modelDir)
	defer func() {
		require.NoError(t, emb.Close())
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := NewEmbeddingRequest([]string{"hello"})
	_, err := emb.Embed(ctx, req)
	require.Error(t, err)
}
