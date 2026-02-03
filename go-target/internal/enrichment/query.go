package enrichment

import (
	"context"
)

// QueryService provides queries for enrichments and their associations.
type QueryService struct {
	enrichmentRepo  EnrichmentRepository
	associationRepo AssociationRepository
}

// NewQueryService creates a new enrichment query service.
func NewQueryService(
	enrichmentRepo EnrichmentRepository,
	associationRepo AssociationRepository,
) *QueryService {
	return &QueryService{
		enrichmentRepo:  enrichmentRepo,
		associationRepo: associationRepo,
	}
}

// EnrichmentsForCommit returns all enrichments associated with a commit.
func (s *QueryService) EnrichmentsForCommit(
	ctx context.Context,
	commitSHA string,
	typ *Type,
	subtype *Subtype,
) ([]Enrichment, error) {
	associations, err := s.associationRepo.FindByEntityTypeAndID(ctx, EntityTypeCommit, commitSHA)
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

	var enrichments []Enrichment
	for _, id := range enrichmentIDs {
		e, err := s.enrichmentRepo.Get(ctx, id)
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
func (s *QueryService) HasSummariesForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := TypeDevelopment
	sub := SubtypeSnippetSummary
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasArchitectureForCommit checks if a commit has architecture enrichments.
func (s *QueryService) HasArchitectureForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := TypeArchitecture
	sub := SubtypePhysical
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasAPIDocsForCommit checks if a commit has API documentation enrichments.
func (s *QueryService) HasAPIDocsForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := TypeUsage
	sub := SubtypeAPIDocs
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasCommitDescriptionForCommit checks if a commit has commit description enrichments.
func (s *QueryService) HasCommitDescriptionForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := TypeHistory
	sub := SubtypeCommitDescription
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasDatabaseSchemaForCommit checks if a commit has database schema enrichments.
func (s *QueryService) HasDatabaseSchemaForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := TypeArchitecture
	sub := SubtypeDatabaseSchema
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasCookbookForCommit checks if a commit has cookbook enrichments.
func (s *QueryService) HasCookbookForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := TypeUsage
	sub := SubtypeCookbook
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasExamplesForCommit checks if a commit has example enrichments.
func (s *QueryService) HasExamplesForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := TypeDevelopment
	sub := SubtypeExample
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// HasExampleSummariesForCommit checks if a commit has example summary enrichments.
func (s *QueryService) HasExampleSummariesForCommit(ctx context.Context, commitSHA string) (bool, error) {
	typ := TypeDevelopment
	sub := SubtypeExampleSummary
	enrichments, err := s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// SnippetsForCommit returns all snippet enrichments for a commit.
func (s *QueryService) SnippetsForCommit(ctx context.Context, commitSHA string) ([]Enrichment, error) {
	typ := TypeDevelopment
	sub := SubtypeSnippet
	return s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
}

// ExamplesForCommit returns all example enrichments for a commit.
func (s *QueryService) ExamplesForCommit(ctx context.Context, commitSHA string) ([]Enrichment, error) {
	typ := TypeDevelopment
	sub := SubtypeExample
	return s.EnrichmentsForCommit(ctx, commitSHA, &typ, &sub)
}

// EnrichmentsForCommits returns enrichments for multiple commits with optional type filter.
// Results are aggregated across all commits and deduplicated.
func (s *QueryService) EnrichmentsForCommits(
	ctx context.Context,
	commitSHAs []string,
	typ *Type,
	limit int,
) ([]Enrichment, error) {
	if len(commitSHAs) == 0 {
		return []Enrichment{}, nil
	}

	if limit <= 0 {
		limit = 20 // default limit
	}

	seen := make(map[int64]struct{})
	var enrichments []Enrichment

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
