package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
)

// SnippetStore implements snippet.SnippetStore using GORM.
type SnippetStore struct {
	db     database.Database
	mapper SnippetMapper
}

// NewSnippetStore creates a new SnippetStore.
func NewSnippetStore(db database.Database) SnippetStore {
	return SnippetStore{
		db:     db,
		mapper: SnippetMapper{},
	}
}

// Save persists snippets for a commit.
// This creates the snippets if they don't exist (content-addressed by SHA)
// and creates associations linking them to the commit.
func (s SnippetStore) Save(ctx context.Context, commitSHA string, snippets []snippet.Snippet) error {
	if len(snippets) == 0 {
		return nil
	}

	return s.db.GORM().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		for _, snip := range snippets {
			model := s.mapper.ToModel(snip)

			// Upsert snippet (content-addressed, so only insert if not exists)
			result := tx.Where("sha = ?", model.SHA).FirstOrCreate(&model)
			if result.Error != nil {
				return result.Error
			}

			// Create association between snippet and commit
			association := SnippetCommitAssociationModel{
				SnippetSHA: snip.SHA(),
				CommitSHA:  commitSHA,
				CreatedAt:  now,
			}

			// Check if association already exists
			var existing SnippetCommitAssociationModel
			err := tx.Where("snippet_sha = ? AND commit_sha = ?", snip.SHA(), commitSHA).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&association).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}

			// Create file derivations for the snippet's source files
			for _, file := range snip.DerivesFrom() {
				derivation := SnippetFileDerivationModel{
					SnippetSHA: snip.SHA(),
					FileID:     file.ID(),
					CreatedAt:  now,
				}

				// Check if derivation already exists
				var existingDerivation SnippetFileDerivationModel
				err := tx.Where("snippet_sha = ? AND file_id = ?", snip.SHA(), file.ID()).First(&existingDerivation).Error
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
func (s SnippetStore) SnippetsForCommit(ctx context.Context, commitSHA string) ([]snippet.Snippet, error) {
	var associations []SnippetCommitAssociationModel
	err := s.db.Session(ctx).Where("commit_sha = ?", commitSHA).Find(&associations).Error
	if err != nil {
		return nil, err
	}

	if len(associations) == 0 {
		return []snippet.Snippet{}, nil
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

	return s.ByIDs(ctx, shas)
}

// DeleteForCommit removes all snippet associations for a commit.
// Note: This only removes associations, not the snippets themselves
// (which may be associated with other commits).
func (s SnippetStore) DeleteForCommit(ctx context.Context, commitSHA string) error {
	return s.db.Session(ctx).Where("commit_sha = ?", commitSHA).Delete(&SnippetCommitAssociationModel{}).Error
}

// ByIDs returns snippets by their SHA identifiers.
func (s SnippetStore) ByIDs(ctx context.Context, ids []string) ([]snippet.Snippet, error) {
	if len(ids) == 0 {
		return []snippet.Snippet{}, nil
	}

	var models []SnippetModel
	err := s.db.Session(ctx).Where("sha IN ?", ids).Find(&models).Error
	if err != nil {
		return nil, err
	}

	// Load enrichments for all snippets in bulk
	enrichmentMap := s.loadEnrichmentsForSnippets(ctx, ids)

	snippets := make([]snippet.Snippet, len(models))
	for i, model := range models {
		// Load file derivations for this snippet
		var derivations []SnippetFileDerivationModel
		if err := s.db.Session(ctx).Where("snippet_sha = ?", model.SHA).Find(&derivations).Error; err == nil {
			derivesFrom := make([]repository.File, 0, len(derivations))
			for _, d := range derivations {
				var fileModel FileModel
				if err := s.db.Session(ctx).Where("id = ?", d.FileID).First(&fileModel).Error; err == nil {
					derivesFrom = append(derivesFrom, FileMapper{}.ToDomain(fileModel))
				}
			}
			snippets[i] = snippet.ReconstructSnippet(
				model.SHA,
				model.Content,
				model.Extension,
				derivesFrom,
				enrichmentMap[model.SHA],
				model.CreatedAt,
				model.UpdatedAt,
			)
		} else {
			snippets[i] = s.mapper.ToDomain(model)
		}
	}

	return snippets, nil
}

// loadEnrichmentsForSnippets loads enrichments for multiple snippets in bulk.
func (s SnippetStore) loadEnrichmentsForSnippets(ctx context.Context, snippetSHAs []string) map[string][]snippet.Enrichment {
	result := make(map[string][]snippet.Enrichment)
	if len(snippetSHAs) == 0 {
		return result
	}

	// Query enrichments via association table
	type enrichmentResult struct {
		SnippetSHA string
		Type       string
		Subtype    string
		Content    string
	}

	var enrichments []enrichmentResult
	err := s.db.Session(ctx).
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
		Scan(&enrichments).Error
	if err != nil {
		return result
	}

	// Group by snippet SHA - use subtype as the enrichment type for display
	for _, e := range enrichments {
		result[e.SnippetSHA] = append(result[e.SnippetSHA], snippet.NewEnrichment(e.Subtype, e.Content))
	}

	return result
}

// BySHA returns a single snippet by its SHA identifier.
func (s SnippetStore) BySHA(ctx context.Context, sha string) (snippet.Snippet, error) {
	var model SnippetModel
	err := s.db.Session(ctx).Where("sha = ?", sha).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return snippet.Snippet{}, fmt.Errorf("%w: snippet %s", database.ErrNotFound, sha)
		}
		return snippet.Snippet{}, err
	}

	return s.mapper.ToDomain(model), nil
}

// CommitIndexStore implements snippet.CommitIndexStore using GORM.
type CommitIndexStore struct {
	db     database.Database
	mapper CommitIndexMapper
}

// NewCommitIndexStore creates a new CommitIndexStore.
func NewCommitIndexStore(db database.Database) CommitIndexStore {
	return CommitIndexStore{
		db:     db,
		mapper: CommitIndexMapper{},
	}
}

// Get returns a commit index by SHA.
func (s CommitIndexStore) Get(ctx context.Context, commitSHA string) (snippet.CommitIndex, error) {
	var model CommitIndexModel
	err := s.db.Session(ctx).Where("commit_sha = ?", commitSHA).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return snippet.CommitIndex{}, fmt.Errorf("%w: commit index %s", database.ErrNotFound, commitSHA)
		}
		return snippet.CommitIndex{}, err
	}
	return s.mapper.ToDomain(model), nil
}

// Save persists a commit index.
func (s CommitIndexStore) Save(ctx context.Context, index snippet.CommitIndex) error {
	model := s.mapper.ToModel(index)
	return s.db.Session(ctx).Save(&model).Error
}

// Delete removes a commit index.
func (s CommitIndexStore) Delete(ctx context.Context, commitSHA string) error {
	return s.db.Session(ctx).Where("commit_sha = ?", commitSHA).Delete(&CommitIndexModel{}).Error
}

// Exists checks if a commit index exists.
func (s CommitIndexStore) Exists(ctx context.Context, commitSHA string) (bool, error) {
	var count int64
	err := s.db.Session(ctx).Model(&CommitIndexModel{}).Where("commit_sha = ?", commitSHA).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
