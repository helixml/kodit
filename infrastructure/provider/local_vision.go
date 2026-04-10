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

	"github.com/helixml/kodit/domain/search"
)

// LocalVisionEmbedding manages a local ONNX dual-encoder vision-language
// model. It implements search.Embedder, dispatching each item to either
// the vision or text encoder based on its payload. Both encoders produce
// vectors in the same embedding space.
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

// Embed dispatches each item to the vision or text ONNX pipeline based on
// which payload the item carries. Image items go through the vision encoder,
// text items go through the text encoder; both produce vectors in the same
// embedding space. Items carrying both payloads use the image encoder (the
// local SigLIP2 model is a dual encoder and cannot embed a combined input).
func (l *LocalVisionEmbedding) Embed(ctx context.Context, items []search.EmbeddingItem) ([][]float64, error) {
	if len(items) == 0 {
		return [][]float64{}, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := l.initialize(); err != nil {
		return nil, fmt.Errorf("initialize vision model: %w", err)
	}

	imgIdx := make([]int, 0, len(items))
	txtIdx := make([]int, 0, len(items))
	imgs := make([]image.Image, 0, len(items))
	texts := make([]string, 0, len(items))
	for i, item := range items {
		switch {
		case item.HasImage():
			decoded, _, err := image.Decode(bytes.NewReader(item.Image()))
			if err != nil {
				return nil, fmt.Errorf("decode image %d: %w", i, err)
			}
			imgs = append(imgs, decoded)
			imgIdx = append(imgIdx, i)
		case item.HasText():
			texts = append(texts, string(item.Text()))
			txtIdx = append(txtIdx, i)
		default:
			return nil, fmt.Errorf("vision embedding item %d has neither text nor image", i)
		}
	}

	results := make([][]float64, len(items))

	ortSingleton.mu.Lock()
	defer ortSingleton.mu.Unlock()

	if len(imgs) > 0 {
		result, err := l.visionPipeline.RunWithImages(imgs)
		if err != nil {
			return nil, fmt.Errorf("run vision pipeline: %w", err)
		}
		for i, idx := range imgIdx {
			results[idx] = float32sToFloat64s(result.Embeddings[i])
		}
	}

	if len(texts) > 0 {
		result, err := l.textPipeline.RunPipeline(texts)
		if err != nil {
			return nil, fmt.Errorf("run text pipeline: %w", err)
		}
		for i, idx := range txtIdx {
			results[idx] = float32sToFloat64s(result.Embeddings[i])
		}
	}

	return results, nil
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

func float32sToFloat64s(v32 []float32) []float64 {
	v64 := make([]float64, len(v32))
	for i, v := range v32 {
		v64[i] = float64(v)
	}
	return v64
}

// float32MatrixToFloat64 converts a slice of float32 vectors into a slice
// of float64 vectors.
func float32MatrixToFloat64(embeddings [][]float32) [][]float64 {
	vecs := make([][]float64, len(embeddings))
	for i, vec32 := range embeddings {
		vecs[i] = float32sToFloat64s(vec32)
	}
	return vecs
}

var _ search.Embedder = (*LocalVisionEmbedding)(nil)
