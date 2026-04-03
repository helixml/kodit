package search

import "context"

// Embedder converts content into embedding vectors. The input bytes may be
// UTF-8 text or encoded images (PNG, JPEG, etc.) depending on the model
// backing the implementation. All implementations accept [][]byte so that
// text and vision embedders share a single interface.
type Embedder interface {
	Embed(ctx context.Context, inputs [][]byte) ([][]float64, error)
}
