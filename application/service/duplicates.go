package service

import (
	"context"
	"math"
	"sort"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/domain/search"
)

// maxEmbeddingsForDuplicate is the maximum number of embeddings that will be
// compared pairwise. Larger sets are truncated to avoid O(N²) timeouts.
const maxEmbeddingsForDuplicate = 5000

// DuplicatePair holds two snippet IDs and their pairwise cosine similarity.
type DuplicatePair struct {
	SnippetIDA string
	SnippetIDB string
	Similarity float64
}

// DuplicateSearch finds semantically duplicated code snippets using pairwise
// embedding comparison.
type DuplicateSearch struct {
	codeVectorStore search.EmbeddingStore
	logger          zerolog.Logger
}

// NewDuplicateSearch creates a new DuplicateSearch service.
func NewDuplicateSearch(codeVectorStore search.EmbeddingStore, logger zerolog.Logger) *DuplicateSearch {
	return &DuplicateSearch{
		codeVectorStore: codeVectorStore,
		logger:          logger,
	}
}

// FindDuplicates returns snippet pairs whose code embeddings have cosine
// similarity ≥ threshold. Pairs are sorted by similarity descending and
// capped at limit. The bool return value is true when the embedding set was
// truncated to maxEmbeddingsForDuplicate.
//
// Returns empty slice (not an error) when the store is nil or has no data.
func (s *DuplicateSearch) FindDuplicates(
	ctx context.Context,
	repoIDs []int64,
	threshold float64,
	limit int,
) ([]DuplicatePair, bool, error) {
	if s.codeVectorStore == nil {
		return nil, false, nil
	}

	filters := search.NewFilters(search.WithSourceRepos(repoIDs))
	embeddings, err := s.codeVectorStore.FindAll(ctx, filters)
	if err != nil {
		return nil, false, err
	}

	if len(embeddings) == 0 {
		return nil, false, nil
	}

	truncated := false
	if len(embeddings) > maxEmbeddingsForDuplicate {
		s.logger.Warn().
			Int("total", len(embeddings)).
			Int("cap", maxEmbeddingsForDuplicate).
			Msg("truncating embeddings for duplicate detection")
		embeddings = embeddings[:maxEmbeddingsForDuplicate]
		truncated = true
	}

	// Normalize all vectors to unit length once, then pairwise similarity = dot product.
	type unitVec struct {
		snippetID string
		v         []float64
	}
	normalized := make([]unitVec, 0, len(embeddings))
	for _, emb := range embeddings {
		vec := emb.Vector()
		mag := magnitude(vec)
		if mag == 0 {
			continue // skip zero vectors
		}
		unit := make([]float64, len(vec))
		for i, x := range vec {
			unit[i] = x / mag
		}
		normalized = append(normalized, unitVec{snippetID: emb.SnippetID(), v: unit})
	}

	// Triangle scan: compare each pair (i, j) where j > i.
	var pairs []DuplicatePair
	for i := 0; i < len(normalized); i++ {
		for j := i + 1; j < len(normalized); j++ {
			sim := dotProduct(normalized[i].v, normalized[j].v)
			if sim >= threshold {
				pairs = append(pairs, DuplicatePair{
					SnippetIDA: normalized[i].snippetID,
					SnippetIDB: normalized[j].snippetID,
					Similarity: sim,
				})
			}
		}
	}

	// Sort by similarity descending.
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Similarity > pairs[j].Similarity
	})

	if limit > 0 && len(pairs) > limit {
		pairs = pairs[:limit]
	}

	return pairs, truncated, nil
}

// magnitude returns the Euclidean magnitude of a vector.
func magnitude(v []float64) float64 {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	return math.Sqrt(sum)
}

// dotProduct returns the dot product of two vectors (assumed equal length).
func dotProduct(a, b []float64) float64 {
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
