package service

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/sourcelocation"
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
	enrichmentStore      enrichment.EnrichmentStore
	associationStore     enrichment.AssociationStore
	bm25Store            search.Store
	codeEmbeddingStore   search.Store
	textEmbeddingStore   search.Store
	visionEmbeddingStore search.Store
	lineRangeStore       sourcelocation.Store
}

// NewEnrichment creates a new Enrichment service.
func NewEnrichment(
	enrichmentStore enrichment.EnrichmentStore,
	associationStore enrichment.AssociationStore,
	bm25Store search.Store,
	codeEmbeddingStore search.Store,
	textEmbeddingStore search.Store,
	visionEmbeddingStore search.Store,
	lineRangeStore sourcelocation.Store,
) *Enrichment {
	return &Enrichment{
		Collection:           repository.NewCollection[enrichment.Enrichment](enrichmentStore),
		enrichmentStore:      enrichmentStore,
		associationStore:     associationStore,
		bm25Store:            bm25Store,
		codeEmbeddingStore:   codeEmbeddingStore,
		textEmbeddingStore:   textEmbeddingStore,
		visionEmbeddingStore: visionEmbeddingStore,
		lineRangeStore:       lineRangeStore,
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
		if s.visionEmbeddingStore != nil {
			if err := s.visionEmbeddingStore.DeleteBy(ctx, searchOpts...); err != nil {
				return fmt.Errorf("delete vision embeddings: %w", err)
			}
		}
	}

	return s.enrichmentStore.DeleteBy(ctx, opts...)
}

// RepositoryEnrichmentIDs returns the IDs of the enrichments directly
// associated with repoID.
func (s *Enrichment) RepositoryEnrichmentIDs(ctx context.Context, repoID int64) ([]int64, error) {
	associations, err := s.associationStore.Find(ctx,
		enrichment.WithEntityType(enrichment.EntityTypeRepository),
		enrichment.WithEntityID(strconv.FormatInt(repoID, 10)),
	)
	if err != nil {
		return nil, fmt.Errorf("find repository associations: %w", err)
	}
	ids := make([]int64, 0, len(associations))
	for _, a := range associations {
		ids = append(ids, a.EnrichmentID())
	}
	return ids, nil
}

// ReleaseForRepository removes repoID's claim on the given enrichments and
// returns the IDs it deleted. An enrichment that another repository also owns
// through its own repository association keeps existing and loses only
// repoID's association; every other enrichment is deleted with DeleteBy.
func (s *Enrichment) ReleaseForRepository(ctx context.Context, enrichmentIDs []int64, repoID int64) ([]int64, error) {
	if len(enrichmentIDs) == 0 {
		return nil, nil
	}
	owners, err := s.RepositoryIDs(ctx, enrichmentIDs)
	if err != nil {
		return nil, err
	}

	var shared, deleted []int64
	for _, id := range enrichmentIDs {
		others := slices.ContainsFunc(owners[strconv.FormatInt(id, 10)], func(owner int64) bool { return owner != repoID })
		if others {
			shared = append(shared, id)
		} else {
			deleted = append(deleted, id)
		}
	}

	if len(shared) > 0 {
		if err := s.associationStore.DeleteBy(ctx,
			enrichment.WithEnrichmentIDIn(shared),
			enrichment.WithEntityType(enrichment.EntityTypeRepository),
			enrichment.WithEntityID(strconv.FormatInt(repoID, 10)),
		); err != nil {
			return nil, fmt.Errorf("delete repository associations: %w", err)
		}
	}
	if len(deleted) > 0 {
		if err := s.DeleteBy(ctx, repository.WithIDIn(deleted)); err != nil {
			return nil, err
		}
	}
	return deleted, nil
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

// SourceLocations returns source locations keyed by enrichment ID string.
func (s *Enrichment) SourceLocations(ctx context.Context, enrichmentIDs []int64) (map[string]sourcelocation.SourceLocation, error) {
	if len(enrichmentIDs) == 0 {
		return map[string]sourcelocation.SourceLocation{}, nil
	}

	ranges, err := s.lineRangeStore.Find(ctx, repository.WithConditionIn("enrichment_id", enrichmentIDs))
	if err != nil {
		return nil, fmt.Errorf("find line ranges: %w", err)
	}

	result := make(map[string]sourcelocation.SourceLocation, len(ranges))
	for _, r := range ranges {
		result[strconv.FormatInt(r.EnrichmentID(), 10)] = r
	}

	return result, nil
}

// RepositoryIDs returns, keyed by enrichment ID, the repositories each
// enrichment belongs to, in ascending order. Membership comes only from the
// enrichment's direct repository associations, which indexing writes for the
// repository that produced the enrichment and for each repository that shares
// the commit. The search stores apply the same rule when filtering by source
// repository (infrastructure/persistence/search_filter_options.go).
func (s *Enrichment) RepositoryIDs(ctx context.Context, enrichmentIDs []int64) (map[string][]int64, error) {
	if len(enrichmentIDs) == 0 {
		return map[string][]int64{}, nil
	}

	associations, err := s.associationStore.Find(ctx,
		enrichment.WithEnrichmentIDIn(enrichmentIDs),
		enrichment.WithEntityType(enrichment.EntityTypeRepository),
	)
	if err != nil {
		return nil, fmt.Errorf("find repository associations: %w", err)
	}

	result := make(map[string][]int64)
	for _, a := range associations {
		repoID, err := strconv.ParseInt(a.EntityID(), 10, 64)
		if err != nil {
			continue
		}
		key := strconv.FormatInt(a.EnrichmentID(), 10)
		if !slices.Contains(result[key], repoID) {
			result[key] = append(result[key], repoID)
		}
	}
	for _, ids := range result {
		slices.Sort(ids)
	}

	return result, nil
}
