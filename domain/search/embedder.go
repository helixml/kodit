package search

import "context"

// EmbeddingItem is a single input to an Embedder. It carries an optional
// text payload and an optional image payload. A text-only embedder uses
// only the text field and ignores the image; a vision embedder may use
// either, or both if the underlying model supports multimodal inputs.
// A nil field means "absent".
type EmbeddingItem struct {
	text  []byte
	image []byte
}

// NewTextItem creates an EmbeddingItem containing text only.
func NewTextItem(text string) EmbeddingItem {
	return EmbeddingItem{text: []byte(text)}
}

// NewImageItem creates an EmbeddingItem containing image bytes only.
func NewImageItem(image []byte) EmbeddingItem {
	cp := make([]byte, len(image))
	copy(cp, image)
	return EmbeddingItem{image: cp}
}

// NewMultimodalItem creates an EmbeddingItem with both text and image.
// Only embedders for models that support multimodal inputs will use both.
func NewMultimodalItem(text string, image []byte) EmbeddingItem {
	cp := make([]byte, len(image))
	copy(cp, image)
	return EmbeddingItem{text: []byte(text), image: cp}
}

// NewTextItems is a convenience constructor that wraps a slice of text
// strings as a slice of text-only EmbeddingItem values.
func NewTextItems(texts []string) []EmbeddingItem {
	items := make([]EmbeddingItem, len(texts))
	for i, t := range texts {
		items[i] = NewTextItem(t)
	}
	return items
}

// NewImageItems is a convenience constructor that wraps a slice of image
// byte slices as a slice of image-only EmbeddingItem values.
func NewImageItems(images [][]byte) []EmbeddingItem {
	items := make([]EmbeddingItem, len(images))
	for i, img := range images {
		items[i] = NewImageItem(img)
	}
	return items
}

// Text returns the text bytes, or nil if no text is set.
func (i EmbeddingItem) Text() []byte {
	if i.text == nil {
		return nil
	}
	cp := make([]byte, len(i.text))
	copy(cp, i.text)
	return cp
}

// Image returns the image bytes, or nil if no image is set.
func (i EmbeddingItem) Image() []byte {
	if i.image == nil {
		return nil
	}
	cp := make([]byte, len(i.image))
	copy(cp, i.image)
	return cp
}

// HasText reports whether the item carries a text payload.
func (i EmbeddingItem) HasText() bool { return i.text != nil }

// HasImage reports whether the item carries an image payload.
func (i EmbeddingItem) HasImage() bool { return i.image != nil }

// Embedder converts items into embedding vectors. Each implementation
// decides which payload fields it uses: text embedders use text and
// ignore images; vision embedders use images and may also use text.
type Embedder interface {
	Embed(ctx context.Context, items []EmbeddingItem) ([][]float64, error)
}
