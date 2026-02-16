package search

import "sort"

// Fusion combines results from multiple search methods using
// Reciprocal Rank Fusion (RRF) algorithm.
type Fusion struct {
	k float64 // RRF constant (typically 60)
}

// NewFusion creates a Fusion with the default RRF constant.
func NewFusion() Fusion {
	return Fusion{k: 60.0}
}

// NewFusionWithK creates a Fusion with a custom RRF constant.
func NewFusionWithK(k float64) Fusion {
	if k <= 0 {
		k = 60.0
	}
	return Fusion{k: k}
}

// Fuse combines multiple ranked result lists using Reciprocal Rank Fusion.
// Each input list should be sorted by score (descending).
// Returns a fused list sorted by combined RRF score.
func (f Fusion) Fuse(lists ...[]FusionRequest) []FusionResult {
	if len(lists) == 0 {
		return []FusionResult{}
	}

	// Track accumulated RRF scores and original scores per document
	scores := make(map[string]float64)
	originals := make(map[string][]float64)

	// Process each ranked list
	for listIdx, list := range lists {
		for rank, req := range list {
			id := req.ID()

			// RRF formula: 1 / (k + rank)
			rrfScore := 1.0 / (f.k + float64(rank))
			scores[id] += rrfScore

			// Track original scores for this document
			if _, exists := originals[id]; !exists {
				originals[id] = make([]float64, len(lists))
			}
			originals[id][listIdx] = req.Score()
		}
	}

	// Convert to result slice
	results := make([]FusionResult, 0, len(scores))
	for id, score := range scores {
		results = append(results, NewFusionResult(id, score, originals[id]))
	}

	// Sort by fused score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score() > results[j].Score()
	})

	return results
}

// FuseTopK combines multiple ranked result lists and returns the top K results.
func (f Fusion) FuseTopK(topK int, lists ...[]FusionRequest) []FusionResult {
	results := f.Fuse(lists...)

	if topK <= 0 || topK >= len(results) {
		return results
	}

	return results[:topK]
}

// K returns the RRF constant used by this service.
func (f Fusion) K() float64 {
	return f.k
}
