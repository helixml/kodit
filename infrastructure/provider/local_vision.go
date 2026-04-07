package provider

import (
	"bytes"
	"context"
	"fmt"
	"image"
	// Register standard image decoders.
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
	"github.com/knights-analytics/hugot/util/imageutil"
)

// LocalVisionEmbedding manages a local ONNX dual-encoder vision-language
// model and exposes two Embedder implementations that produce vectors in the
// same embedding space: one for images, one for text queries.
//
// The specific model (SigLIP2, CLIP, etc.) is determined by the
// VisionModelConfig passed at construction time. All instances share the
// process-wide ORT session.
type LocalVisionEmbedding struct {
	config         VisionModelConfig
	cacheDir       string
	visionPipeline *pipelines.FeatureExtractionPipeline
	textPipeline   *pipelines.FeatureExtractionPipeline
}

// NewLocalVisionEmbedding creates a LocalVisionEmbedding for the model
// described by config, looking for files in cacheDir.
func NewLocalVisionEmbedding(config VisionModelConfig, cacheDir string) *LocalVisionEmbedding {
	return &LocalVisionEmbedding{config: config, cacheDir: cacheDir}
}

// VisionEmbedder returns an Embedder that accepts image bytes (PNG, JPEG)
// and produces vectors in the model's shared embedding space.
func (l *LocalVisionEmbedding) VisionEmbedder() Embedder {
	return &localVisionEmbedder{parent: l}
}

// TextEmbedder returns an Embedder that accepts UTF-8 text and produces
// vectors in the same embedding space as VisionEmbedder. Use this to embed
// text queries for cross-modal image search.
func (l *LocalVisionEmbedding) TextEmbedder() Embedder {
	return &localTextEmbedder{parent: l}
}

// Close is a no-op. The ORT session is process-global.
func (l *LocalVisionEmbedding) Close() error {
	return nil
}

func (l *LocalVisionEmbedding) hasModelFiles(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "onnx", l.config.VisionOnnx)); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "onnx", l.config.TextOnnx)); err != nil {
		return false
	}
	return true
}

func (l *LocalVisionEmbedding) visionPipelineName() string {
	return l.config.ModelDir + "-vision"
}

func (l *LocalVisionEmbedding) textPipelineName() string {
	return l.config.ModelDir + "-text"
}

func (l *LocalVisionEmbedding) initialize() error {
	if l.visionPipeline != nil && l.textPipeline != nil {
		return nil
	}

	session, err := ensureORTSession()
	if err != nil {
		return err
	}

	// Reuse existing pipelines if already registered on the session.
	if v, vErr := hugot.GetPipeline[*pipelines.FeatureExtractionPipeline](session, l.visionPipelineName()); vErr == nil {
		l.visionPipeline = v
	}
	if t, tErr := hugot.GetPipeline[*pipelines.FeatureExtractionPipeline](session, l.textPipelineName()); tErr == nil {
		l.textPipeline = t
	}
	if l.visionPipeline != nil && l.textPipeline != nil {
		return nil
	}

	modelPath, err := l.resolveModelPath()
	if err != nil {
		return err
	}

	if l.visionPipeline == nil {
		norm := imageutil.PixelNormalizationStep(l.config.ImageMean, l.config.ImageStd)
		visionOpts := []hugot.FeatureExtractionOption{
			pipelines.WithNormalization(),
			pipelines.WithImageMode(),
			pipelines.WithPreprocessSteps[*pipelines.FeatureExtractionPipeline](
				imageutil.ResizeStep(l.config.ImageSize),
				imageutil.CenterCropStep(l.config.ImageSize, l.config.ImageSize),
			),
			pipelines.WithNormalizationSteps[*pipelines.FeatureExtractionPipeline](
				imageutil.RescaleStep(),
				norm,
			),
		}
		if l.config.VisionOutputName != "" {
			visionOpts = append(visionOpts, pipelines.WithOutputName(l.config.VisionOutputName))
		}
		visionConfig := hugot.FeatureExtractionConfig{
			ModelPath:    modelPath,
			Name:         l.visionPipelineName(),
			OnnxFilename: l.config.VisionOnnx,
			Options:      visionOpts,
		}
		visionPipeline, err := hugot.NewPipeline(session, visionConfig)
		if err != nil {
			return fmt.Errorf("create vision pipeline %s: %w", l.config.ModelDir, err)
		}
		l.visionPipeline = visionPipeline
	}

	if l.textPipeline == nil {
		textConfig := hugot.FeatureExtractionConfig{
			ModelPath:    modelPath,
			Name:         l.textPipelineName(),
			OnnxFilename: l.config.TextOnnx,
			Options: []hugot.FeatureExtractionOption{
				pipelines.WithNormalization(),
			},
		}
		textPipeline, err := hugot.NewPipeline(session, textConfig)
		if err != nil {
			return fmt.Errorf("create text pipeline %s: %w", l.config.ModelDir, err)
		}
		l.textPipeline = textPipeline
	}

	return nil
}

func (l *LocalVisionEmbedding) resolveModelPath() (string, error) {
	diskPath := filepath.Join(l.cacheDir, l.config.ModelDir)
	if l.hasModelFiles(diskPath) {
		return diskPath, nil
	}

	if !hasEmbeddedModel {
		return "", fmt.Errorf("vision model %s not found in %s and no embedded model compiled in", l.config.ModelDir, l.cacheDir)
	}

	if err := os.MkdirAll(l.cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create cache directory: %w", err)
	}

	return extractEmbeddedModelByName(embeddedModelFS, l.cacheDir, l.config.ModelDir)
}

// localVisionEmbedder implements Embedder for image inputs.
type localVisionEmbedder struct {
	parent *LocalVisionEmbedding
}

func (v *localVisionEmbedder) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	inputs := req.Inputs()
	if len(inputs) == 0 {
		return NewEmbeddingResponse([][]float64{}, NewUsage(0, 0, 0)), nil
	}

	if err := ctx.Err(); err != nil {
		return EmbeddingResponse{}, err
	}

	if err := v.parent.initialize(); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("initialize vision model: %w", err)
	}

	goImages := make([]image.Image, len(inputs))
	for i, data := range inputs {
		decoded, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return EmbeddingResponse{}, fmt.Errorf("decode image %d: %w", i, err)
		}
		goImages[i] = decoded
	}

	ortSingleton.mu.Lock()
	defer ortSingleton.mu.Unlock()

	result, err := v.parent.visionPipeline.RunWithImages(goImages)
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("run vision pipeline: %w", err)
	}

	return toEmbeddingResponse(result.Embeddings), nil
}

// localTextEmbedder implements Embedder for text query inputs.
type localTextEmbedder struct {
	parent *LocalVisionEmbedding
}

func (t *localTextEmbedder) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	inputs := req.Inputs()
	if len(inputs) == 0 {
		return NewEmbeddingResponse([][]float64{}, NewUsage(0, 0, 0)), nil
	}

	if err := ctx.Err(); err != nil {
		return EmbeddingResponse{}, err
	}

	if err := t.parent.initialize(); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("initialize vision model: %w", err)
	}

	texts := make([]string, len(inputs))
	for i, b := range inputs {
		texts[i] = string(b)
	}

	ortSingleton.mu.Lock()
	defer ortSingleton.mu.Unlock()

	result, err := t.parent.textPipeline.RunPipeline(texts)
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("run text pipeline: %w", err)
	}

	return toEmbeddingResponse(result.Embeddings), nil
}

func toEmbeddingResponse(embeddings [][]float32) EmbeddingResponse {
	vecs := make([][]float64, len(embeddings))
	for i, vec32 := range embeddings {
		vec64 := make([]float64, len(vec32))
		for j, v := range vec32 {
			vec64[j] = float64(v)
		}
		vecs[i] = vec64
	}
	return NewEmbeddingResponse(vecs, NewUsage(0, 0, 0))
}

var (
	_ Embedder = (*localVisionEmbedder)(nil)
	_ Embedder = (*localTextEmbedder)(nil)
)
