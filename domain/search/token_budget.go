package search

import "fmt"

// maxTextsPerBatch is an internal cap on the number of texts in a single
// embedding API call. This matches the previous DefaultBatchSize.
const maxTextsPerBatch = 10

// TokenBudget constrains embedding batches to stay within model token limits.
// It holds a single character budget: each batch's total (truncated) text
// must not exceed maxChars, and individual texts are truncated to maxChars.
type TokenBudget struct {
	maxChars int
}

// NewTokenBudget creates a TokenBudget with the given character limit.
// maxChars must be positive.
func NewTokenBudget(maxChars int) (TokenBudget, error) {
	if maxChars <= 0 {
		return TokenBudget{}, fmt.Errorf("NewTokenBudget: maxChars must be positive, got %d", maxChars)
	}
	return TokenBudget{maxChars: maxChars}, nil
}

// DefaultTokenBudget returns a conservative budget of 16 000 characters
// (~5 300 tokens at ~3 chars/token), safe for 8 192-token models like
// text-embedding-3-small.
func DefaultTokenBudget() TokenBudget {
	b, _ := NewTokenBudget(16000)
	return b
}

// Truncate returns text capped to the character limit.
func (b TokenBudget) Truncate(text string) string {
	if len(text) > b.maxChars {
		return text[:b.maxChars]
	}
	return text
}

// Batches partitions documents into groups whose total truncated character
// count stays within the budget. Each batch also contains at most 10 texts.
// A single document whose truncated text still exceeds the budget is placed
// alone in its own batch.
func (b TokenBudget) Batches(documents []Document) [][]Document {
	if len(documents) == 0 {
		return nil
	}

	var batches [][]Document
	i := 0

	for i < len(documents) {
		start := i
		batchChars := 0

		for i < len(documents) && i-start < maxTextsPerBatch {
			textLen := min(len(documents[i].Text()), b.maxChars)

			if batchChars+textLen > b.maxChars && i > start {
				break
			}

			batchChars += textLen
			i++
		}

		batch := make([]Document, i-start)
		copy(batch, documents[start:i])
		batches = append(batches, batch)
	}

	return batches
}
