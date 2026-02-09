package service

import (
	"context"

	"github.com/helixml/kodit/domain/enrichment"
)

// EnrichmentQuery provides queries for enrichments and their associations.
type EnrichmentQuery struct {
	enrichmentStore enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
}

// NewEnrichmentQuery creates a new enrichment query service.
func NewEnrichmentQuery(
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
) *EnrichmentQuery {
	return &EnrichmentQuery{
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
	}
}

// EnrichmentsForCommit returns all enrichments associated with a commit.
func (s *EnrichmentQuery) EnrichmentsForCommit(
	ctx context.Context,
	commitSHA string,
	typ *enrichment.Type,
	subtype *enrichment.Subtype,
) ([]enrichment.Enrichment, error) {
	associations, err := s.associationStore.FindByEntityTypeAndID(ctx, enrichment.EntityTypeCommit, commitSHA)
	if err != nil {
		return nil, err
	}

	if len(associations) == 0 {
		return nil, nil
	}

	enrichmentIDs := make([]int64, 0, len(associations))
	for _, a := range associations {
		enrichmentIDs = append(enrichmentIDs, a.EnrichmentID())
	}

	var enrichments []enrichment.Enrichment
	for _, id := range enrichmentIDs {
		e, err := s.enrichmentStore.Get(ctx, id)
		if err != nil {
			continue
		}

		if typ != nil && e.Type() != *typ {
			continue
		}
		if subtype != nil && e.Subtype() != *subtype {
			continue
		}

		enrichments = append(enrichments, e)
	}

	return enrichments, nil
}

// HasSummariesForCommit checks if a commit has snippet summary enrichments.
func (s *EnrichmentQuery) HasSummariesForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := enrichment.TypeDevelopment
	sub := enrichment.SubtypeSnippetSummary
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasArchitectureForCommit checks if a commit has architecture enrichments.
func (s *EnrichmentQuery) HasArchitectureForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := enrichment.TypeArchitecture
	sub := enrichment.SubtypePhysical
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasAPIDocsForCommit checks if a commit has API documentation enrichments.
func (s *EnrichmentQuery) HasAPIDocsForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := enrichment.TypeUsage
	sub := enrichment.SubtypeAPIDocs
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasCommitDescriptionForCommit checks if a commit has commit description enrichments.
func (s *EnrichmentQuery) HasCommitDescriptionForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := enrichment.TypeHistory
	sub := enrichment.SubtypeCommitDescription
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasDatabaseSchemaForCommit checks if a commit has database schema enrichments.
func (s *EnrichmentQuery) HasDatabaseSchemaForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := enrichment.TypeArchitecture
	sub := enrichment.SubtypeDatabaseSchema
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasCookbookForCommit checks if a commit has cookbook enrichments.
func (s *EnrichmentQuery) HasCookbookForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := enrichment.TypeUsage
	sub := enrichment.SubtypeCookbook
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasExamplesForCommit checks if a commit has example enrichments.
func (s *EnrichmentQuery) HasExamplesForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := enrichment.TypeDevelopment
	sub := enrichment.SubtypeExample
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasExampleSummariesForCommit checks if a commit has example summary enrichments.
func (s *EnrichmentQuery) HasExampleSummariesForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := enrichment.TypeDevelopment
	sub := enrichment.SubtypeExampleSummary
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// SnippetsForCommit returns all snippet enrichments for a commit.
func (s *EnrichmentQuery) SnippetsForCommit(ctx context.Context, commitSHA string) ([]enrichment.Enrichment, error) {
	typ := enrichment.TypeDevelopment
	sub := enrichment.SubtypeSnippet
	return s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
}

// ExamplesForCommit returns all example enrichments for a commit.
func (s *EnrichmentQuery) ExamplesForCommit(ctx context.Context, commitSHA string) ([]enrichment.Enrichment, error) {
	typ := enrichment.TypeDevelopment
	sub := enrichment.SubtypeExample
	return s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
}

// Get retrieves a single enrichment by ID.
func (s *EnrichmentQuery) Get(ctx context.Context, id int64) (enrichment.Enrichment, error) {
	return s.enrichmentStore.Get(ctx, id)
}

// List returns enrichments matching the given filter.
func (s *EnrichmentQuery) List(ctx context.Context, filter enrichment.Filter) ([]enrichment.Enrichment, error) {
	typ := filter.FirstType()
	sub := filter.FirstSubtype()

	if typ != nil && sub != nil {
		return s.enrichmentStore.FindByTypeAndSubtype(ctx, *typ, *sub)
	}
	if typ != nil {
		return s.enrichmentStore.FindByType(ctx, *typ)
	}
	return []enrichment.Enrichment{}, nil
}

// Update replaces the content of an enrichment and returns the saved result.
func (s *EnrichmentQuery) Update(ctx context.Context, id int64, content string) (enrichment.Enrichment, error) {
	existing, err := s.enrichmentStore.Get(ctx, id)
	if err != nil {
		return enrichment.Enrichment{}, err
	}
	updated := existing.WithContent(content)
	return s.enrichmentStore.Save(ctx, updated)
}

// Delete removes an enrichment and its associations by ID.
func (s *EnrichmentQuery) Delete(ctx context.Context, id int64) error {
	existing, err := s.enrichmentStore.Get(ctx, id)
	if err != nil {
		return err
	}
	_ = s.associationStore.DeleteByEnrichmentID(ctx, id)
	return s.enrichmentStore.Delete(ctx, existing)
}

// DeleteForCommit removes all enrichments and associations for a commit.
func (s *EnrichmentQuery) DeleteForCommit(ctx context.Context, commitSHA string) error {
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, nil, nil)
	if err == nil {
		for _, en := range enrichments {
			_ = s.enrichmentStore.Delete(ctx, en)
		}
	}
	return s.associationStore.DeleteByEntityID(ctx, commitSHA)
}

// EnrichmentsForCommits returns enrichments for multiple commits with optional type filter.
// Results are aggregated across all commits and deduplicated.
func (s *EnrichmentQuery) EnrichmentsForCommits(
	ctx context.Context,
	commitSHAs []string,
	typ *enrichment.Type,
	limit int,
) ([]enrichment.Enrichment, error) {
	if len(commitSHAs) == 0 {
		return []enrichment.Enrichment{}, nil
	}

	if limit <= 0 {
		limit = 20 // default limit
	}

	seen := make(map[int64]struct{})
	var enrichments []enrichment.Enrichment

	for _, sha := range commitSHAs {
		if len(enrichments) >= limit {
			break
		}

		commitEnrichments, err := s.EnrichmentsForCommit(ctx, sha, typ, nil)
		if err != nil {
			continue
		}

		for _, e := range commitEnrichments {
			if len(enrichments) >= limit {
				break
			}
			if _, exists := seen[e.ID()]; !exists {
				seen[e.ID()] = struct{}{}
				enrichments = append(enrichments, e)
			}
		}
	}

	return enrichments, nil
}
