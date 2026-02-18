package search

import "time"

// Filters represents filters for snippet search.
type Filters struct {
	language           string
	author             string
	createdAfter       time.Time
	createdBefore      time.Time
	sourceRepo         int64
	filePath           string
	enrichmentTypes    []string
	enrichmentSubtypes []string
	commitSHAs         []string
}

// FiltersOption is a functional option for Filters.
type FiltersOption func(*Filters)

// WithLanguage sets the language filter.
func WithLanguage(language string) FiltersOption {
	return func(f *Filters) {
		f.language = language
	}
}

// WithAuthor sets the author filter.
func WithAuthor(author string) FiltersOption {
	return func(f *Filters) {
		f.author = author
	}
}

// WithCreatedAfter sets the created after filter.
func WithCreatedAfter(t time.Time) FiltersOption {
	return func(f *Filters) {
		f.createdAfter = t
	}
}

// WithCreatedBefore sets the created before filter.
func WithCreatedBefore(t time.Time) FiltersOption {
	return func(f *Filters) {
		f.createdBefore = t
	}
}

// WithSourceRepo sets the source repository filter.
func WithSourceRepo(repo int64) FiltersOption {
	return func(f *Filters) {
		f.sourceRepo = repo
	}
}

// WithFilePath sets the file path filter.
func WithFilePath(path string) FiltersOption {
	return func(f *Filters) {
		f.filePath = path
	}
}

// WithEnrichmentTypes sets the enrichment types filter.
func WithEnrichmentTypes(types []string) FiltersOption {
	return func(f *Filters) {
		if types != nil {
			f.enrichmentTypes = make([]string, len(types))
			copy(f.enrichmentTypes, types)
		}
	}
}

// WithEnrichmentSubtypes sets the enrichment subtypes filter.
func WithEnrichmentSubtypes(subtypes []string) FiltersOption {
	return func(f *Filters) {
		if subtypes != nil {
			f.enrichmentSubtypes = make([]string, len(subtypes))
			copy(f.enrichmentSubtypes, subtypes)
		}
	}
}

// WithCommitSHAs sets the commit SHA filter.
func WithCommitSHAs(shas []string) FiltersOption {
	return func(f *Filters) {
		if shas != nil {
			f.commitSHAs = make([]string, len(shas))
			copy(f.commitSHAs, shas)
		}
	}
}

// NewFilters creates a new Filters with options.
func NewFilters(opts ...FiltersOption) Filters {
	f := Filters{}
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

// Language returns the language filter.
func (f Filters) Language() string { return f.language }

// Author returns the author filter.
func (f Filters) Author() string { return f.author }

// CreatedAfter returns the created after filter.
func (f Filters) CreatedAfter() time.Time { return f.createdAfter }

// CreatedBefore returns the created before filter.
func (f Filters) CreatedBefore() time.Time { return f.createdBefore }

// SourceRepo returns the source repository filter.
func (f Filters) SourceRepo() int64 { return f.sourceRepo }

// FilePath returns the file path filter.
func (f Filters) FilePath() string { return f.filePath }

// EnrichmentTypes returns the enrichment types filter.
func (f Filters) EnrichmentTypes() []string {
	if f.enrichmentTypes == nil {
		return nil
	}
	result := make([]string, len(f.enrichmentTypes))
	copy(result, f.enrichmentTypes)
	return result
}

// EnrichmentSubtypes returns the enrichment subtypes filter.
func (f Filters) EnrichmentSubtypes() []string {
	if f.enrichmentSubtypes == nil {
		return nil
	}
	result := make([]string, len(f.enrichmentSubtypes))
	copy(result, f.enrichmentSubtypes)
	return result
}

// CommitSHAs returns the commit SHA filter.
func (f Filters) CommitSHAs() []string {
	if f.commitSHAs == nil {
		return nil
	}
	result := make([]string, len(f.commitSHAs))
	copy(result, f.commitSHAs)
	return result
}

// IsEmpty returns true if no filters are set.
func (f Filters) IsEmpty() bool {
	return f.language == "" &&
		f.author == "" &&
		f.createdAfter.IsZero() &&
		f.createdBefore.IsZero() &&
		f.sourceRepo == 0 &&
		f.filePath == "" &&
		len(f.enrichmentTypes) == 0 &&
		len(f.enrichmentSubtypes) == 0 &&
		len(f.commitSHAs) == 0
}
