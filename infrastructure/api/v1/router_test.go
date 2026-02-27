package v1_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/domain/enrichment"
	v1 "github.com/helixml/kodit/infrastructure/api/v1"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/database"
)

func newTestClient(t *testing.T) *kodit.Client {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(tmpDir),
		kodit.WithSkipProviderValidation(),
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// newTestClientWithSeededEnrichment creates a client with a pre-seeded enrichment.
// It opens the DB first to seed data, then creates the client.
func newTestClientWithSeededEnrichment(t *testing.T) (*kodit.Client, enrichment.Enrichment) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db := openTestDB(t, dbPath)
	store := persistence.NewEnrichmentStore(db)
	e := enrichment.NewEnrichment(
		enrichment.TypeDevelopment,
		enrichment.SubtypeSnippet,
		enrichment.EntityTypeSnippet,
		"test content",
	)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	saved, err := store.Save(ctx, e)
	if err != nil {
		t.Fatalf("save enrichment: %v", err)
	}
	_ = db.Close()

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(tmpDir),
		kodit.WithSkipProviderValidation(),
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, saved
}

func TestEnrichmentsRouter_List(t *testing.T) {
	client, _ := newTestClientWithSeededEnrichment(t)

	router := v1.NewEnrichmentsRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, "/?enrichment_type=development", nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusOK)
	}

	var response dto.EnrichmentJSONAPIListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 1 {
		t.Errorf("len(Data) = %v, want 1", len(response.Data))
	}
	if response.Data[0].Type != "enrichment" {
		t.Errorf("type = %v, want enrichment", response.Data[0].Type)
	}
}

func TestEnrichmentsRouter_List_NoFilter(t *testing.T) {
	client := newTestClient(t)

	router := v1.NewEnrichmentsRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusOK)
	}

	var response dto.EnrichmentJSONAPIListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 0 {
		t.Errorf("len(Data) = %v, want 0 (no filter specified)", len(response.Data))
	}
}

func TestEnrichmentsRouter_Get(t *testing.T) {
	client, saved := newTestClientWithSeededEnrichment(t)

	router := v1.NewEnrichmentsRouter(client)
	routes := router.Routes()

	idStr := fmt.Sprintf("%d", saved.ID())
	req := httptest.NewRequest(http.MethodGet, "/"+idStr, nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusOK)
	}

	var response dto.EnrichmentJSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data.ID != idStr {
		t.Errorf("ID = %v, want %v", response.Data.ID, idStr)
	}
	if response.Data.Type != "enrichment" {
		t.Errorf("type = %v, want enrichment", response.Data.Type)
	}
}

func TestEnrichmentsRouter_Get_NotFound(t *testing.T) {
	client := newTestClient(t)

	router := v1.NewEnrichmentsRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, "/999", nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func openTestDB(t *testing.T, dbPath string) database.Database {
	t.Helper()
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	db, err := database.NewDatabase(ctx, "sqlite:///"+dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := persistence.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// newTestClientWithSeededLineRange creates a client with a pre-seeded enrichment
// and chunk line range. Returns the client and the saved enrichment.
func newTestClientWithSeededLineRange(t *testing.T) (*kodit.Client, enrichment.Enrichment) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db := openTestDB(t, dbPath)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	store := persistence.NewEnrichmentStore(db)
	e := enrichment.NewEnrichment(
		enrichment.TypeDevelopment,
		enrichment.SubtypeSnippet,
		enrichment.EntityTypeSnippet,
		"test content",
	)
	saved, err := store.Save(ctx, e)
	if err != nil {
		t.Fatalf("save enrichment: %v", err)
	}

	lrStore := persistence.NewChunkLineRangeStore(db)
	_, err = lrStore.Save(ctx, chunk.NewLineRange(saved.ID(), 10, 20))
	if err != nil {
		t.Fatalf("save line range: %v", err)
	}

	_ = db.Close()

	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(tmpDir),
		kodit.WithSkipProviderValidation(),
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, saved
}

func TestEnrichmentsRouter_Get_WithLineRange(t *testing.T) {
	client, saved := newTestClientWithSeededLineRange(t)

	router := v1.NewEnrichmentsRouter(client)
	routes := router.Routes()

	idStr := fmt.Sprintf("%d", saved.ID())
	req := httptest.NewRequest(http.MethodGet, "/"+idStr, nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %v, want %v", w.Code, http.StatusOK)
	}

	var response dto.EnrichmentJSONAPIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	attrs := response.Data.Attributes
	if attrs.StartLine == nil {
		t.Error("start_line is nil, want non-nil")
	} else if *attrs.StartLine != 10 {
		t.Errorf("start_line = %d, want 10", *attrs.StartLine)
	}
	if attrs.EndLine == nil {
		t.Error("end_line is nil, want non-nil")
	} else if *attrs.EndLine != 20 {
		t.Errorf("end_line = %d, want 20", *attrs.EndLine)
	}
}

func TestEnrichmentsRouter_List_WithLineRange(t *testing.T) {
	client, _ := newTestClientWithSeededLineRange(t)

	router := v1.NewEnrichmentsRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, "/?enrichment_type=development", nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %v, want %v", w.Code, http.StatusOK)
	}

	var response dto.EnrichmentJSONAPIListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Data) != 1 {
		t.Fatalf("len(Data) = %v, want 1", len(response.Data))
	}

	attrs := response.Data[0].Attributes
	if attrs.StartLine == nil {
		t.Error("start_line is nil, want non-nil")
	} else if *attrs.StartLine != 10 {
		t.Errorf("start_line = %d, want 10", *attrs.StartLine)
	}
	if attrs.EndLine == nil {
		t.Error("end_line is nil, want non-nil")
	} else if *attrs.EndLine != 20 {
		t.Errorf("end_line = %d, want 20", *attrs.EndLine)
	}
}
