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

const (
	siglip2ModelDir   = "google_siglip2-base-patch16-512"
	siglip2VisionOnnx = "vision_model.onnx"
	siglip2TextOnnx   = "text_model.onnx"
)

// SigLIP2Embedding manages the SigLIP2 dual-encoder model and exposes two
// Embedder implementations that produce vectors in the same 768-dimensional
// space: one for images, one for text queries. Both share the process-wide
// ORT session and the same model directory.
type SigLIP2Embedding struct {
	cacheDir       string
	visionPipeline *pipelines.FeatureExtractionPipeline
	textPipeline   *pipelines.FeatureExtractionPipeline
}

// NewSigLIP2Embedding creates a SigLIP2Embedding that looks for the
// siglip2 model files in cacheDir.
func NewSigLIP2Embedding(cacheDir string) *SigLIP2Embedding {
	return &SigLIP2Embedding{cacheDir: cacheDir}
}

// VisionEmbedder returns an Embedder that accepts image bytes (PNG, JPEG)
// and produces vectors in SigLIP2's shared embedding space.
func (s *SigLIP2Embedding) VisionEmbedder() Embedder {
	return &siglip2VisionEmbedder{parent: s}
}

// TextEmbedder returns an Embedder that accepts UTF-8 text and produces
// vectors in the same embedding space as VisionEmbedder. Use this to embed
// text queries for cross-modal image search.
func (s *SigLIP2Embedding) TextEmbedder() Embedder {
	return &siglip2TextEmbedder{parent: s}
}

// Available reports whether the SigLIP2 model files exist on disk or
// are embedded in the binary.
func (s *SigLIP2Embedding) Available() bool {
	if hasEmbeddedModel {
		modelDir := filepath.Join(s.cacheDir, siglip2ModelDir)
		if s.hasModelFiles(modelDir) {
			return true
		}
		visionPath := filepath.Join("models", siglip2ModelDir, "onnx", siglip2VisionOnnx)
		if _, err := embeddedModelFS.Open(visionPath); err == nil {
			return true
		}
	}
	return s.hasModelFiles(filepath.Join(s.cacheDir, siglip2ModelDir))
}

// Close is a no-op. The ORT session is process-global.
func (s *SigLIP2Embedding) Close() error {
	return nil
}

func (s *SigLIP2Embedding) hasModelFiles(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "onnx", siglip2VisionOnnx)); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "onnx", siglip2TextOnnx)); err != nil {
		return false
	}
	return true
}

const (
	siglip2VisionPipeline = "siglip2-vision"
	siglip2TextPipeline   = "siglip2-text"
)

func (s *SigLIP2Embedding) initialize() error {
	if s.visionPipeline != nil && s.textPipeline != nil {
		return nil
	}

	session, err := ensureORTSession()
	if err != nil {
		return err
	}

	if v, vErr := hugot.GetPipeline[*pipelines.FeatureExtractionPipeline](session, siglip2VisionPipeline); vErr == nil {
		s.visionPipeline = v
	}
	if t, tErr := hugot.GetPipeline[*pipelines.FeatureExtractionPipeline](session, siglip2TextPipeline); tErr == nil {
		s.textPipeline = t
	}
	if s.visionPipeline != nil && s.textPipeline != nil {
		return nil
	}

	modelPath, err := s.resolveModelPath()
	if err != nil {
		return err
	}

	if s.visionPipeline == nil {
		siglipNorm := imageutil.PixelNormalizationStep(
			[3]float32{0.5, 0.5, 0.5},
			[3]float32{0.5, 0.5, 0.5},
		)
		visionConfig := hugot.FeatureExtractionConfig{
			ModelPath:    modelPath,
			Name:         siglip2VisionPipeline,
			OnnxFilename: siglip2VisionOnnx,
			Options: []hugot.FeatureExtractionOption{
				pipelines.WithNormalization(),
				pipelines.WithImageMode(),
				pipelines.WithOutputName("pooler_output"),
				pipelines.WithPreprocessSteps[*pipelines.FeatureExtractionPipeline](
					imageutil.ResizeStep(512),
					imageutil.CenterCropStep(512, 512),
				),
				pipelines.WithNormalizationSteps[*pipelines.FeatureExtractionPipeline](
					imageutil.RescaleStep(),
					siglipNorm,
				),
			},
		}
		visionPipeline, err := hugot.NewPipeline(session, visionConfig)
		if err != nil {
			return fmt.Errorf("create siglip2 vision pipeline: %w", err)
		}
		s.visionPipeline = visionPipeline
	}

	if s.textPipeline == nil {
		textConfig := hugot.FeatureExtractionConfig{
			ModelPath:    modelPath,
			Name:         siglip2TextPipeline,
			OnnxFilename: siglip2TextOnnx,
			Options: []hugot.FeatureExtractionOption{
				pipelines.WithNormalization(),
			},
		}
		textPipeline, err := hugot.NewPipeline(session, textConfig)
		if err != nil {
			return fmt.Errorf("create siglip2 text pipeline: %w", err)
		}
		s.textPipeline = textPipeline
	}

	return nil
}

func (s *SigLIP2Embedding) resolveModelPath() (string, error) {
	diskPath := filepath.Join(s.cacheDir, siglip2ModelDir)
	if s.hasModelFiles(diskPath) {
		return diskPath, nil
	}

	if !hasEmbeddedModel {
		return "", fmt.Errorf("siglip2 model not found in %s and no embedded model compiled in", s.cacheDir)
	}

	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create cache directory: %w", err)
	}

	return extractEmbeddedModelByName(embeddedModelFS, s.cacheDir, siglip2ModelDir)
}

// siglip2VisionEmbedder implements Embedder for image inputs.
type siglip2VisionEmbedder struct {
	parent *SigLIP2Embedding
}

func (v *siglip2VisionEmbedder) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	inputs := req.Inputs()
	if len(inputs) == 0 {
		return NewEmbeddingResponse([][]float64{}, NewUsage(0, 0, 0)), nil
	}

	if err := ctx.Err(); err != nil {
		return EmbeddingResponse{}, err
	}

	if err := v.parent.initialize(); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("initialize siglip2: %w", err)
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

// siglip2TextEmbedder implements Embedder for text query inputs.
type siglip2TextEmbedder struct {
	parent *SigLIP2Embedding
}

func (t *siglip2TextEmbedder) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	inputs := req.Inputs()
	if len(inputs) == 0 {
		return NewEmbeddingResponse([][]float64{}, NewUsage(0, 0, 0)), nil
	}

	if err := ctx.Err(); err != nil {
		return EmbeddingResponse{}, err
	}

	if err := t.parent.initialize(); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("initialize siglip2: %w", err)
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
	_ Embedder = (*siglip2VisionEmbedder)(nil)
	_ Embedder = (*siglip2TextEmbedder)(nil)
)
