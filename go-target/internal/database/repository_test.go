package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// Test domain entity
type testUser struct {
	id     int64
	name   string
	email  string
	active bool
}

func (u testUser) ID() int64      { return u.id }
func (u testUser) Name() string   { return u.name }
func (u testUser) Email() string  { return u.email }
func (u testUser) Active() bool   { return u.active }

// Test database entity
type testUserEntity struct {
	ID     int64  `gorm:"primaryKey"`
	Name   string `gorm:"column:name"`
	Email  string `gorm:"column:email"`
	Active bool   `gorm:"column:active"`
}

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

func (m testUserMapper) ToDatabase(domain testUser) testUserEntity {
	return testUserEntity{
		ID:     domain.id,
		Name:   domain.name,
		Email:  domain.email,
		Active: domain.active,
	}
}

func setupTestRepo(t *testing.T) (Repository[testUser, testUserEntity], Database, func()) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	// Create test table
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

	repo := NewRepository[testUser, testUserEntity](db, testUserMapper{}, "users")
	cleanup := func() { _ = db.Close() }

	return repo, db, cleanup
}

func TestRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	user := testUser{name: "Alice", email: "alice@example.com", active: true}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.ID() == 0 {
		t.Error("expected non-zero ID")
	}
	if created.Name() != "Alice" {
		t.Errorf("Name() = %v, want Alice", created.Name())
	}
}

func TestRepository_Get(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	user := testUser{name: "Bob", email: "bob@example.com", active: true}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	retrieved, err := repo.Get(ctx, created.ID())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if retrieved.Name() != "Bob" {
		t.Errorf("Name() = %v, want Bob", retrieved.Name())
	}
}

func TestRepository_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	_, err := repo.Get(ctx, 999)
	if err == nil {
		t.Fatal("expected error for non-existent ID")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestRepository_Find(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create test users
	users := []testUser{
		{name: "Alice", email: "alice@example.com", active: true},
		{name: "Bob", email: "bob@example.com", active: false},
		{name: "Charlie", email: "charlie@example.com", active: true},
	}
	for _, u := range users {
		if _, err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Find active users
	query := NewQuery().Equal("active", true)
	found, err := repo.Find(ctx, query)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if len(found) != 2 {
		t.Errorf("expected 2 active users, got %d", len(found))
	}
}

func TestRepository_FindAll(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	users := []testUser{
		{name: "Alice", email: "alice@example.com", active: true},
		{name: "Bob", email: "bob@example.com", active: false},
	}
	for _, u := range users {
		if _, err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	found, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	if len(found) != 2 {
		t.Errorf("expected 2 users, got %d", len(found))
	}
}

func TestRepository_Save(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	user := testUser{name: "Alice", email: "alice@example.com", active: true}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update user
	updated := testUser{
		id:     created.ID(),
		name:   "Alice Smith",
		email:  "alice.smith@example.com",
		active: false,
	}
	saved, err := repo.Save(ctx, updated)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if saved.Name() != "Alice Smith" {
		t.Errorf("Name() = %v, want Alice Smith", saved.Name())
	}

	// Verify update persisted
	retrieved, err := repo.Get(ctx, created.ID())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if retrieved.Name() != "Alice Smith" {
		t.Errorf("Name() after Get = %v, want Alice Smith", retrieved.Name())
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	user := testUser{name: "Alice", email: "alice@example.com", active: true}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = repo.Delete(ctx, created)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.Get(ctx, created.ID())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got: %v", err)
	}
}

func TestRepository_DeleteByID(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	user := testUser{name: "Alice", email: "alice@example.com", active: true}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = repo.DeleteByID(ctx, created.ID())
	if err != nil {
		t.Fatalf("DeleteByID: %v", err)
	}

	_, err = repo.Get(ctx, created.ID())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got: %v", err)
	}
}

func TestRepository_Count(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	users := []testUser{
		{name: "Alice", email: "alice@example.com", active: true},
		{name: "Bob", email: "bob@example.com", active: false},
		{name: "Charlie", email: "charlie@example.com", active: true},
	}
	for _, u := range users {
		if _, err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	count, err := repo.Count(ctx, NewQuery().Equal("active", true))
	if err != nil {
		t.Fatalf("Count: %v", err)
	}

	if count != 2 {
		t.Errorf("Count() = %d, want 2", count)
	}
}

func TestRepository_Exists(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	user := testUser{name: "Alice", email: "alice@example.com", active: true}
	if _, err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	exists, err := repo.Exists(ctx, NewQuery().Equal("name", "Alice"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("expected Exists to return true")
	}

	exists, err = repo.Exists(ctx, NewQuery().Equal("name", "Bob"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("expected Exists to return false for non-existent name")
	}
}

func TestRepository_ExistsByID(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	user := testUser{name: "Alice", email: "alice@example.com", active: true}
	created, err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	exists, err := repo.ExistsByID(ctx, created.ID())
	if err != nil {
		t.Fatalf("ExistsByID: %v", err)
	}
	if !exists {
		t.Error("expected ExistsByID to return true")
	}

	exists, err = repo.ExistsByID(ctx, 999)
	if err != nil {
		t.Fatalf("ExistsByID: %v", err)
	}
	if exists {
		t.Error("expected ExistsByID to return false for non-existent ID")
	}
}

func TestRepository_FindOne(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	user := testUser{name: "Alice", email: "alice@example.com", active: true}
	if _, err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindOne(ctx, NewQuery().Equal("name", "Alice"))
	if err != nil {
		t.Fatalf("FindOne: %v", err)
	}

	if found.Name() != "Alice" {
		t.Errorf("Name() = %v, want Alice", found.Name())
	}
}

func TestRepository_FindOne_NotFound(t *testing.T) {
	ctx := context.Background()
	repo, _, cleanup := setupTestRepo(t)
	defer cleanup()

	_, err := repo.FindOne(ctx, NewQuery().Equal("name", "NonExistent"))
	if err == nil {
		t.Fatal("expected error for non-existent entity")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

