package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/api"
	apimiddleware "github.com/helixml/kodit/infrastructure/api/middleware"
	v1 "github.com/helixml/kodit/infrastructure/api/v1"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/internal/database"
)

// TestServer wraps the API server for e2e testing.
type TestServer struct {
	t          *testing.T
	client     *kodit.Client
	db         database.Database
	httpServer *httptest.Server

	// Stores - for direct DB manipulation in tests
	repoStore        persistence.RepositoryStore
	commitStore      persistence.CommitStore
	branchStore      persistence.BranchStore
	tagStore         persistence.TagStore
	fileStore        persistence.FileStore
	snippetStore     persistence.SnippetStore
	taskStore        persistence.TaskStore
	taskStatusStore  persistence.StatusStore
	enrichmentStore  persistence.EnrichmentStore
	associationStore persistence.AssociationStore
}

// NewTestServer creates a new test server with all dependencies wired up.
// Creates a kodit.Client backed by SQLite and a separate DB handle for test data seeding.
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create the kodit client first
	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(tmpDir),
		kodit.WithSkipProviderValidation(),
	)
	if err != nil {
		t.Fatalf("create kodit client: %v", err)
	}

	// Open a separate DB handle for seeding test data
	db, err := database.NewDatabase(ctx, "sqlite:///"+dbPath)
	if err != nil {
		t.Fatalf("create database: %v", err)
	}

	// Create stores for direct test data manipulation
	repoStore := persistence.NewRepositoryStore(db)
	commitStore := persistence.NewCommitStore(db)
	branchStore := persistence.NewBranchStore(db)
	tagStore := persistence.NewTagStore(db)
	fileStore := persistence.NewFileStore(db)
	taskStore := persistence.NewTaskStore(db)
	taskStatusStore := persistence.NewStatusStore(db)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	snippetStore := persistence.NewSnippetStore(db)

	// Create API server using the client
	logger := client.Logger()
	server := api.NewServer(":0", logger)
	router := server.Router()

	// Apply middleware
	router.Use(apimiddleware.Logging(logger))
	router.Use(apimiddleware.CorrelationID)

	// Register routes — each router takes just the client
	router.Route("/api/v1", func(r chi.Router) {
		r.Mount("/repositories", v1.NewRepositoriesRouter(client).Routes())
		r.Mount("/enrichments", v1.NewEnrichmentsRouter(client).Routes())
		r.Mount("/queue", v1.NewQueueRouter(client).Routes())

		r.Mount("/search", v1.NewSearchRouter(client).Routes())
	})

	// Health check
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create httptest server
	httpServer := httptest.NewServer(router)

	ts := &TestServer{
		t:                t,
		client:           client,
		db:               db,
		httpServer:       httpServer,
		repoStore:        repoStore,
		commitStore:      commitStore,
		branchStore:      branchStore,
		tagStore:         tagStore,
		fileStore:        fileStore,
		snippetStore:     snippetStore,
		taskStore:        taskStore,
		taskStatusStore:  taskStatusStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
	}

	t.Cleanup(func() {
		ts.Close()
	})

	return ts
}

// URL returns the base URL of the test server.
func (ts *TestServer) URL() string {
	return ts.httpServer.URL
}

// Close shuts down the test server.
func (ts *TestServer) Close() {
	ts.httpServer.Close()
	_ = ts.client.Close()
	_ = ts.db.Close()
}

// GET performs a GET request and returns the response.
func (ts *TestServer) GET(path string) *http.Response {
	ts.t.Helper()
	resp, err := http.Get(ts.URL() + path)
	if err != nil {
		ts.t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// POST performs a POST request with JSON body and returns the response.
func (ts *TestServer) POST(path string, body any) *http.Response {
	ts.t.Helper()
	jsonBody, err := json.Marshal(body)
	if err != nil {
		ts.t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(ts.URL()+path, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		ts.t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// DELETE performs a DELETE request and returns the response.
func (ts *TestServer) DELETE(path string) *http.Response {
	ts.t.Helper()
	req, err := http.NewRequest(http.MethodDelete, ts.URL()+path, nil)
	if err != nil {
		ts.t.Fatalf("create DELETE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ts.t.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}

// DecodeJSON decodes the response body as JSON into v.
func (ts *TestServer) DecodeJSON(resp *http.Response, v any) {
	ts.t.Helper()
	defer func() {
		_ = resp.Body.Close()
	}()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		ts.t.Fatalf("decode response: %v", err)
	}
}

// ReadBody reads and returns the response body as a string.
func (ts *TestServer) ReadBody(resp *http.Response) string {
	ts.t.Helper()
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// CreateRepository creates a repository in the database directly.
func (ts *TestServer) CreateRepository(remoteURL string) repository.Repository {
	ts.t.Helper()
	ctx := context.Background()

	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		ts.t.Fatalf("create repo: %v", err)
	}
	saved, err := ts.repoStore.Save(ctx, repo)
	if err != nil {
		ts.t.Fatalf("save repo: %v", err)
	}
	return saved
}

// CreateTask creates a task in the database directly.
func (ts *TestServer) CreateTask(operation task.Operation, payload map[string]any) task.Task {
	ts.t.Helper()
	ctx := context.Background()

	tsk := task.NewTask(operation, int(task.PriorityNormal), payload)
	saved, err := ts.taskStore.Save(ctx, tsk)
	if err != nil {
		ts.t.Fatalf("save task: %v", err)
	}
	return saved
}

// CreateEnrichment creates an enrichment in the database directly.
func (ts *TestServer) CreateEnrichment(typ enrichment.Type, subtype enrichment.Subtype, content string) enrichment.Enrichment {
	ts.t.Helper()
	ctx := context.Background()

	e := enrichment.NewEnrichment(typ, subtype, enrichment.EntityTypeSnippet, content)
	saved, err := ts.enrichmentStore.Save(ctx, e)
	if err != nil {
		ts.t.Fatalf("save enrichment: %v", err)
	}
	return saved
}

// CreateCommit creates a commit in the database directly.
func (ts *TestServer) CreateCommit(repo repository.Repository, sha, message string) repository.Commit {
	ts.t.Helper()
	ctx := context.Background()

	author := repository.NewAuthor("Test User", "test@example.com")
	now := time.Now()
	commit := repository.NewCommit(sha, repo.ID(), message, author, author, now, now)
	saved, err := ts.commitStore.Save(ctx, commit)
	if err != nil {
		ts.t.Fatalf("save commit: %v", err)
	}
	return saved
}

// CreateFile creates a file in the database directly.
func (ts *TestServer) CreateFile(commitSHA, path, blobSHA, mimeType, extension string, size int64) repository.File {
	ts.t.Helper()
	ctx := context.Background()

	file := repository.NewFileWithDetails(commitSHA, path, blobSHA, mimeType, extension, size)
	saved, err := ts.fileStore.Save(ctx, file)
	if err != nil {
		ts.t.Fatalf("save file: %v", err)
	}
	return saved
}

// CreateSnippetForCommit creates a snippet and associates it with a commit.
func (ts *TestServer) CreateSnippetForCommit(commitSHA, content, extension string) snippet.Snippet {
	ts.t.Helper()
	ctx := context.Background()

	snip := snippet.NewSnippet(content, extension, nil)
	if err := ts.snippetStore.Save(ctx, commitSHA, []snippet.Snippet{snip}); err != nil {
		ts.t.Fatalf("save snippet: %v", err)
	}
	return snip
}

// CreateRepositoryWithWorkingCopy creates a repository with a working copy in the database directly.
func (ts *TestServer) CreateRepositoryWithWorkingCopy(remoteURL string) repository.Repository {
	ts.t.Helper()
	ctx := context.Background()

	repo, err := repository.NewRepository(remoteURL)
	if err != nil {
		ts.t.Fatalf("create repo: %v", err)
	}

	// Add a working copy (fake path, just needs to be non-empty)
	workingCopy := repository.NewWorkingCopy("/tmp/fake-repo", remoteURL)
	repo = repo.WithWorkingCopy(workingCopy)

	saved, err := ts.repoStore.Save(ctx, repo)
	if err != nil {
		ts.t.Fatalf("save repo: %v", err)
	}
	return saved
}

// CreateEnrichmentAssociation creates an enrichment association in the database directly.
func (ts *TestServer) CreateEnrichmentAssociation(e enrichment.Enrichment, entityType enrichment.EntityTypeKey, entityID string) enrichment.Association {
	ts.t.Helper()
	ctx := context.Background()

	assoc := enrichment.NewAssociation(e.ID(), entityID, entityType)
	saved, err := ts.associationStore.Save(ctx, assoc)
	if err != nil {
		ts.t.Fatalf("save association: %v", err)
	}
	return saved
}
