package service

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/enrichment"
)

// EnrichmentListParams configures enrichment listing.
type EnrichmentListParams struct {
	Type       *enrichment.Type
	Subtype    *enrichment.Subtype
	CommitSHA  string
	CommitSHAs []string
	Limit      int
}

// EnrichmentExistsParams specifies which enrichments to check for existence.
type EnrichmentExistsParams struct {
	CommitSHA string
	Type      enrichment.Type
	Subtype   enrichment.Subtype
}

// EnrichmentUpdateParams configures enrichment content updates.
type EnrichmentUpdateParams struct {
	Content string
}

// EnrichmentDeleteParams configures enrichment deletion.
// Set ID to delete a single enrichment, or CommitSHA to delete all enrichments for a commit.
type EnrichmentDeleteParams struct {
	ID        *int64
	CommitSHA string
}

// Enrichment provides queries for enrichments and their associations.
type Enrichment struct {
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
}

// NewEnrichment creates a new Enrichment service.
func NewEnrichment(
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
) *Enrichment {
	return &Enrichment{
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
	}
}

// List returns enrichments matching the given params.
// If CommitSHA is set, returns enrichments for that commit.
// If CommitSHAs is set, returns enrichments across multiple commits.
// Otherwise returns enrichments matching the type/subtype filter.
func (s *Enrichment) List(ctx context.Context, params *EnrichmentListParams) ([]enrichment.Enrichment, error) {
	if params == nil {
		return []enrichment.Enrichment{}, nil
	}

	if params.CommitSHA != "" {
		associations, err := s.associationStore.FindByEntityTypeAndID(ctx, enrichment.EntityTypeCommit, params.CommitSHA)
		if err != nil {
			return nil, err
		}
		if len(associations) == 0 {
			return []enrichment.Enrichment{}, nil
		}

		var enrichments []enrichment.Enrichment
		for _, a := range associations {
			e, err := s.enrichmentStore.Get(ctx, a.EnrichmentID())
			if err != nil {
				continue
			}
			if params.Type != nil && e.Type() != *params.Type {
				continue
			}
			if params.Subtype != nil && e.Subtype() != *params.Subtype {
				continue
			}
			enrichments = append(enrichments, e)
		}
		return enrichments, nil
	}

	if len(params.CommitSHAs) > 0 {
		return s.listForCommits(ctx, params.CommitSHAs, params.Type, params.Limit)
	}

	if params.Type != nil && params.Subtype != nil {
		return s.enrichmentStore.FindByTypeAndSubtype(ctx, *params.Type, *params.Subtype)
	}
	if params.Type != nil {
		return s.enrichmentStore.FindByType(ctx, *params.Type)
	}
	return []enrichment.Enrichment{}, nil
}

// Update replaces the content of an enrichment and returns the saved result.
func (s *Enrichment) Update(ctx context.Context, id int64, params *EnrichmentUpdateParams) (enrichment.Enrichment, error) {
	existing, err := s.enrichmentStore.Get(ctx, id)
	if err != nil {
		return enrichment.Enrichment{}, err
	}
	updated := existing.WithContent(params.Content)
	return s.enrichmentStore.Save(ctx, updated)
}

// Exists checks whether any enrichments match the given params.
func (s *Enrichment) Exists(ctx context.Context, params *EnrichmentExistsParams) (bool, error) {
	typ := params.Type
	sub := params.Subtype
	enrichments, err := s.List(ctx, &EnrichmentListParams{
		CommitSHA: params.CommitSHA,
		Type:      &typ,
		Subtype:   &sub,
	})
	if err != nil {
		return false, err
	}
	return len(enrichments) > 0, nil
}

// Get retrieves a single enrichment by ID.
func (s *Enrichment) Get(ctx context.Context, id int64) (enrichment.Enrichment, error) {
	return s.enrichmentStore.Get(ctx, id)
}

// Delete removes enrichments and their associations.
// When ID is set, deletes a single enrichment.
// When CommitSHA is set, batch-deletes all enrichments for that commit.
func (s *Enrichment) Delete(ctx context.Context, params *EnrichmentDeleteParams) error {
	if params.ID != nil {
		_ = s.associationStore.DeleteByEnrichmentID(ctx, *params.ID)
		return s.enrichmentStore.DeleteByIDs(ctx, []int64{*params.ID})
	}

	if params.CommitSHA != "" {
		associations, err := s.associationStore.FindByEntityTypeAndID(ctx, enrichment.EntityTypeCommit, params.CommitSHA)
		if err != nil {
			return fmt.Errorf("find associations for commit: %w", err)
		}

		if len(associations) > 0 {
			ids := make([]int64, len(associations))
			for i, a := range associations {
				ids[i] = a.EnrichmentID()
			}
			if err := s.enrichmentStore.DeleteByIDs(ctx, ids); err != nil {
				return fmt.Errorf("delete enrichments: %w", err)
			}
		}

		return s.associationStore.DeleteByEntityID(ctx, params.CommitSHA)
	}

	return nil
}

func (s *Enrichment) listForCommits(
	ctx context.Context,
	commitSHAs []string,
	typ *enrichment.Type,
	limit int,
) ([]enrichment.Enrichment, error) {
	if len(commitSHAs) == 0 {
		return []enrichment.Enrichment{}, nil
	}

	if limit <= 0 {
		limit = 20
	}

	seen := make(map[int64]struct{})
	var enrichments []enrichment.Enrichment

	for _, sha := range commitSHAs {
		if len(enrichments) >= limit {
			break
		}

		batch, err := s.List(ctx, &EnrichmentListParams{CommitSHA: sha, Type: typ})
		if err != nil {
			continue
		}

		for _, e := range batch {
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
