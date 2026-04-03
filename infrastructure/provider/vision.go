package provider

import (
	"context"
)

// VisionEmbeddingRequest holds images to embed.
type VisionEmbeddingRequest struct {
	images []VisionImage
}

// NewVisionEmbeddingRequest creates a VisionEmbeddingRequest.
func NewVisionEmbeddingRequest(images []VisionImage) VisionEmbeddingRequest {
	imgs := make([]VisionImage, len(images))
	copy(imgs, images)
	return VisionEmbeddingRequest{images: imgs}
}

// Images returns the images to embed.
func (r VisionEmbeddingRequest) Images() []VisionImage {
	imgs := make([]VisionImage, len(r.images))
	copy(imgs, r.images)
	return imgs
}

// VisionImage holds raw image data for embedding at the provider layer.
type VisionImage struct {
	data     []byte
	mimeType string
}

// NewVisionImage creates a VisionImage.
func NewVisionImage(data []byte, mimeType string) VisionImage {
	cp := make([]byte, len(data))
	copy(cp, data)
	return VisionImage{data: cp, mimeType: mimeType}
}

// Data returns a defensive copy of the image bytes.
func (i VisionImage) Data() []byte {
	cp := make([]byte, len(i.data))
	copy(cp, i.data)
	return cp
}

// MIMEType returns the image MIME type.
func (i VisionImage) MIMEType() string { return i.mimeType }

// VisionEmbedder generates embeddings for images and text queries in a
// shared embedding space (CLIP-style dual encoder).
type VisionEmbedder interface {
	// EmbedImages returns one embedding vector per image.
	EmbedImages(ctx context.Context, req VisionEmbeddingRequest) (EmbeddingResponse, error)

	// EmbedQuery embeds a text description in the vision embedding space,
	// producing a vector comparable to image embeddings via cosine similarity.
	EmbedQuery(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}
