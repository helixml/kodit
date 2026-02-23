package provider

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

const hugotBatchMax = 10

// ortSingleton holds the process-wide ONNX Runtime session and pipeline.
// ORT only allows one active session per process, so all HugotEmbedding
// instances must share it. The mutex serializes both initialization and
// inference (ORT is not thread-safe).
var ortSingleton struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	mu       sync.Mutex
	ready    bool
}

// HugotEmbedding provides local embedding generation using the st-codesearch-distilroberta-base
// model via the hugot Go backend.
//
// The model can come from two sources (checked in order):
//  1. Model files on disk — a subdirectory of cacheDir containing tokenizer.json.
//  2. Statically embedded in the binary (build tag embed_model), extracted to
//     cacheDir on first use.
//
// All instances share a single ONNX Runtime session because ORT only supports
// one active session per process.
type HugotEmbedding struct {
	cacheDir string
}

// NewHugotEmbedding creates a HugotEmbedding that looks for model files in cacheDir.
// If no model exists on disk and the embed_model build tag was used, the
// embedded model is extracted to cacheDir automatically.
func NewHugotEmbedding(cacheDir string) *HugotEmbedding {
	return &HugotEmbedding{
		cacheDir: cacheDir,
	}
}

// Available reports whether a usable model exists — either compiled into
// the binary (embed_model build tag) or present on disk in cacheDir.
func (h *HugotEmbedding) Available() bool {
	if hasEmbeddedModel {
		return true
	}
	_, err := h.diskModelPath()
	return err == nil
}

func (h *HugotEmbedding) initialize() error {
	ortSingleton.mu.Lock()
	defer ortSingleton.mu.Unlock()

	if ortSingleton.ready {
		return nil
	}

	session, err := newHugotSession()
	if err != nil {
		return fmt.Errorf("create hugot session: %w", err)
	}

	modelPath, err := h.resolveModelPath()
	if err != nil {
		_ = session.Destroy()
		return err
	}

	config := hugot.FeatureExtractionConfig{
		ModelPath: modelPath,
		Name:      "builtin-embeddings",
		Options: []hugot.FeatureExtractionOption{
			pipelines.WithNormalization(),
		},
	}
	pipeline, err := hugot.NewPipeline(session, config)
	if err != nil {
		_ = session.Destroy()
		return fmt.Errorf("create feature extraction pipeline: %w", err)
	}

	ortSingleton.session = session
	ortSingleton.pipeline = pipeline
	ortSingleton.ready = true
	return nil
}

// resolveModelPath returns the path to a usable model directory.
// It first checks for model files already on disk in cacheDir, then
// falls back to extracting the statically embedded model (if compiled in).
func (h *HugotEmbedding) resolveModelPath() (string, error) {
	// Prefer model files already present on disk.
	if diskPath, err := h.diskModelPath(); err == nil {
		return diskPath, nil
	}

	if !hasEmbeddedModel {
		return "", fmt.Errorf("no model found in %s and no embedded model compiled in (build with -tags embed_model)", h.cacheDir)
	}

	if err := os.MkdirAll(h.cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create cache directory: %w", err)
	}

	return extractEmbeddedModel(embeddedModelFS, h.cacheDir)
}

// diskModelPath looks for a model subdirectory containing tokenizer.json
// inside cacheDir. Returns the path if found, or an error if no valid
// model directory exists on disk.
func (h *HugotEmbedding) diskModelPath() (string, error) {
	entries, err := os.ReadDir(h.cacheDir)
	if err != nil {
		return "", fmt.Errorf("read model directory %s: %w", h.cacheDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(h.cacheDir, entry.Name())
		if _, statErr := os.Stat(filepath.Join(candidate, "tokenizer.json")); statErr == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no model subdirectory with tokenizer.json found in %s", h.cacheDir)
}

// extractEmbeddedModel writes the statically embedded model files to targetDir
// and returns the path to the model subdirectory.
func extractEmbeddedModel(embedded fs.FS, targetDir string) (string, error) {
	modelsFS, err := fs.Sub(embedded, "models")
	if err != nil {
		return "", fmt.Errorf("access embedded models: %w", err)
	}

	entries, err := fs.ReadDir(modelsFS, ".")
	if err != nil {
		return "", fmt.Errorf("read embedded models: %w", err)
	}

	var modelSubdir string
	for _, entry := range entries {
		if entry.IsDir() {
			modelSubdir = entry.Name()
			break
		}
	}
	if modelSubdir == "" {
		return "", fmt.Errorf("no model directory found in embedded models")
	}

	modelPath := filepath.Join(targetDir, modelSubdir)

	// Skip extraction if already present
	if _, statErr := os.Stat(filepath.Join(modelPath, "tokenizer.json")); statErr == nil {
		return modelPath, nil
	}

	modelFS, err := fs.Sub(modelsFS, modelSubdir)
	if err != nil {
		return "", fmt.Errorf("access model subdirectory: %w", err)
	}

	err = fs.WalkDir(modelFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(modelPath, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, readErr := fs.ReadFile(modelFS, path)
		if readErr != nil {
			return fmt.Errorf("read embedded file %s: %w", path, readErr)
		}
		if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o755); mkdirErr != nil {
			return fmt.Errorf("create directory for %s: %w", path, mkdirErr)
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return "", fmt.Errorf("extract embedded model: %w", err)
	}

	return modelPath, nil
}

// Capacity returns the maximum number of texts per Embed call.
func (h *HugotEmbedding) Capacity() int { return hugotBatchMax }

// Embed generates embeddings for the given texts using the local model.
// The number of texts must not exceed Capacity().
func (h *HugotEmbedding) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	texts := req.Texts()
	if len(texts) == 0 {
		return NewEmbeddingResponse([][]float64{}, NewUsage(0, 0, 0)), nil
	}

	if len(texts) > hugotBatchMax {
		return EmbeddingResponse{}, fmt.Errorf("embed: %d texts exceeds capacity %d", len(texts), hugotBatchMax)
	}

	if err := ctx.Err(); err != nil {
		return EmbeddingResponse{}, err
	}

	if err := h.initialize(); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("initialize hugot: %w", err)
	}

	// Hold the singleton mutex for inference — ORT is not thread-safe.
	ortSingleton.mu.Lock()
	defer ortSingleton.mu.Unlock()

	result, err := ortSingleton.pipeline.RunPipeline(texts)
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("run embedding pipeline: %w", err)
	}

	embeddings := make([][]float64, len(result.Embeddings))
	for i, vec32 := range result.Embeddings {
		vec64 := make([]float64, len(vec32))
		for j, v := range vec32 {
			vec64[j] = float64(v)
		}
		embeddings[i] = vec64
	}

	return NewEmbeddingResponse(embeddings, NewUsage(0, 0, 0)), nil
}

// Close is a no-op. The ONNX Runtime session is process-global and shared
// across all HugotEmbedding instances; it is cleaned up when the process exits.
func (h *HugotEmbedding) Close() error {
	return nil
}

var _ Embedder = (*HugotEmbedding)(nil)
