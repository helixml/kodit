package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"gorm.io/gorm"
)

// ErrSnippetNotFound indicates a snippet was not found.
var ErrSnippetNotFound = errors.New("snippet not found")

// SnippetRepository implements indexing.SnippetRepository using GORM.
type SnippetRepository struct {
	db     *gorm.DB
	mapper SnippetMapper
}

// NewSnippetRepository creates a new SnippetRepository.
func NewSnippetRepository(db *gorm.DB) *SnippetRepository {
	return &SnippetRepository{
		db:     db,
		mapper: SnippetMapper{},
	}
}

// Save persists snippets for a commit.
// This creates the snippets if they don't exist (content-addressed by SHA)
// and creates associations linking them to the commit.
func (r *SnippetRepository) Save(ctx context.Context, commitSHA string, snippets []indexing.Snippet) error {
	if len(snippets) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		for _, snippet := range snippets {
			entity := r.mapper.ToEntity(snippet)

			// Upsert snippet (content-addressed, so only insert if not exists)
			result := tx.Where("sha = ?", entity.SHA).FirstOrCreate(&entity)
			if result.Error != nil {
				return result.Error
			}

			// Create association between snippet and commit
			association := SnippetCommitAssociationEntity{
				SnippetSHA: snippet.SHA(),
				CommitSHA:  commitSHA,
				CreatedAt:  now,
			}

			// Check if association already exists
			var existing SnippetCommitAssociationEntity
			err := tx.Where("snippet_sha = ? AND commit_sha = ?", snippet.SHA(), commitSHA).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&association).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}

			// Create file derivations for the snippet's source files
			for _, file := range snippet.DerivesFrom() {
				derivation := SnippetFileDerivationEntity{
					SnippetSHA: snippet.SHA(),
					FileID:     file.ID(),
					CreatedAt:  now,
				}

				// Check if derivation already exists
				var existingDerivation SnippetFileDerivationEntity
				err := tx.Where("snippet_sha = ? AND file_id = ?", snippet.SHA(), file.ID()).First(&existingDerivation).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					if err := tx.Create(&derivation).Error; err != nil {
						return err
					}
				} else if err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// SnippetsForCommit returns all snippets for a specific commit.
func (r *SnippetRepository) SnippetsForCommit(ctx context.Context, commitSHA string) ([]indexing.Snippet, error) {
	var associations []SnippetCommitAssociationEntity
	err := r.db.WithContext(ctx).Where("commit_sha = ?", commitSHA).Find(&associations).Error
	if err != nil {
		return nil, err
	}

	if len(associations) == 0 {
		return []indexing.Snippet{}, nil
	}

	// Get unique snippet SHAs
	shaSet := make(map[string]struct{})
	for _, assoc := range associations {
		shaSet[assoc.SnippetSHA] = struct{}{}
	}

	shas := make([]string, 0, len(shaSet))
	for sha := range shaSet {
		shas = append(shas, sha)
	}

	return r.ByIDs(ctx, shas)
}

// DeleteForCommit removes all snippet associations for a commit.
// Note: This only removes associations, not the snippets themselves
// (which may be associated with other commits).
func (r *SnippetRepository) DeleteForCommit(ctx context.Context, commitSHA string) error {
	return r.db.WithContext(ctx).Where("commit_sha = ?", commitSHA).Delete(&SnippetCommitAssociationEntity{}).Error
}

// Search finds snippets matching the search request.
func (r *SnippetRepository) Search(ctx context.Context, request domain.MultiSearchRequest) ([]indexing.Snippet, error) {
	query := r.db.WithContext(ctx).Model(&SnippetEntity{})

	filters := request.Filters()

	// Apply commit SHA filter via association table
	if len(filters.CommitSHAs()) > 0 {
		query = query.Joins("INNER JOIN snippet_commit_associations ON snippet_commit_associations.snippet_sha = snippets.sha").
			Where("snippet_commit_associations.commit_sha IN ?", filters.CommitSHAs())
	}

	// Apply extension/language filter
	if filters.Language() != "" {
		mapping := domain.LanguageMapping{}
		extensions := mapping.ExtensionsWithFallback(filters.Language())
		query = query.Where("extension IN ?", extensions)
	}

	// Apply file path filter via file derivations
	if filters.FilePath() != "" {
		query = query.Joins("INNER JOIN snippet_file_derivations ON snippet_file_derivations.snippet_sha = snippets.sha").
			Joins("INNER JOIN git_commit_files ON git_commit_files.id = snippet_file_derivations.file_id").
			Where("git_commit_files.path LIKE ?", "%"+filters.FilePath()+"%")
	}

	// Apply created time filters
	if !filters.CreatedAfter().IsZero() {
		query = query.Where("snippets.created_at >= ?", filters.CreatedAfter())
	}
	if !filters.CreatedBefore().IsZero() {
		query = query.Where("snippets.created_at <= ?", filters.CreatedBefore())
	}

	// Apply enrichment type filter via enrichment associations
	if len(filters.EnrichmentTypes()) > 0 || len(filters.EnrichmentSubtypes()) > 0 {
		query = query.Joins("INNER JOIN enrichment_associations ON enrichment_associations.entity_id = snippets.sha AND enrichment_associations.entity_type = 'snippets'").
			Joins("INNER JOIN enrichments_v2 ON enrichments_v2.id = enrichment_associations.enrichment_id")

		if len(filters.EnrichmentTypes()) > 0 {
			query = query.Where("enrichments_v2.type IN ?", filters.EnrichmentTypes())
		}
		if len(filters.EnrichmentSubtypes()) > 0 {
			query = query.Where("enrichments_v2.subtype IN ?", filters.EnrichmentSubtypes())
		}
	}

	// Apply limit
	if request.TopK() > 0 {
		query = query.Limit(request.TopK())
	}

	// Select distinct to avoid duplicates from joins
	query = query.Distinct()

	var entities []SnippetEntity
	if err := query.Find(&entities).Error; err != nil {
		return nil, err
	}

	snippets := make([]indexing.Snippet, len(entities))
	for i, entity := range entities {
		snippets[i] = r.mapper.ToDomain(entity)
	}

	return snippets, nil
}

// ByIDs returns snippets by their SHA identifiers with their file derivations and enrichments.
func (r *SnippetRepository) ByIDs(ctx context.Context, ids []string) ([]indexing.Snippet, error) {
	if len(ids) == 0 {
		return []indexing.Snippet{}, nil
	}

	var entities []SnippetEntity
	err := r.db.WithContext(ctx).Where("sha IN ?", ids).Find(&entities).Error
	if err != nil {
		return nil, err
	}

	// Load file derivations for all snippets
	derivations, err := r.loadFileDerivations(ctx, ids)
	if err != nil {
		return nil, err
	}

	// Load enrichments for all snippets
	enrichments, err := r.loadEnrichments(ctx, ids)
	if err != nil {
		return nil, err
	}

	snippets := make([]indexing.Snippet, len(entities))
	for i, entity := range entities {
		files := derivations[entity.SHA]
		enrich := enrichments[entity.SHA]
		snippets[i] = r.mapper.ToDomainWithRelations(entity, files, enrich)
	}

	return snippets, nil
}

// BySHA returns a single snippet by its SHA identifier.
func (r *SnippetRepository) BySHA(ctx context.Context, sha string) (indexing.Snippet, error) {
	var entity SnippetEntity
	err := r.db.WithContext(ctx).Where("sha = ?", sha).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return indexing.Snippet{}, ErrSnippetNotFound
		}
		return indexing.Snippet{}, err
	}

	return r.mapper.ToDomain(entity), nil
}

// FileDerivationWithFile represents a file derivation joined with its file data.
type FileDerivationWithFile struct {
	SnippetSHA string
	FileID     int64
	CommitSHA  string
	Path       string
	BlobSHA    string
	MimeType   string
	Extension  string
	Size       int64
	CreatedAt  time.Time
}

// loadFileDerivations loads file derivations for snippets, returning a map of snippet SHA to files.
func (r *SnippetRepository) loadFileDerivations(ctx context.Context, snippetSHAs []string) (map[string][]FileDerivationWithFile, error) {
	if len(snippetSHAs) == 0 {
		return map[string][]FileDerivationWithFile{}, nil
	}

	// Query file derivations with joined file data
	var results []FileDerivationWithFile
	err := r.db.WithContext(ctx).
		Table("snippet_file_derivations").
		Select(`
			snippet_file_derivations.snippet_sha,
			snippet_file_derivations.file_id,
			git_commit_files.commit_sha,
			git_commit_files.path,
			git_commit_files.blob_sha,
			git_commit_files.mime_type,
			git_commit_files.extension,
			git_commit_files.size,
			git_commit_files.created_at
		`).
		Joins("INNER JOIN git_commit_files ON git_commit_files.id = snippet_file_derivations.file_id").
		Where("snippet_file_derivations.snippet_sha IN ?", snippetSHAs).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// Group by snippet SHA
	derivationMap := make(map[string][]FileDerivationWithFile)
	for _, result := range results {
		derivationMap[result.SnippetSHA] = append(derivationMap[result.SnippetSHA], result)
	}

	return derivationMap, nil
}

// EnrichmentWithData represents an enrichment joined with its data.
type EnrichmentWithData struct {
	SnippetSHA string
	Type       string
	Subtype    string
	Content    string
}

// loadEnrichments loads enrichments for snippets, returning a map of snippet SHA to enrichments.
func (r *SnippetRepository) loadEnrichments(ctx context.Context, snippetSHAs []string) (map[string][]EnrichmentWithData, error) {
	if len(snippetSHAs) == 0 {
		return map[string][]EnrichmentWithData{}, nil
	}

	// Query enrichments via association table
	var results []EnrichmentWithData
	err := r.db.WithContext(ctx).
		Table("enrichment_associations").
		Select(`
			enrichment_associations.entity_id as snippet_sha,
			enrichments_v2.type,
			enrichments_v2.subtype,
			enrichments_v2.content
		`).
		Joins("INNER JOIN enrichments_v2 ON enrichments_v2.id = enrichment_associations.enrichment_id").
		Where("enrichment_associations.entity_type = ?", "snippets").
		Where("enrichment_associations.entity_id IN ?", snippetSHAs).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// Group by snippet SHA
	enrichmentMap := make(map[string][]EnrichmentWithData)
	for _, result := range results {
		enrichmentMap[result.SnippetSHA] = append(enrichmentMap[result.SnippetSHA], result)
	}

	return enrichmentMap, nil
}
