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
