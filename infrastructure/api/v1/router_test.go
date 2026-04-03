package v1_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/domain/sourcelocation"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
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

	lrStore := persistence.NewSourceLocationStore(db)
	_, err = lrStore.Save(ctx, sourcelocation.New(saved.ID(), 10, 20))
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

func TestRepositoriesRouter_List_SanitizesCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db := openTestDB(t, dbPath)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	store := persistence.NewRepositoryStore(db)
	repo, err := repository.NewRepository("http://user:secret-token@api:8080/git/my-repo")
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	_, err = store.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repository: %v", err)
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

	router := v1.NewRepositoriesRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %v, want %v; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	body := w.Body.String()
	if strings.Contains(body, "secret-token") {
		t.Errorf("repository list API leaks credentials: %s", body)
	}
	if !strings.Contains(body, "http://api:8080/git/my-repo") {
		t.Errorf("expected sanitized URL in response, got: %s", body)
	}
}

func newTestClientWithSeededRepository(t *testing.T) (*kodit.Client, int64) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db := openTestDB(t, dbPath)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	store := persistence.NewRepositoryStore(db)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	saved, err := store.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repository: %v", err)
	}
	repoID := saved.ID()
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
	return client, repoID
}

func TestRepositoriesRouter_GetChunkingConfig(t *testing.T) {
	client, repoID := newTestClientWithSeededRepository(t)

	router := v1.NewRepositoriesRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%d/config/chunking", repoID), nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %v, want %v; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response dto.ChunkingConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data.Type != "chunking-config" {
		t.Errorf("type = %v, want chunking-config", response.Data.Type)
	}
	if response.Data.Attributes.ChunkSize != 1500 {
		t.Errorf("chunk_size = %v, want 1500", response.Data.Attributes.ChunkSize)
	}
	if response.Data.Attributes.ChunkOverlap != 200 {
		t.Errorf("chunk_overlap = %v, want 200", response.Data.Attributes.ChunkOverlap)
	}
	if response.Data.Attributes.MinChunkSize != 50 {
		t.Errorf("min_chunk_size = %v, want 50", response.Data.Attributes.MinChunkSize)
	}
}

func TestRepositoriesRouter_GetChunkingConfig_NotFound(t *testing.T) {
	client := newTestClient(t)

	router := v1.NewRepositoriesRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, "/999/config/chunking", nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestRepositoriesRouter_UpdateChunkingConfig(t *testing.T) {
	client, repoID := newTestClientWithSeededRepository(t)

	router := v1.NewRepositoriesRouter(client)
	routes := router.Routes()

	body := `{"data":{"type":"chunking-config","attributes":{"chunk_size":2000,"chunk_overlap":300,"min_chunk_size":100}}}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%d/config/chunking", repoID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %v, want %v; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response dto.ChunkingConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data.Attributes.ChunkSize != 2000 {
		t.Errorf("chunk_size = %v, want 2000", response.Data.Attributes.ChunkSize)
	}
	if response.Data.Attributes.ChunkOverlap != 300 {
		t.Errorf("chunk_overlap = %v, want 300", response.Data.Attributes.ChunkOverlap)
	}
	if response.Data.Attributes.MinChunkSize != 100 {
		t.Errorf("min_chunk_size = %v, want 100", response.Data.Attributes.MinChunkSize)
	}

	// Verify GET returns updated values
	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%d/config/chunking", repoID), nil)
	getW := httptest.NewRecorder()
	routes.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GET status code = %v, want %v", getW.Code, http.StatusOK)
	}

	var getResponse dto.ChunkingConfigResponse
	if err := json.NewDecoder(getW.Body).Decode(&getResponse); err != nil {
		t.Fatalf("failed to decode GET response: %v", err)
	}

	if getResponse.Data.Attributes.ChunkSize != 2000 {
		t.Errorf("GET chunk_size = %v, want 2000", getResponse.Data.Attributes.ChunkSize)
	}
	if getResponse.Data.Attributes.ChunkOverlap != 300 {
		t.Errorf("GET chunk_overlap = %v, want 300", getResponse.Data.Attributes.ChunkOverlap)
	}
	if getResponse.Data.Attributes.MinChunkSize != 100 {
		t.Errorf("GET min_chunk_size = %v, want 100", getResponse.Data.Attributes.MinChunkSize)
	}
}

func TestRepositoriesRouter_UpdateChunkingConfig_InvalidParams(t *testing.T) {
	client, repoID := newTestClientWithSeededRepository(t)

	router := v1.NewRepositoriesRouter(client)
	routes := router.Routes()

	body := `{"data":{"type":"chunking-config","attributes":{"chunk_size":100,"chunk_overlap":200,"min_chunk_size":50}}}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%d/config/chunking", repoID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("expected error status for invalid params, got %v", w.Code)
	}
}

func TestRepositoriesRouter_Get_SanitizesCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db := openTestDB(t, dbPath)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	store := persistence.NewRepositoryStore(db)
	repo, err := repository.NewRepository("http://user:secret-token@api:8080/git/my-repo")
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	saved, err := store.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repository: %v", err)
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

	router := v1.NewRepositoriesRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%d", saved.ID()), nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %v, want %v; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	body := w.Body.String()
	if strings.Contains(body, "secret-token") {
		t.Errorf("repository get API leaks credentials: %s", body)
	}
	if !strings.Contains(body, "http://api:8080/git/my-repo") {
		t.Errorf("expected sanitized URL in response, got: %s", body)
	}
}

func newTestClientWithSeededRepositoryAndPipeline(t *testing.T) (*kodit.Client, int64, int64) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db := openTestDB(t, dbPath)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	repoStore := persistence.NewRepositoryStore(db)
	repo, err := repository.NewRepository("https://github.com/test/repo")
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	pipelineStore := persistence.NewPipelineStore(db)
	pipeline := repository.NewPipeline("test-pipeline")
	savedPipeline, err := pipelineStore.Save(ctx, pipeline)
	if err != nil {
		t.Fatalf("save pipeline: %v", err)
	}

	repo = repo.WithPipelineID(savedPipeline.ID())
	savedRepo, err := repoStore.Save(ctx, repo)
	if err != nil {
		t.Fatalf("save repository: %v", err)
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
	return client, savedRepo.ID(), savedPipeline.ID()
}

func TestRepositoriesRouter_GetPipelineConfig(t *testing.T) {
	client, repoID, pipelineID := newTestClientWithSeededRepositoryAndPipeline(t)

	router := v1.NewRepositoriesRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%d/config/pipeline", repoID), nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %v, want %v; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response dto.PipelineConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data.Type != "pipeline-config" {
		t.Errorf("type = %v, want pipeline-config", response.Data.Type)
	}
	if response.Data.Attributes.PipelineID != pipelineID {
		t.Errorf("pipeline_id = %v, want %v", response.Data.Attributes.PipelineID, pipelineID)
	}
	expectedLink := fmt.Sprintf("/api/v1/pipelines/%d", pipelineID)
	if response.Data.Links.Pipeline != expectedLink {
		t.Errorf("links.pipeline = %v, want %v", response.Data.Links.Pipeline, expectedLink)
	}
	if len(response.Included) != 0 {
		t.Errorf("expected no included resources by default, got %d", len(response.Included))
	}
}

func TestRepositoriesRouter_GetPipelineConfig_WithInclude(t *testing.T) {
	client, repoID, pipelineID := newTestClientWithSeededRepositoryAndPipeline(t)

	router := v1.NewRepositoriesRouter(client)
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%d/config/pipeline?include=pipeline", repoID), nil)
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %v, want %v; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response dto.PipelineConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data.Attributes.PipelineID != pipelineID {
		t.Errorf("pipeline_id = %v, want %v", response.Data.Attributes.PipelineID, pipelineID)
	}
	if len(response.Included) != 1 {
		t.Fatalf("expected 1 included resource, got %d", len(response.Included))
	}
	included := response.Included[0]
	if included.Type != "pipeline" {
		t.Errorf("included type = %v, want pipeline", included.Type)
	}
	if included.ID != pipelineID {
		t.Errorf("included id = %v, want %v", included.ID, pipelineID)
	}
	if included.Attributes.Name != "test-pipeline" {
		t.Errorf("included name = %v, want test-pipeline", included.Attributes.Name)
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
