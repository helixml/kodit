package enrichment

// NewCookbook creates a cookbook enrichment for a commit.
// Cookbooks are task-oriented usage guides generated from code patterns.
func NewCookbook(content string) Enrichment {
	return NewEnrichment(TypeUsage, SubtypeCookbook, EntityTypeCommit, content)
}

// NewAPIDocs creates an API documentation enrichment for a commit.
// API docs describe public interfaces extracted from code.
func NewAPIDocs(content, language string) Enrichment {
	return NewEnrichmentWithLanguage(TypeUsage, SubtypeAPIDocs, EntityTypeCommit, content, language)
}

// NewAPIDocsAttempt creates a sentinel enrichment marking that API doc
// extraction has been attempted for a commit. It distinguishes "we tried and
// got nothing" from "we haven't tried yet" so the handler skips on the next
// run instead of re-running extraction every cycle.
//
// The marker uses EntityTypeCommit (real per-file API docs use
// EntityTypeSnippet) and empty content. Consumers reading API docs filter
// markers out by skipping empty-content enrichments.
func NewAPIDocsAttempt() Enrichment {
	return NewEnrichment(TypeUsage, SubtypeAPIDocs, EntityTypeCommit, "")
}

// IsAPIDocsAttempt returns true if the enrichment is the per-commit attempt
// marker (empty content, EntityTypeCommit) rather than real per-file API docs.
func IsAPIDocsAttempt(e Enrichment) bool {
	return IsAPIDocs(e) && e.EntityTypeKey() == EntityTypeCommit && e.Content() == ""
}

// IsUsageEnrichment returns true if the enrichment is a usage type.
func IsUsageEnrichment(e Enrichment) bool {
	return e.Type() == TypeUsage
}

// IsCookbook returns true if the enrichment is a cookbook subtype.
func IsCookbook(e Enrichment) bool {
	return e.Type() == TypeUsage && e.Subtype() == SubtypeCookbook
}

// IsAPIDocs returns true if the enrichment is an API docs subtype.
func IsAPIDocs(e Enrichment) bool {
	return e.Type() == TypeUsage && e.Subtype() == SubtypeAPIDocs
}

// NewWiki creates a wiki enrichment for a commit.
// Wikis are multi-page structured documentation generated from repository content.
func NewWiki(content string) Enrichment {
	return NewEnrichment(TypeUsage, SubtypeWiki, EntityTypeCommit, content)
}

// IsWiki returns true if the enrichment is a wiki subtype.
func IsWiki(e Enrichment) bool {
	return e.Type() == TypeUsage && e.Subtype() == SubtypeWiki
}
