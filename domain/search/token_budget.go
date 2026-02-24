package search

import (
	"fmt"
	"unicode/utf8"
)

// TokenBudget constrains embedding batches to stay within model token limits.
// It holds a character budget and a maximum batch size: each batch's total
// (truncated) text must not exceed maxChars, each batch contains at most
// maxBatchSize documents, and individual texts are truncated to maxChars.
type TokenBudget struct {
	maxChars     int
	maxBatchSize int
}

// NewTokenBudget creates a TokenBudget with the given character limit.
// maxChars must be positive.
func NewTokenBudget(maxChars int) (TokenBudget, error) {
	if maxChars <= 0 {
		return TokenBudget{}, fmt.Errorf("NewTokenBudget: maxChars must be positive, got %d", maxChars)
	}
	return TokenBudget{maxChars: maxChars, maxBatchSize: 1}, nil
}

// DefaultTokenBudget returns a conservative budget of 16 000 characters
// (~5 300 tokens at ~3 chars/token), safe for 8 192-token models like
// text-embedding-3-small.
func DefaultTokenBudget() TokenBudget {
	b, _ := NewTokenBudget(16000)
	return b
}

// WithMaxBatchSize returns a new TokenBudget with the given maximum number
// of documents per batch. Values <= 0 are clamped to 1.
func (b TokenBudget) WithMaxBatchSize(n int) TokenBudget {
	if n <= 0 {
		n = 1
	}
	b.maxBatchSize = n
	return b
}

// Truncate returns text capped to the character (rune) limit.
func (b TokenBudget) Truncate(text string) string {
	if utf8.RuneCountInString(text) <= b.maxChars {
		return text
	}
	runes := []rune(text)
	return string(runes[:b.maxChars])
}

// Batches partitions documents into groups whose total truncated character
// count stays within the budget and whose size does not exceed maxBatchSize.
// A single document whose truncated text still exceeds the character budget
// is placed alone in its own batch.
func (b TokenBudget) Batches(documents []Document) [][]Document {
	if len(documents) == 0 {
		return nil
	}

	var batches [][]Document
	i := 0

	for i < len(documents) {
		start := i
		batchChars := 0

		for i < len(documents) {
			if i-start >= b.maxBatchSize && i > start {
				break
			}

			textLen := min(utf8.RuneCountInString(documents[i].Text()), b.maxChars)

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
