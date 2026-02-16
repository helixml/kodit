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

const hugotBatchMax = 32

// HugotEmbedding provides local embedding generation using the st-codesearch-distilroberta-base
// model via the hugot Go backend.
//
// When built with -tags embed_model, the model is statically compiled into the
// binary and extracted to modelDir on first use. Otherwise, the model must already
// exist on disk (via 'kodit download-model' or WithModelDir).
type HugotEmbedding struct {
	modelDir string
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	mu       sync.Mutex
	ready    bool
}

// NewHugotEmbedding creates a HugotEmbedding that stores model files in modelDir.
func NewHugotEmbedding(modelDir string) *HugotEmbedding {
	return &HugotEmbedding{
		modelDir: modelDir,
	}
}

// Available reports whether a model is present (on disk or embedded).
func (h *HugotEmbedding) Available() bool {
	if hasEmbeddedModel {
		return true
	}
	_, err := findModelOnDisk(h.modelDir)
	return err == nil
}

func (h *HugotEmbedding) initialize() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.ready {
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

	h.session = session
	h.pipeline = pipeline
	h.ready = true
	return nil
}

// resolveModelPath finds or extracts the model to disk.
// Priority: existing files on disk > embedded model > error.
func (h *HugotEmbedding) resolveModelPath() (string, error) {
	if err := os.MkdirAll(h.modelDir, 0o755); err != nil {
		return "", fmt.Errorf("create model directory: %w", err)
	}

	// Check if model already exists on disk
	if path, err := findModelOnDisk(h.modelDir); err == nil {
		return path, nil
	}

	// Extract from embedded model if available
	if hasEmbeddedModel {
		return extractEmbeddedModel(embeddedModelFS, h.modelDir)
	}

	return "", fmt.Errorf("no model found at %q: run 'kodit download-model --dest %s' or build with -tags embed_model", h.modelDir, h.modelDir)
}

// findModelOnDisk looks for a model subdirectory containing tokenizer.json.
func findModelOnDisk(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(dir, entry.Name())
		if _, statErr := os.Stat(filepath.Join(candidate, "tokenizer.json")); statErr == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no model directory found in %s", dir)
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

// Embed generates embeddings for the given texts using the local model.
func (h *HugotEmbedding) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	texts := req.Texts()
	if len(texts) == 0 {
		return NewEmbeddingResponse([][]float64{}, NewUsage(0, 0, 0)), nil
	}

	if err := ctx.Err(); err != nil {
		return EmbeddingResponse{}, err
	}

	if err := h.initialize(); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("initialize hugot: %w", err)
	}

	embeddings := make([][]float64, 0, len(texts))

	for i := 0; i < len(texts); i += hugotBatchMax {
		if err := ctx.Err(); err != nil {
			return EmbeddingResponse{}, err
		}

		end := min(i+hugotBatchMax, len(texts))
		batch := texts[i:end]

		result, err := h.pipeline.RunPipeline(batch)
		if err != nil {
			return EmbeddingResponse{}, fmt.Errorf("run embedding pipeline: %w", err)
		}

		for _, vec32 := range result.Embeddings {
			vec64 := make([]float64, len(vec32))
			for j, v := range vec32 {
				vec64[j] = float64(v)
			}
			embeddings = append(embeddings, vec64)
		}
	}

	return NewEmbeddingResponse(embeddings, NewUsage(0, 0, 0)), nil
}

// Close releases the hugot session resources.
func (h *HugotEmbedding) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session == nil {
		return nil
	}

	err := h.session.Destroy()
	h.session = nil
	h.pipeline = nil
	h.ready = false
	return err
}

var _ Embedder = (*HugotEmbedding)(nil)
