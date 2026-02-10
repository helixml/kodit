// Package persistence provides database storage implementations.
package persistence

import (
	"github.com/helixml/kodit/internal/database"
)

// AutoMigrate runs GORM auto migration for all models.
func AutoMigrate(db database.Database) error {
	return db.GORM().AutoMigrate(
		&RepositoryModel{},
		&CommitModel{},
		&BranchModel{},
		&TagModel{},
		&FileModel{},
		&SnippetModel{},
		&SnippetCommitAssociationModel{},
		&SnippetFileDerivationModel{},
		&CommitIndexModel{},
		&EnrichmentModel{},
		&EnrichmentAssociationModel{},
		&EmbeddingModel{},
		&TaskModel{},
		&TaskStatusModel{},
	)
}
