// Package testutil provides common test utilities and fixtures.
package testutil

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/helixml/kodit/internal/config"
	"github.com/helixml/kodit/internal/database"
)

// TestDatabase creates an in-memory SQLite database for testing.
func TestDatabase(t *testing.T) database.Database {
	t.Helper()
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := database.NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("testutil.TestDatabase: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

// TestConfig creates a test configuration.
func TestConfig(t *testing.T) config.AppConfig {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	return config.NewAppConfigWithOptions(
		config.WithDataDir(tmpDir),
		config.WithDBURL("sqlite:///"+dbPath),
		config.WithLogLevel("DEBUG"),
	)
}

// TestDatabaseWithSchema creates a test database and executes the provided schema SQL.
func TestDatabaseWithSchema(t *testing.T, schema string) database.Database {
	t.Helper()
	ctx := context.Background()
	db := TestDatabase(t)

	if schema != "" {
		if err := db.Session(ctx).Exec(schema).Error; err != nil {
			t.Fatalf("testutil.TestDatabaseWithSchema: execute schema: %v", err)
		}
	}

	return db
}

// Ptr returns a pointer to the given value. Useful for creating pointers to literals.
func Ptr[T any](v T) *T {
	return &v
}

// Must panics if err is not nil, otherwise returns v.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// AssertEqual fails the test if got != want.
func AssertEqual[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", msg, err)
	}
}

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error, got nil", msg)
	}
}

// AssertNil fails the test if v is not nil.
func AssertNil[T any](t *testing.T, v *T, msg string) {
	t.Helper()
	if v != nil {
		t.Errorf("%s: expected nil, got %v", msg, v)
	}
}

// AssertNotNil fails the test if v is nil.
func AssertNotNil[T any](t *testing.T, v *T, msg string) {
	t.Helper()
	if v == nil {
		t.Errorf("%s: expected non-nil, got nil", msg)
	}
}

// AssertLen fails the test if len(slice) != want.
func AssertLen[T any](t *testing.T, slice []T, want int, msg string) {
	t.Helper()
	if len(slice) != want {
		t.Errorf("%s: len = %d, want %d", msg, len(slice), want)
	}
}
