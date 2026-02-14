package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDatabase_SQLite(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	if !db.IsSQLite() {
		t.Error("expected IsSQLite() to return true")
	}
	if db.IsPostgres() {
		t.Error("expected IsPostgres() to return false")
	}
}

func TestNewDatabase_UnsupportedDriver(t *testing.T) {
	ctx := context.Background()

	_, err := NewDatabase(ctx, "mysql://user:pass@localhost/db")
	if err == nil {
		t.Fatal("expected error for unsupported driver")
	}
	if err.Error() != "parse database url: unsupported database driver" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDatabase_Session(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	session := db.Session(ctx)
	if session == nil {
		t.Fatal("Session returned nil")
	}

	// Execute a simple query to verify session works
	var result int
	err = session.Raw("SELECT 1").Scan(&result).Error
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result != 1 {
		t.Errorf("expected result 1, got %d", result)
	}
}

func TestDatabase_ConfigurePool(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	err = db.ConfigurePool(10, 5, 30*time.Minute)
	if err != nil {
		t.Fatalf("ConfigurePool: %v", err)
	}
}

func TestDatabase_Close(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify the database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestParseDialector(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "sqlite",
			url:     "sqlite:///path/to/db.sqlite",
			wantErr: false,
		},
		{
			name:    "postgresql",
			url:     "postgresql://user:pass@localhost:5432/dbname",
			wantErr: false,
		},
		{
			name:    "postgres",
			url:     "postgres://user:pass@localhost:5432/dbname",
			wantErr: false,
		},
		{
			name:    "unsupported",
			url:     "mysql://user:pass@localhost/db",
			wantErr: true,
		},
		{
			name:    "empty",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDialector(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDialector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
