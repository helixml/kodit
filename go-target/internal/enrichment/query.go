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

func (s *QueryService) enrichmentsForCommit(
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

// ListParams configures enrichment listing.
type ListParams struct {
	CommitSHA string
	Type      *Type
	Subtype   *Subtype
}

// List returns enrichments matching the given params.
func (s *QueryService) List(ctx context.Context, params *ListParams) ([]Enrichment, error) {
	if params == nil {
		return []Enrichment{}, nil
	}
	return s.enrichmentsForCommit(ctx, params.CommitSHA, params.Type, params.Subtype)
}

// ExistsParams specifies which enrichments to check for existence.
type ExistsParams struct {
	CommitSHA string
	Type      Type
	Subtype   Subtype
}

// Exists checks whether any enrichments match the given params.
func (s *QueryService) Exists(ctx context.Context, params *ExistsParams) (bool, error) {
	typ := params.Type
	sub := params.Subtype
	enrichments, err := s.enrichmentsForCommit(ctx, params.CommitSHA, &typ, &sub)
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
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

		commitEnrichments, err := s.enrichmentsForCommit(ctx, sha, typ, nil)
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
