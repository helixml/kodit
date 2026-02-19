// Package testdb provides a shared test database helper for fast,
// realistic testing against an in-memory SQLite database.
package testdb

import (
	"context"
	"testing"

	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/database"
)

// New creates an in-memory SQLite database with all migrations applied.
// The database is automatically closed when the test finishes.
func New(t *testing.T) database.Database {
	t.Helper()
	ctx := context.Background()
	db, err := database.NewDatabase(ctx, "sqlite:///:memory:")
	if err != nil {
		t.Fatalf("testdb.New: open database: %v", err)
	}
	if err := persistence.AutoMigrate(db); err != nil {
		_ = db.Close()
		t.Fatalf("testdb.New: auto migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// NewPlain creates an in-memory SQLite database without running migrations.
// Useful for tests that manage their own schema (e.g. the database package's
// own unit tests for Repository, Transaction, and Query).
func NewPlain(t *testing.T) database.Database {
	t.Helper()
	ctx := context.Background()
	db, err := database.NewDatabase(ctx, "sqlite:///:memory:")
	if err != nil {
		t.Fatalf("testdb.NewPlain: open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// WithSchema creates an in-memory SQLite database and executes the given
// SQL statements to set up a custom schema. Useful for database package
// tests that need specific table structures.
func WithSchema(t *testing.T, statements ...string) database.Database {
	t.Helper()
	ctx := context.Background()
	db := NewPlain(t)
	for _, stmt := range statements {
		if err := db.Session(ctx).Exec(stmt).Error; err != nil {
			t.Fatalf("testdb.WithSchema: %v\nSQL: %s", err, stmt)
		}
	}
	return db
}
