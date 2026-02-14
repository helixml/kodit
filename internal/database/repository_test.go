package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/helixml/kodit/domain/repository"
)

// Test domain entity
type testUser struct {
	id     int64
	name   string
	email  string
	active bool
}

func (u testUser) ID() int64    { return u.id }
func (u testUser) Name() string { return u.name }

// Test database entity
type testUserEntity struct {
	ID     int64  `gorm:"primaryKey"`
	Name   string `gorm:"column:name"`
	Email  string `gorm:"column:email"`
	Active bool   `gorm:"column:active"`
}

func (testUserEntity) TableName() string { return "users" }

// Test mapper
type testUserMapper struct{}

func (m testUserMapper) ToDomain(entity testUserEntity) testUser {
	return testUser{
		id:     entity.ID,
		name:   entity.Name,
		email:  entity.Email,
		active: entity.Active,
	}
}

func (m testUserMapper) ToModel(domain testUser) testUserEntity {
	return testUserEntity{
		ID:     domain.id,
		Name:   domain.name,
		Email:  domain.email,
		Active: domain.active,
	}
}

func setupTestRepo(t *testing.T) (Repository[testUser, testUserEntity], func()) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	err = db.Session(ctx).Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			active BOOLEAN DEFAULT true
		)
	`).Error
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	repo := NewRepository[testUser, testUserEntity](db, testUserMapper{}, "user")
	cleanup := func() { _ = db.Close() }

	return repo, cleanup
}

func seedUser(t *testing.T, repo Repository[testUser, testUserEntity], name, email string, active bool) testUser {
	t.Helper()
	ctx := context.Background()
	entity := testUserEntity{Name: name, Email: email, Active: active}
	result := repo.DB(ctx).Create(&entity)
	if result.Error != nil {
		t.Fatalf("seed user: %v", result.Error)
	}
	return repo.Mapper().ToDomain(entity)
}

func TestRepository_Find(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	seedUser(t, repo, "Alice", "alice@example.com", true)
	seedUser(t, repo, "Bob", "bob@example.com", false)
	seedUser(t, repo, "Charlie", "charlie@example.com", true)

	found, err := repo.Find(ctx, repository.WithCondition("active", true))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if len(found) != 2 {
		t.Errorf("expected 2 active users, got %d", len(found))
	}
}

func TestRepository_Find_All(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	seedUser(t, repo, "Alice", "alice@example.com", true)
	seedUser(t, repo, "Bob", "bob@example.com", false)

	found, err := repo.Find(ctx)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if len(found) != 2 {
		t.Errorf("expected 2 users, got %d", len(found))
	}
}

func TestRepository_FindOne(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	seedUser(t, repo, "Alice", "alice@example.com", true)

	found, err := repo.FindOne(ctx, repository.WithCondition("name", "Alice"))
	if err != nil {
		t.Fatalf("FindOne: %v", err)
	}

	if found.Name() != "Alice" {
		t.Errorf("Name() = %v, want Alice", found.Name())
	}
}

func TestRepository_FindOne_NotFound(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	_, err := repo.FindOne(ctx, repository.WithCondition("name", "NonExistent"))
	if err == nil {
		t.Fatal("expected error for non-existent entity")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestRepository_Exists(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	seedUser(t, repo, "Alice", "alice@example.com", true)

	exists, err := repo.Exists(ctx, repository.WithCondition("name", "Alice"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("expected Exists to return true")
	}

	exists, err = repo.Exists(ctx, repository.WithCondition("name", "Bob"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("expected Exists to return false for non-existent name")
	}
}

func TestRepository_DeleteBy(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	seedUser(t, repo, "Alice", "alice@example.com", true)
	seedUser(t, repo, "Bob", "bob@example.com", false)

	err := repo.DeleteBy(ctx, repository.WithCondition("name", "Alice"))
	if err != nil {
		t.Fatalf("DeleteBy: %v", err)
	}

	found, err := repo.Find(ctx)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(found) != 1 {
		t.Errorf("expected 1 user after delete, got %d", len(found))
	}
	if found[0].Name() != "Bob" {
		t.Errorf("expected remaining user Bob, got %s", found[0].Name())
	}
}

func TestRepository_DB(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	db := repo.DB(ctx)
	if db == nil {
		t.Fatal("expected non-nil DB session")
	}
}
