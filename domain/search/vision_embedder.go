package search

import "context"

// VisionEmbedder converts images and text queries into embedding vectors
// within a shared embedding space (CLIP-style). Both EmbedImages and
// EmbedQuery produce vectors in the same space, enabling cross-modal
// similarity search (e.g. finding images by text description).
type VisionEmbedder interface {
	// EmbedImages returns one embedding vector per image.
	EmbedImages(ctx context.Context, images []Image) ([][]float64, error)

	// EmbedQuery embeds a text description in the vision embedding space.
	EmbedQuery(ctx context.Context, text string) ([]float64, error)
}

// Image holds raw image data for embedding. The format (PNG, JPEG, etc.)
// is detected automatically from the data bytes.
type Image struct {
	data []byte
}

// NewImage creates an Image from raw bytes.
func NewImage(data []byte) Image {
	cp := make([]byte, len(data))
	copy(cp, data)
	return Image{data: cp}
}

// Data returns a defensive copy of the image bytes.
func (i Image) Data() []byte {
	cp := make([]byte, len(i.data))
	copy(cp, i.data)
	return cp
}
