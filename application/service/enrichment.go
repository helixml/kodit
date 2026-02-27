package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
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

// Enrichment provides queries for enrichments and their associations.
// Embeds Collection for Find/Get/Count; bespoke methods handle complex queries.
type Enrichment struct {
	repository.Collection[enrichment.Enrichment]
	enrichmentStore    enrichment.EnrichmentStore
	associationStore   enrichment.AssociationStore
	bm25Store          search.BM25Store
	codeEmbeddingStore search.EmbeddingStore
	textEmbeddingStore search.EmbeddingStore
	lineRangeStore     chunk.LineRangeStore
}

// NewEnrichment creates a new Enrichment service.
func NewEnrichment(
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	bm25Store search.BM25Store,
	codeEmbeddingStore search.EmbeddingStore,
	textEmbeddingStore search.EmbeddingStore,
	lineRangeStore chunk.LineRangeStore,
) *Enrichment {
	return &Enrichment{
		Collection:         repository.NewCollection[enrichment.Enrichment](enrichmentStore),
		enrichmentStore:    enrichmentStore,
		associationStore:   associationStore,
		bm25Store:          bm25Store,
		codeEmbeddingStore: codeEmbeddingStore,
		textEmbeddingStore: textEmbeddingStore,
		lineRangeStore:     lineRangeStore,
	}
}

// List returns enrichments matching the given params.
// Commit SHA filtering is handled via enrichment.WithCommitSHA / WithCommitSHAs
// options, which the store resolves to association JOINs transparently.
func (s *Enrichment) List(ctx context.Context, params *EnrichmentListParams) ([]enrichment.Enrichment, error) {
	if params == nil {
		return []enrichment.Enrichment{}, nil
	}

	opts := s.filterOptions(params)
	opts = append(opts, s.commitOptions(params)...)
	opts = append(opts, s.paginationOptions(params)...)
	return s.enrichmentStore.Find(ctx, opts...)
}

// Count returns the total count of enrichments matching the given params (without pagination).
func (s *Enrichment) Count(ctx context.Context, params *EnrichmentListParams) (int64, error) {
	if params == nil {
		return 0, nil
	}

	opts := s.filterOptions(params)
	opts = append(opts, s.commitOptions(params)...)
	return s.enrichmentStore.Count(ctx, opts...)
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

func (s *Enrichment) commitOptions(params *EnrichmentListParams) []repository.Option {
	if params.CommitSHA != "" {
		return []repository.Option{enrichment.WithCommitSHA(params.CommitSHA)}
	}
	if len(params.CommitSHAs) > 0 {
		return []repository.Option{enrichment.WithCommitSHAs(params.CommitSHAs)}
	}
	return nil
}

func (s *Enrichment) paginationOptions(params *EnrichmentListParams) []repository.Option {
	if params.Limit > 0 {
		return repository.WithPagination(params.Limit, params.Offset)
	}
	return nil
}

// Save persists an enrichment (create or update).
// Associations cascade-delete via GORM constraints when an enrichment is removed.
func (s *Enrichment) Save(ctx context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error) {
	return s.enrichmentStore.Save(ctx, e)
}

// DeleteBy removes enrichments matching the given options.
// Also cleans up associated search indexes (BM25, code embeddings, text embeddings).
// Associations cascade-delete via the GORM OnDelete:CASCADE constraint on EnrichmentAssociationModel.
func (s *Enrichment) DeleteBy(ctx context.Context, opts ...repository.Option) error {
	// Find enrichments to be deleted so we can clean up search indexes
	enrichments, err := s.enrichmentStore.Find(ctx, opts...)
	if err != nil {
		return fmt.Errorf("find enrichments for deletion: %w", err)
	}

	if len(enrichments) > 0 {
		snippetIDs := make([]string, len(enrichments))
		for i, e := range enrichments {
			snippetIDs[i] = strconv.FormatInt(e.ID(), 10)
		}
		searchOpts := []repository.Option{search.WithSnippetIDs(snippetIDs)}

		if s.bm25Store != nil {
			if err := s.bm25Store.DeleteBy(ctx, searchOpts...); err != nil {
				return fmt.Errorf("delete bm25 indexes: %w", err)
			}
		}
		if s.codeEmbeddingStore != nil {
			if err := s.codeEmbeddingStore.DeleteBy(ctx, searchOpts...); err != nil {
				return fmt.Errorf("delete code embeddings: %w", err)
			}
		}
		if s.textEmbeddingStore != nil {
			if err := s.textEmbeddingStore.DeleteBy(ctx, searchOpts...); err != nil {
				return fmt.Errorf("delete text embeddings: %w", err)
			}
		}
	}

	return s.enrichmentStore.DeleteBy(ctx, opts...)
}

// RelatedEnrichments returns enrichments that reference the given enrichment IDs
// through the association store (e.g., snippet_summary enrichments pointing to snippet enrichments).
// Returns a map of parent enrichment ID (as string) to its related enrichments.
func (s *Enrichment) RelatedEnrichments(ctx context.Context, enrichmentIDs []int64) (map[string][]enrichment.Enrichment, error) {
	if len(enrichmentIDs) == 0 {
		return map[string][]enrichment.Enrichment{}, nil
	}

	// Convert enrichment IDs to entity ID strings (associations store entity_id as string)
	entityIDs := make([]string, len(enrichmentIDs))
	for i, id := range enrichmentIDs {
		entityIDs[i] = strconv.FormatInt(id, 10)
	}

	// Find associations where entity_id is one of our enrichment IDs and entity_type is "snippets"
	associations, err := s.associationStore.Find(ctx,
		enrichment.WithEntityIDIn(entityIDs),
		enrichment.WithEntityType(enrichment.EntityTypeSnippet),
	)
	if err != nil {
		return nil, fmt.Errorf("find related associations: %w", err)
	}

	if len(associations) == 0 {
		return map[string][]enrichment.Enrichment{}, nil
	}

	// Group association enrichment IDs by entity ID, and collect all enrichment IDs to fetch
	entityToEnrichmentIDs := make(map[string][]int64)
	var allIDs []int64
	for _, a := range associations {
		entityToEnrichmentIDs[a.EntityID()] = append(entityToEnrichmentIDs[a.EntityID()], a.EnrichmentID())
		allIDs = append(allIDs, a.EnrichmentID())
	}

	// Fetch the actual enrichment objects
	related, err := s.enrichmentStore.Find(ctx, repository.WithIDIn(allIDs))
	if err != nil {
		return nil, fmt.Errorf("fetch related enrichments: %w", err)
	}

	// Index by ID for lookup
	byID := make(map[int64]enrichment.Enrichment, len(related))
	for _, e := range related {
		byID[e.ID()] = e
	}

	// Build the result map: parent entity ID -> related enrichments
	result := make(map[string][]enrichment.Enrichment, len(entityToEnrichmentIDs))
	for entityID, ids := range entityToEnrichmentIDs {
		for _, id := range ids {
			if e, ok := byID[id]; ok {
				result[entityID] = append(result[entityID], e)
			}
		}
	}

	return result, nil
}

// SourceFiles returns file IDs grouped by enrichment ID string.
// It queries associations where enrichment_id IN (ids) and entity_type = "git_commit_files".
func (s *Enrichment) SourceFiles(ctx context.Context, enrichmentIDs []int64) (map[string][]int64, error) {
	if len(enrichmentIDs) == 0 {
		return map[string][]int64{}, nil
	}

	associations, err := s.associationStore.Find(ctx,
		enrichment.WithEnrichmentIDIn(enrichmentIDs),
		enrichment.WithEntityType(enrichment.EntityTypeFile),
	)
	if err != nil {
		return nil, fmt.Errorf("find file associations: %w", err)
	}

	result := make(map[string][]int64)
	for _, a := range associations {
		key := strconv.FormatInt(a.EnrichmentID(), 10)
		fileID, err := strconv.ParseInt(a.EntityID(), 10, 64)
		if err != nil {
			continue
		}
		result[key] = append(result[key], fileID)
	}

	return result, nil
}

// LineRanges returns chunk line ranges keyed by enrichment ID string.
func (s *Enrichment) LineRanges(ctx context.Context, enrichmentIDs []int64) (map[string]chunk.LineRange, error) {
	if len(enrichmentIDs) == 0 {
		return map[string]chunk.LineRange{}, nil
	}

	ranges, err := s.lineRangeStore.Find(ctx, repository.WithConditionIn("enrichment_id", enrichmentIDs))
	if err != nil {
		return nil, fmt.Errorf("find line ranges: %w", err)
	}

	result := make(map[string]chunk.LineRange, len(ranges))
	for _, r := range ranges {
		result[strconv.FormatInt(r.EnrichmentID(), 10)] = r
	}

	return result, nil
}
