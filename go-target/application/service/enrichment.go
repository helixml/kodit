package service

import (
	"context"
	"fmt"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
)

// EnrichmentListParams configures enrichment listing.
type EnrichmentListParams struct {
	Type       *enrichment.Type
	Subtype    *enrichment.Subtype
	CommitSHA  string
	CommitSHAs []string
	Limit      int
	Offset     int
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
// Embeds Collection for Find/Get; bespoke methods handle writes and complex queries.
type Enrichment struct {
	repository.Collection[enrichment.Enrichment]
	enrichmentStore  enrichment.EnrichmentStore
	associationStore enrichment.AssociationStore
}

// NewEnrichment creates a new Enrichment service.
func NewEnrichment(
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
) *Enrichment {
	return &Enrichment{
		Collection:       repository.NewCollection[enrichment.Enrichment](enrichmentStore),
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

	paginationOpts := s.paginationOptions(params)

	if params.CommitSHA != "" {
		opts := append(s.filterOptions(params), paginationOpts...)
		return s.enrichmentStore.FindByCommitSHA(ctx, params.CommitSHA, opts...)
	}

	if len(params.CommitSHAs) > 0 {
		opts := append(s.filterOptions(params), paginationOpts...)
		return s.enrichmentStore.FindByCommitSHAs(ctx, params.CommitSHAs, opts...)
	}

	opts := append(s.filterOptions(params), paginationOpts...)
	return s.enrichmentStore.Find(ctx, opts...)
}

// Count returns the total count of enrichments matching the given params (without pagination).
func (s *Enrichment) Count(ctx context.Context, params *EnrichmentListParams) (int64, error) {
	if params == nil {
		return 0, nil
	}

	filterOpts := s.filterOptions(params)

	if params.CommitSHA != "" {
		return s.enrichmentStore.CountByCommitSHA(ctx, params.CommitSHA, filterOpts...)
	}

	if len(params.CommitSHAs) > 0 {
		return s.enrichmentStore.CountByCommitSHAs(ctx, params.CommitSHAs, filterOpts...)
	}

	return s.enrichmentStore.Count(ctx, filterOpts...)
}

func (s *Enrichment) filterOptions(params *EnrichmentListParams) []repository.Option {
	var opts []repository.Option
	if params.Type != nil {
		opts = append(opts, enrichment.WithType(*params.Type))
	}
	if params.Subtype != nil {
		opts = append(opts, enrichment.WithSubtype(*params.Subtype))
	}
	return opts
}

func (s *Enrichment) paginationOptions(params *EnrichmentListParams) []repository.Option {
	if params.Limit > 0 {
		return repository.WithPagination(params.Limit, params.Offset)
	}
	return nil
}

// Update replaces the content of an enrichment and returns the saved result.
func (s *Enrichment) Update(ctx context.Context, id int64, params *EnrichmentUpdateParams) (enrichment.Enrichment, error) {
	existing, err := s.enrichmentStore.FindOne(ctx, repository.WithID(id))
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

// Delete removes enrichments and their associations.
// When ID is set, deletes a single enrichment.
// When CommitSHA is set, batch-deletes all enrichments for that commit.
func (s *Enrichment) Delete(ctx context.Context, params *EnrichmentDeleteParams) error {
	if params.ID != nil {
		_ = s.associationStore.DeleteBy(ctx, enrichment.WithEnrichmentID(*params.ID))
		return s.enrichmentStore.DeleteBy(ctx, repository.WithID(*params.ID))
	}

	if params.CommitSHA != "" {
		associations, err := s.associationStore.Find(ctx, enrichment.WithEntityType(enrichment.EntityTypeCommit), enrichment.WithEntityID(params.CommitSHA))
		if err != nil {
			return fmt.Errorf("find associations for commit: %w", err)
		}

		if len(associations) > 0 {
			ids := make([]int64, len(associations))
			for i, a := range associations {
				ids[i] = a.EnrichmentID()
			}
			if err := s.enrichmentStore.DeleteBy(ctx, repository.WithIDIn(ids)); err != nil {
				return fmt.Errorf("delete enrichments: %w", err)
			}
		}

		return s.associationStore.DeleteBy(ctx, enrichment.WithEntityID(params.CommitSHA))
	}

	return nil
}

