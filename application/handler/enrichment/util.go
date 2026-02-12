// Package enrichment provides task handlers for enrichment operations.
package enrichment

// TruncateDiff truncates a diff to a reasonable length for LLM processing.
func TruncateDiff(diff string, maxLength int) string {
	if len(diff) <= maxLength {
		return diff
	}
	truncationNotice := "\n\n[diff truncated due to size]"
	return diff[:maxLength-len(truncationNotice)] + truncationNotice
}

// MaxDiffLength is the maximum characters for a commit diff (~25k tokens).
const MaxDiffLength = 100_000
