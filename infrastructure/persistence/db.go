// Package persistence provides database storage implementations.
package persistence

import (
	"fmt"
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
	if err := db.GORM().AutoMigrate(
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
	); err != nil {
		return err
	}
	return postMigrate(db)
}

// postMigrate creates FK constraints that GORM cannot manage correctly.
//
// GORM has a bug (go-gorm/gorm#7693) where AutoMigrate with multiple models
// creates spurious reverse FK constraints when a child model's composite PK
// shares a column with a parent model's PK. We suppress GORM's FK generation
// on affected fields (constraint:-) and create the correct forward FKs here.
//
// Also cleans up duplicate Python-era FK constraints that lack ON DELETE CASCADE.
func postMigrate(db database.Database) error {
	if !db.IsPostgres() {
		return nil
	}

	gdb := db.GORM()

	// Forward FK constraints suppressed by constraint:- in models.go.
	// Drop any old-style (Python-era or previous GORM) versions first, then
	// create with ON DELETE CASCADE. Idempotent: safe to run on every startup.
	constraints := []struct {
		table      string
		old        []string
		name       string
		definition string
	}{
		{
			table:      "git_commit_files",
			old:        []string{"git_commit_files_commit_sha_fkey"},
			name:       "fk_commit_files_commit_sha",
			definition: "FOREIGN KEY (commit_sha) REFERENCES git_commits(commit_sha) ON DELETE CASCADE",
		},
		{
			table:      "commit_indexes",
			old:        []string{"commit_indexes_commit_sha_fkey"},
			name:       "fk_commit_indexes_commit_sha",
			definition: "FOREIGN KEY (commit_sha) REFERENCES git_commits(commit_sha) ON DELETE CASCADE",
		},
	}

	for _, c := range constraints {
		for _, old := range c.old {
			if err := gdb.Exec(fmt.Sprintf(
				`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`, c.table, old,
			)).Error; err != nil {
				return fmt.Errorf("drop old constraint %s.%s: %w", c.table, old, err)
			}
		}
		if err := gdb.Exec(fmt.Sprintf(
			`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`, c.table, c.name,
		)).Error; err != nil {
			return fmt.Errorf("drop constraint %s.%s: %w", c.table, c.name, err)
		}
		if err := gdb.Exec(fmt.Sprintf(
			`ALTER TABLE %s ADD CONSTRAINT %s %s`, c.table, c.name, c.definition,
		)).Error; err != nil {
			return fmt.Errorf("create constraint %s.%s: %w", c.table, c.name, err)
		}
	}

	// Clean up duplicate Python-era FK constraints (superseded by GORM equivalents).
	oldFKs := []struct{ table, name string }{
		{"git_commits", "git_commits_repo_id_fkey"},
		{"git_branches", "git_branches_repo_id_fkey"},
		{"git_tags", "git_tags_repo_id_fkey"},
		{"enrichment_associations", "enrichment_associations_enrichment_id_fkey"},
		{"task_status", "task_status_parent_fkey"},
	}
	for _, fk := range oldFKs {
		if err := gdb.Exec(fmt.Sprintf(
			`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`, fk.table, fk.name,
		)).Error; err != nil {
			return fmt.Errorf("drop old constraint %s.%s: %w", fk.table, fk.name, err)
		}
	}

	return nil
}
