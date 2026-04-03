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
)

const (
	siglip2ModelDir   = "google_siglip2-base-patch16-512"
	siglip2VisionOnnx = "vision_model.onnx"
	siglip2TextOnnx   = "text_model.onnx"
)

// SigLIP2Embedding provides vision and text embedding using the SigLIP2
// dual-encoder model. Images and text queries are embedded into the same
// 768-dimensional space, enabling cross-modal similarity search.
//
// The model is loaded from cacheDir (disk or extracted from the embedded
// binary) and shares the process-wide ORT session with HugotEmbedding.
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

// Available reports whether the SigLIP2 model files exist on disk or
// are embedded in the binary.
func (s *SigLIP2Embedding) Available() bool {
	if hasEmbeddedModel {
		modelDir := filepath.Join(s.cacheDir, siglip2ModelDir)
		// Check extracted cache first.
		if s.hasModelFiles(modelDir) {
			return true
		}
		// Check the embedded FS.
		visionPath := filepath.Join("models", siglip2ModelDir, "onnx", siglip2VisionOnnx)
		if _, err := embeddedModelFS.Open(visionPath); err == nil {
			return true
		}
	}
	return s.hasModelFiles(filepath.Join(s.cacheDir, siglip2ModelDir))
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

	// Reuse existing pipelines if already registered on the session.
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
		visionConfig := hugot.FeatureExtractionConfig{
			ModelPath:    modelPath,
			Name:         siglip2VisionPipeline,
			OnnxFilename: siglip2VisionOnnx,
			Options: []hugot.FeatureExtractionOption{
				pipelines.WithNormalization(),
				pipelines.WithImageMode(),
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

	extracted, err := extractEmbeddedModelByName(embeddedModelFS, s.cacheDir, siglip2ModelDir)
	if err != nil {
		return "", err
	}
	return extracted, nil
}

// EmbedImages returns one embedding vector per image.
func (s *SigLIP2Embedding) EmbedImages(ctx context.Context, req VisionEmbeddingRequest) (EmbeddingResponse, error) {
	images := req.Images()
	if len(images) == 0 {
		return NewEmbeddingResponse([][]float64{}, NewUsage(0, 0, 0)), nil
	}

	if err := ctx.Err(); err != nil {
		return EmbeddingResponse{}, err
	}

	if err := s.initialize(); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("initialize siglip2: %w", err)
	}

	goImages := make([]image.Image, len(images))
	for i, img := range images {
		decoded, _, err := image.Decode(bytes.NewReader(img.Data()))
		if err != nil {
			return EmbeddingResponse{}, fmt.Errorf("decode image %d: %w", i, err)
		}
		goImages[i] = decoded
	}

	ortSingleton.mu.Lock()
	defer ortSingleton.mu.Unlock()

	result, err := s.visionPipeline.RunWithImages(goImages)
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("run vision pipeline: %w", err)
	}

	return toEmbeddingResponse(result.Embeddings), nil
}

// EmbedQuery embeds a text description in the vision embedding space.
func (s *SigLIP2Embedding) EmbedQuery(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	texts := req.Texts()
	if len(texts) == 0 {
		return NewEmbeddingResponse([][]float64{}, NewUsage(0, 0, 0)), nil
	}

	if err := ctx.Err(); err != nil {
		return EmbeddingResponse{}, err
	}

	if err := s.initialize(); err != nil {
		return EmbeddingResponse{}, fmt.Errorf("initialize siglip2: %w", err)
	}

	ortSingleton.mu.Lock()
	defer ortSingleton.mu.Unlock()

	result, err := s.textPipeline.RunPipeline(texts)
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("run text pipeline: %w", err)
	}

	return toEmbeddingResponse(result.Embeddings), nil
}

// Close is a no-op. The ORT session is process-global.
func (s *SigLIP2Embedding) Close() error {
	return nil
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

var _ VisionEmbedder = (*SigLIP2Embedding)(nil)
