// Package search provides code search functionality with hybrid retrieval.
package search

import (
	"sort"

	"github.com/helixml/kodit/internal/domain"
)

// FusionService combines results from multiple search methods using
// Reciprocal Rank Fusion (RRF) algorithm.
type FusionService struct {
	k float64 // RRF constant (typically 60)
}

// NewFusionService creates a FusionService with the default RRF constant.
func NewFusionService() FusionService {
	return FusionService{k: 60.0}
}

// NewFusionServiceWithK creates a FusionService with a custom RRF constant.
func NewFusionServiceWithK(k float64) FusionService {
	if k <= 0 {
		k = 60.0
	}
	return FusionService{k: k}
}

// Fuse combines multiple ranked result lists using Reciprocal Rank Fusion.
// Each input list should be sorted by score (descending).
// Returns a fused list sorted by combined RRF score.
func (f FusionService) Fuse(lists ...[]domain.FusionRequest) []domain.FusionResult {
	if len(lists) == 0 {
		return []domain.FusionResult{}
	}

	// Track accumulated RRF scores and original scores per document
	scores := make(map[string]float64)
	originals := make(map[string][]float64)

	// Initialize original scores map with placeholders
	for range lists {
		// Will be filled as we process each list
	}

	// Process each ranked list
	for listIdx, list := range lists {
		for rank, req := range list {
			id := req.ID()

			// RRF formula: 1 / (k + rank)
			// rank is 0-indexed, but RRF uses 1-indexed ranks
			rrfScore := 1.0 / (f.k + float64(rank+1))
			scores[id] += rrfScore

			// Track original scores for this document
			if _, exists := originals[id]; !exists {
				originals[id] = make([]float64, len(lists))
			}
			originals[id][listIdx] = req.Score()
		}
	}

	// Convert to result slice
	results := make([]domain.FusionResult, 0, len(scores))
	for id, score := range scores {
		results = append(results, domain.NewFusionResult(id, score, originals[id]))
	}

	// Sort by fused score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score() > results[j].Score()
	})

	return results
}

// FuseTopK combines multiple ranked result lists and returns the top K results.
func (f FusionService) FuseTopK(topK int, lists ...[]domain.FusionRequest) []domain.FusionResult {
	results := f.Fuse(lists...)

	if topK <= 0 || topK >= len(results) {
		return results
	}

	return results[:topK]
}

// K returns the RRF constant used by this service.
func (f FusionService) K() float64 {
	return f.k
}
