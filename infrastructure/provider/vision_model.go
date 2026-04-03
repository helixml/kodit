package provider

// VisionModelConfig describes how to load and preprocess images for a
// specific local ONNX vision-language model. Different models (SigLIP2,
// CLIP, etc.) provide different configs; the runtime code is shared.
type VisionModelConfig struct {
	// ModelDir is the subdirectory name under the models cache directory
	// (e.g. "google_siglip2-base-patch16-512").
	ModelDir string

	// VisionOnnx is the ONNX filename for the vision encoder
	// (e.g. "vision_model.onnx").
	VisionOnnx string

	// TextOnnx is the ONNX filename for the text encoder
	// (e.g. "text_model.onnx").
	TextOnnx string

	// ImageSize is the target height/width in pixels after resize and crop
	// (e.g. 512 for SigLIP2, 224 for CLIP ViT-B/32).
	ImageSize int

	// ImageMean is the per-channel normalization mean applied after rescaling
	// pixel values to [0, 1].
	ImageMean [3]float32

	// ImageStd is the per-channel normalization standard deviation.
	ImageStd [3]float32

	// VisionOutputName selects which model output to use for embeddings
	// (e.g. "pooler_output"). Empty string uses the first output.
	VisionOutputName string
}

// SigLIP2BaseConfig is the configuration for google/siglip2-base-patch16-512.
var SigLIP2BaseConfig = VisionModelConfig{
	ModelDir:         "google_siglip2-base-patch16-512",
	VisionOnnx:       "vision_model.onnx",
	TextOnnx:         "text_model.onnx",
	ImageSize:        512,
	ImageMean:        [3]float32{0.5, 0.5, 0.5},
	ImageStd:         [3]float32{0.5, 0.5, 0.5},
	VisionOutputName: "pooler_output",
}
