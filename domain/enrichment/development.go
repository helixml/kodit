package enrichment

// NewSnippetEnrichment creates a snippet enrichment for a commit.
// Snippet enrichments describe code snippets within the repository.
func NewSnippetEnrichment(content string) Enrichment {
	return NewEnrichment(TypeDevelopment, SubtypeSnippet, EntityTypeCommit, content)
}

// NewSnippetEnrichmentWithLanguage creates a snippet enrichment with language metadata.
// The language field preserves the file extension so search results can report it.
func NewSnippetEnrichmentWithLanguage(content, language string) Enrichment {
	return NewEnrichmentWithLanguage(TypeDevelopment, SubtypeSnippet, EntityTypeCommit, content, language)
}

// NewSnippetSummary creates a snippet summary enrichment for a commit.
// Snippet summaries provide AI-generated descriptions of code snippets.
func NewSnippetSummary(content string) Enrichment {
	return NewEnrichment(TypeDevelopment, SubtypeSnippetSummary, EntityTypeCommit, content)
}

// NewExample creates an example enrichment for a commit.
// Examples are code samples extracted from documentation.
func NewExample(content string) Enrichment {
	return NewEnrichment(TypeDevelopment, SubtypeExample, EntityTypeCommit, content)
}

// NewExampleSummary creates an example summary enrichment for a commit.
// Example summaries provide AI-generated descriptions of code examples.
func NewExampleSummary(content string) Enrichment {
	return NewEnrichment(TypeDevelopment, SubtypeExampleSummary, EntityTypeCommit, content)
}

// NewChunkEnrichment creates a chunk enrichment for a commit.
// Chunk enrichments represent fixed-size text chunks of repository files.
func NewChunkEnrichment(content string) Enrichment {
	return NewEnrichment(TypeDevelopment, SubtypeChunk, EntityTypeCommit, content)
}

// NewChunkEnrichmentWithLanguage creates a chunk enrichment with language metadata.
// The language field preserves the file extension so search results can report it.
func NewChunkEnrichmentWithLanguage(content, language string) Enrichment {
	return NewEnrichmentWithLanguage(TypeDevelopment, SubtypeChunk, EntityTypeCommit, content, language)
}

// IsDevelopmentEnrichment returns true if the enrichment is a development type.
func IsDevelopmentEnrichment(e Enrichment) bool {
	return e.Type() == TypeDevelopment
}

// IsSnippetEnrichment returns true if the enrichment is a snippet subtype.
func IsSnippetEnrichment(e Enrichment) bool {
	return e.Type() == TypeDevelopment && e.Subtype() == SubtypeSnippet
}

// IsSnippetSummary returns true if the enrichment is a snippet summary subtype.
func IsSnippetSummary(e Enrichment) bool {
	return e.Type() == TypeDevelopment && e.Subtype() == SubtypeSnippetSummary
}

// IsExample returns true if the enrichment is an example subtype.
func IsExample(e Enrichment) bool {
	return e.Type() == TypeDevelopment && e.Subtype() == SubtypeExample
}

// IsExampleSummary returns true if the enrichment is an example summary subtype.
func IsExampleSummary(e Enrichment) bool {
	return e.Type() == TypeDevelopment && e.Subtype() == SubtypeExampleSummary
}
