package search

import "time"

// Filters represents filters for snippet search.
type Filters struct {
	languages          []string
	authors            []string
	createdAfter       time.Time
	createdBefore      time.Time
	sourceRepos        []int64
	filePaths          []string
	enrichmentTypes    []string
	enrichmentSubtypes []string
	commitSHAs         []string
}

// FiltersOption is a functional option for Filters.
type FiltersOption func(*Filters)

// WithLanguages sets the language filter.
func WithLanguages(languages []string) FiltersOption {
	return func(f *Filters) {
		if languages != nil {
			f.languages = make([]string, len(languages))
			copy(f.languages, languages)
		}
	}
}

// WithAuthors sets the author filter.
func WithAuthors(authors []string) FiltersOption {
	return func(f *Filters) {
		if authors != nil {
			f.authors = make([]string, len(authors))
			copy(f.authors, authors)
		}
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

// WithSourceRepos sets the source repository filter.
func WithSourceRepos(repos []int64) FiltersOption {
	return func(f *Filters) {
		if repos != nil {
			f.sourceRepos = make([]int64, len(repos))
			copy(f.sourceRepos, repos)
		}
	}
}

// WithFilePaths sets the file path filter.
func WithFilePaths(paths []string) FiltersOption {
	return func(f *Filters) {
		if paths != nil {
			f.filePaths = make([]string, len(paths))
			copy(f.filePaths, paths)
		}
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

// Languages returns the language filter.
func (f Filters) Languages() []string {
	if f.languages == nil {
		return nil
	}
	result := make([]string, len(f.languages))
	copy(result, f.languages)
	return result
}

// Authors returns the author filter.
func (f Filters) Authors() []string {
	if f.authors == nil {
		return nil
	}
	result := make([]string, len(f.authors))
	copy(result, f.authors)
	return result
}

// CreatedAfter returns the created after filter.
func (f Filters) CreatedAfter() time.Time { return f.createdAfter }

// CreatedBefore returns the created before filter.
func (f Filters) CreatedBefore() time.Time { return f.createdBefore }

// SourceRepos returns the source repository filter.
func (f Filters) SourceRepos() []int64 {
	if f.sourceRepos == nil {
		return nil
	}
	result := make([]int64, len(f.sourceRepos))
	copy(result, f.sourceRepos)
	return result
}

// FilePaths returns the file path filter.
func (f Filters) FilePaths() []string {
	if f.filePaths == nil {
		return nil
	}
	result := make([]string, len(f.filePaths))
	copy(result, f.filePaths)
	return result
}

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
	return len(f.languages) == 0 &&
		len(f.authors) == 0 &&
		f.createdAfter.IsZero() &&
		f.createdBefore.IsZero() &&
		len(f.sourceRepos) == 0 &&
		len(f.filePaths) == 0 &&
		len(f.enrichmentTypes) == 0 &&
		len(f.enrichmentSubtypes) == 0 &&
		len(f.commitSHAs) == 0
}
