package enrichment

// NewCommitDescription creates a commit description enrichment.
// Commit descriptions provide AI-generated summaries of what a commit changed.
func NewCommitDescription(content string) Enrichment {
	return NewEnrichment(TypeHistory, SubtypeCommitDescription, EntityTypeCommit, content)
}

// IsHistoryEnrichment returns true if the enrichment is a history type.
func IsHistoryEnrichment(e Enrichment) bool {
	return e.Type() == TypeHistory
}

// IsCommitDescription returns true if the enrichment is a commit description subtype.
func IsCommitDescription(e Enrichment) bool {
	return e.Type() == TypeHistory && e.Subtype() == SubtypeCommitDescription
}
