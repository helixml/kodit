package search

import "context"

// Embedder converts text into embedding vectors.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}
