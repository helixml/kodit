// Package persistence provides database storage implementations.
package persistence

import (
	"log/slog"

	"github.com/helixml/kodit/internal/database"
)

// PreMigrate handles one-time schema conversions from the Python-era database.
// It converts PostgreSQL enum columns to text so GORM AutoMigrate can manage them.
// Safe to run repeatedly — it checks whether the enum types still exist before acting.
func PreMigrate(db database.Database) error {
	if !db.IsPostgres() {
		return nil
	}

	gdb := db.GORM()

	// Convert embeddings.type from embeddingtype enum to text.
	var enumExists bool
	err := gdb.Raw(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname = 'embeddingtype')`).Scan(&enumExists).Error
	if err != nil {
		return err
	}
	if enumExists {
		slog.Warn("one-time database migration: converting Python-era enum columns to text — please wait, do not interrupt")
		if err := gdb.Exec(`ALTER TABLE embeddings ALTER COLUMN type TYPE text USING type::text`).Error; err != nil {
			return err
		}
		if err := gdb.Exec(`UPDATE embeddings SET type = LOWER(type)`).Error; err != nil {
			return err
		}
		if err := gdb.Exec(`DROP TYPE IF EXISTS embeddingtype`).Error; err != nil {
			return err
		}
		slog.Info("one-time database migration complete")
	}

	return nil
}

// AutoMigrate runs GORM auto migration for all models.
func AutoMigrate(db database.Database) error {
	return db.GORM().AutoMigrate(
		&RepositoryModel{},
		&CommitModel{},
		&BranchModel{},
		&TagModel{},
		&FileModel{},
		&CommitIndexModel{},
		&EnrichmentModel{},
		&EnrichmentAssociationModel{},
		&EmbeddingModel{},
		&TaskModel{},
		&TaskStatusModel{},
	)
}
