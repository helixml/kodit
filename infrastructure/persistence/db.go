// Package persistence provides database storage implementations.
package persistence

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/helixml/kodit/internal/database"
	"gorm.io/gorm"
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

	// Add auto-increment id column to git_commit_files if missing.
	// Old dumps predate this column; without it existing rows get id=0
	// and enrichment associations are never created.
	// On fresh databases the table doesn't exist yet — AutoMigrate creates it
	// with the id column, so we skip this migration entirely.
	var tableExists bool
	err = gdb.Raw(`
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'git_commit_files'
		)
	`).Scan(&tableExists).Error
	if err != nil {
		return err
	}
	if tableExists {
		var hasIDColumn bool
		err = gdb.Raw(`
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'git_commit_files' AND column_name = 'id'
			)
		`).Scan(&hasIDColumn).Error
		if err != nil {
			return err
		}
		if !hasIDColumn {
			slog.Warn("one-time database migration: adding id column to git_commit_files")
			stmts := []string{
				`CREATE SEQUENCE IF NOT EXISTS git_commit_files_id_seq`,
				`ALTER TABLE git_commit_files ADD COLUMN id BIGINT NOT NULL DEFAULT nextval('git_commit_files_id_seq')`,
				`ALTER SEQUENCE git_commit_files_id_seq OWNED BY git_commit_files.id`,
			}
			for _, stmt := range stmts {
				if err := gdb.Exec(stmt).Error; err != nil {
					return fmt.Errorf("git_commit_files id migration: %w", err)
				}
			}
			slog.Info("one-time database migration complete: git_commit_files.id added")
		}
	}

	// Replace non-unique ix_tasks_dedup_key with the unique index GORM expects.
	var hasOldDedupIndex bool
	err = gdb.Raw(`
		SELECT EXISTS(
			SELECT 1 FROM pg_indexes
			WHERE indexname = 'ix_tasks_dedup_key'
		)
	`).Scan(&hasOldDedupIndex).Error
	if err != nil {
		return err
	}
	if hasOldDedupIndex {
		slog.Warn("one-time database migration: replacing non-unique ix_tasks_dedup_key with unique index")
		stmts := []string{
			`DROP INDEX IF EXISTS ix_tasks_dedup_key`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_dedup_key ON tasks (dedup_key)`,
		}
		for _, stmt := range stmts {
			if err := gdb.Exec(stmt).Error; err != nil {
				return fmt.Errorf("tasks dedup_key index migration: %w", err)
			}
		}
		slog.Info("one-time database migration complete: tasks.dedup_key unique index created")
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

// allModels returns every GORM model that AutoMigrate manages.
func allModels() []interface{} {
	return []interface{}{
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
	}
}

// ValidateSchema verifies every GORM model field has a corresponding column
// in the database. Returns an error listing any missing columns.
func ValidateSchema(db database.Database) error {
	gdb := db.GORM()
	migrator := gdb.Migrator()

	var missing []string
	for _, model := range allModels() {
		stmt := &gorm.Statement{DB: gdb}
		if err := stmt.Parse(model); err != nil {
			return fmt.Errorf("parse model schema: %w", err)
		}

		columnTypes, err := migrator.ColumnTypes(model)
		if err != nil {
			return fmt.Errorf("get column types for %s: %w", stmt.Table, err)
		}

		actual := make(map[string]bool, len(columnTypes))
		for _, ct := range columnTypes {
			actual[ct.Name()] = true
		}

		for _, field := range stmt.Schema.Fields {
			if field.DBName == "" || field.DBName == "-" {
				continue
			}
			if !actual[field.DBName] {
				missing = append(missing, stmt.Table+"."+field.DBName)
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("schema validation failed — missing columns: %s", strings.Join(missing, ", "))
	}
	return nil
}
