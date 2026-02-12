package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

func TestNewTransaction(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	txn, err := NewTransaction(ctx, db)
	if err != nil {
		t.Fatalf("NewTransaction: %v", err)
	}

	if txn.Session() == nil {
		t.Error("Session() returned nil")
	}

	if err := txn.Rollback(); err != nil {
		t.Errorf("Rollback: %v", err)
	}
}

func TestTransaction_Commit(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a test table
	if err := db.Session(ctx).Exec("CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	txn, err := NewTransaction(ctx, db)
	if err != nil {
		t.Fatalf("NewTransaction: %v", err)
	}

	if err := txn.Session().Exec("INSERT INTO test_items (name) VALUES (?)", "item1").Error; err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := txn.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify data persisted
	var count int64
	if err := db.Session(ctx).Raw("SELECT COUNT(*) FROM test_items").Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	// Second commit should be no-op
	if err := txn.Commit(); err != nil {
		t.Errorf("second Commit should not error: %v", err)
	}
}

func TestTransaction_Rollback(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a test table
	if err := db.Session(ctx).Exec("CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	txn, err := NewTransaction(ctx, db)
	if err != nil {
		t.Fatalf("NewTransaction: %v", err)
	}

	if err := txn.Session().Exec("INSERT INTO test_items (name) VALUES (?)", "item1").Error; err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := txn.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	// Verify data was not persisted
	var count int64
	if err := db.Session(ctx).Raw("SELECT COUNT(*) FROM test_items").Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0 after rollback, got %d", count)
	}

	// Rollback after rollback should be no-op
	if err := txn.Rollback(); err != nil {
		t.Errorf("second Rollback should not error: %v", err)
	}
}

func TestTransaction_RollbackAfterCommit(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	txn, err := NewTransaction(ctx, db)
	if err != nil {
		t.Fatalf("NewTransaction: %v", err)
	}

	if err := txn.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Rollback after commit should be no-op
	if err := txn.Rollback(); err != nil {
		t.Errorf("Rollback after Commit should not error: %v", err)
	}
}

func TestWithTransaction_Success(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a test table
	if err := db.Session(ctx).Exec("CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	err = WithTransaction(ctx, db, func(tx *gorm.DB) error {
		return tx.Exec("INSERT INTO test_items (name) VALUES (?)", "item1").Error
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}

	// Verify data persisted
	var count int64
	if err := db.Session(ctx).Raw("SELECT COUNT(*) FROM test_items").Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestWithTransaction_Error(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a test table
	if err := db.Session(ctx).Exec("CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	testErr := errors.New("test error")
	err = WithTransaction(ctx, db, func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT INTO test_items (name) VALUES (?)", "item1").Error; err != nil {
			return err
		}
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Errorf("expected test error, got: %v", err)
	}

	// Verify data was rolled back
	var count int64
	if err := db.Session(ctx).Raw("SELECT COUNT(*) FROM test_items").Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0 after error, got %d", count)
	}
}

func TestWithTransactionResult_Success(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	result, err := WithTransactionResult(ctx, db, func(tx *gorm.DB) (int, error) {
		var val int
		if err := tx.Raw("SELECT 42").Scan(&val).Error; err != nil {
			return 0, err
		}
		return val, nil
	})
	if err != nil {
		t.Fatalf("WithTransactionResult: %v", err)
	}
	if result != 42 {
		t.Errorf("expected result 42, got %d", result)
	}
}

func TestWithTransactionResult_Error(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	testErr := errors.New("test error")
	_, err = WithTransactionResult(ctx, db, func(tx *gorm.DB) (int, error) {
		return 0, testErr
	})
	if !errors.Is(err, testErr) {
		t.Errorf("expected test error, got: %v", err)
	}
}
