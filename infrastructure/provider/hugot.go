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

	"github.com/helixml/kodit/domain/search"
)

// ortSingleton holds the process-wide ONNX Runtime session.
// ORT only allows one active session per process, so all providers
// (HugotEmbedding, SigLIP2Embedding, etc.) must share it. The mutex
// serializes session creation, pipeline registration, and inference
// (ORT is not thread-safe).
var ortSingleton struct {
	session *hugot.Session
	mu      sync.Mutex
	ready   bool
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
	pipeline *pipelines.FeatureExtractionPipeline
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

const builtinEmbeddingsPipeline = "builtin-embeddings"

func (h *HugotEmbedding) initialize() error {
	if h.pipeline != nil {
		return nil
	}

	session, err := ensureORTSession()
	if err != nil {
		return err
	}

	// Reuse existing pipeline if another HugotEmbedding already created it.
	if existing, getErr := hugot.GetPipeline[*pipelines.FeatureExtractionPipeline](session, builtinEmbeddingsPipeline); getErr == nil {
		h.pipeline = existing
		return nil
	}

	modelPath, err := h.resolveModelPath()
	if err != nil {
		return err
	}

	config := hugot.FeatureExtractionConfig{
		ModelPath: modelPath,
		Name:      builtinEmbeddingsPipeline,
		Options: []hugot.FeatureExtractionOption{
			pipelines.WithNormalization(),
		},
	}
	pipeline, err := hugot.NewPipeline(session, config)
	if err != nil {
		return fmt.Errorf("create feature extraction pipeline: %w", err)
	}

	h.pipeline = pipeline
	return nil
}

// ensureORTSession creates the process-wide ORT session if it does not exist,
// or returns the existing one. Callers must hold ortSingleton.mu for inference.
func ensureORTSession() (*hugot.Session, error) {
	ortSingleton.mu.Lock()
	defer ortSingleton.mu.Unlock()

	if ortSingleton.ready {
		return ortSingleton.session, nil
	}

	session, err := newHugotSession()
	if err != nil {
		return nil, fmt.Errorf("create hugot session: %w", err)
	}

	ortSingleton.session = session
	ortSingleton.ready = true
	return session, nil
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

// extractEmbeddedModel writes the first embedded model subdirectory to targetDir
// and returns the path to the extracted model.
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

	return extractEmbeddedModelByName(embedded, targetDir, modelSubdir)
}

// extractEmbeddedModelByName writes the named model subdirectory from the
// embedded FS to targetDir and returns the path to the extracted model.
func extractEmbeddedModelByName(embedded fs.FS, targetDir, name string) (string, error) {
	modelFS, err := fs.Sub(embedded, filepath.Join("models", name))
	if err != nil {
		return "", fmt.Errorf("access embedded model %s: %w", name, err)
	}

	modelPath := filepath.Join(targetDir, name)

	// Skip extraction if already present.
	if _, statErr := os.Stat(filepath.Join(modelPath, "tokenizer.json")); statErr == nil {
		return modelPath, nil
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
		return "", fmt.Errorf("extract embedded model %s: %w", name, err)
	}

	return modelPath, nil
}

// Embed generates embeddings for the given text items using the local model.
// Items without a text payload return an error — hugot is a text-only model.
func (h *HugotEmbedding) Embed(ctx context.Context, items []search.EmbeddingItem) ([][]float64, error) {
	if len(items) == 0 {
		return [][]float64{}, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := h.initialize(); err != nil {
		return nil, fmt.Errorf("initialize hugot: %w", err)
	}

	texts := make([]string, len(items))
	for i, item := range items {
		if !item.HasText() {
			return nil, fmt.Errorf("hugot embedding requires text, got item %d with no text", i)
		}
		texts[i] = string(item.Text())
	}

	// Hold the singleton mutex for inference — ORT is not thread-safe.
	ortSingleton.mu.Lock()
	defer ortSingleton.mu.Unlock()

	result, err := h.pipeline.RunPipeline(texts)
	if err != nil {
		return nil, fmt.Errorf("run embedding pipeline: %w", err)
	}

	return float32MatrixToFloat64(result.Embeddings), nil
}

// Close is a no-op. The ONNX Runtime session is process-global and shared
// across all HugotEmbedding instances; it is cleaned up when the process exits.
func (h *HugotEmbedding) Close() error {
	return nil
}

var _ search.Embedder = (*HugotEmbedding)(nil)
