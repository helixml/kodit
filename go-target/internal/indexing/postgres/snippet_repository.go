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
		query = query.Joins("INNER JOIN enrichment_associations ON enrichment_associations.entity_id = snippets.sha AND enrichment_associations.entity_type = 'snippet'").
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

// ByIDs returns snippets by their SHA identifiers.
func (r *SnippetRepository) ByIDs(ctx context.Context, ids []string) ([]indexing.Snippet, error) {
	if len(ids) == 0 {
		return []indexing.Snippet{}, nil
	}

	var entities []SnippetEntity
	err := r.db.WithContext(ctx).Where("sha IN ?", ids).Find(&entities).Error
	if err != nil {
		return nil, err
	}

	snippets := make([]indexing.Snippet, len(entities))
	for i, entity := range entities {
		snippets[i] = r.mapper.ToDomain(entity)
	}

	return snippets, nil
}
