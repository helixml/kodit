package search

import (
	"math"
	"sort"
)

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 (opposite) and 1 (identical).
// Returns 0 if either vector has zero magnitude.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, magA, magB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}

	if magA == 0 || magB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(magA) * math.Sqrt(magB))
}

// SimilarityMatch holds a snippet ID and its similarity score.
type SimilarityMatch struct {
	snippetID  string
	similarity float64
}

// NewSimilarityMatch creates a new SimilarityMatch.
func NewSimilarityMatch(snippetID string, similarity float64) SimilarityMatch {
	return SimilarityMatch{
		snippetID:  snippetID,
		similarity: similarity,
	}
}

// SnippetID returns the snippet identifier.
func (m SimilarityMatch) SnippetID() string { return m.snippetID }

// Similarity returns the similarity score.
func (m SimilarityMatch) Similarity() float64 { return m.similarity }

// StoredVector holds an embedding vector with its snippet ID.
type StoredVector struct {
	snippetID string
	embedding []float64
}

// NewStoredVector creates a new StoredVector.
func NewStoredVector(snippetID string, embedding []float64) StoredVector {
	vec := make([]float64, len(embedding))
	copy(vec, embedding)
	return StoredVector{
		snippetID: snippetID,
		embedding: vec,
	}
}

// SnippetID returns the snippet identifier.
func (v StoredVector) SnippetID() string { return v.snippetID }

// Embedding returns the embedding vector (copy).
func (v StoredVector) Embedding() []float64 {
	result := make([]float64, len(v.embedding))
	copy(result, v.embedding)
	return result
}

// TopKSimilar finds the top-k most similar vectors to the query.
// Returns results sorted by similarity in descending order (highest similarity first).
func TopKSimilar(query []float64, vectors []StoredVector, k int) []SimilarityMatch {
	if len(vectors) == 0 || k <= 0 {
		return []SimilarityMatch{}
	}

	// Compute similarities for all vectors
	matches := make([]SimilarityMatch, 0, len(vectors))
	for _, v := range vectors {
		similarity := CosineSimilarity(query, v.embedding)
		matches = append(matches, NewSimilarityMatch(v.snippetID, similarity))
	}

	// Sort by similarity descending (highest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].similarity > matches[j].similarity
	})

	// Return top-k
	if k > len(matches) {
		k = len(matches)
	}
	return matches[:k]
}

// TopKSimilarFiltered finds the top-k most similar vectors, filtering by allowed snippet IDs.
func TopKSimilarFiltered(query []float64, vectors []StoredVector, k int, allowedIDs map[string]struct{}) []SimilarityMatch {
	if len(vectors) == 0 || k <= 0 {
		return []SimilarityMatch{}
	}

	// If no filter provided, use regular TopKSimilar
	if len(allowedIDs) == 0 {
		return TopKSimilar(query, vectors, k)
	}

	// Compute similarities for filtered vectors
	matches := make([]SimilarityMatch, 0, len(vectors))
	for _, v := range vectors {
		if _, ok := allowedIDs[v.snippetID]; !ok {
			continue
		}
		similarity := CosineSimilarity(query, v.embedding)
		matches = append(matches, NewSimilarityMatch(v.snippetID, similarity))
	}

	// Sort by similarity descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].similarity > matches[j].similarity
	})

	// Return top-k
	if k > len(matches) {
		k = len(matches)
	}
	return matches[:k]
}
