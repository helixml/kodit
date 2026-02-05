package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/api"
	apimiddleware "github.com/helixml/kodit/infrastructure/api/middleware"
	v1 "github.com/helixml/kodit/infrastructure/api/v1"
	"github.com/helixml/kodit/infrastructure/persistence"
)

// TestServer wraps the API server for e2e testing.
type TestServer struct {
	t          *testing.T
	db         persistence.Database
	httpServer *httptest.Server
	logger     *slog.Logger

	// Stores - created via persistence package, schema via auto-migrate
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

	// Services
	queueService  *service.Queue
	queryService  *service.RepositoryQuery
	syncService   *service.RepositorySync
	searchService service.CodeSearch
}

// NewTestServer creates a new test server with all dependencies wired up.
// Uses GORM auto-migrate for schema creation instead of manual SQL.
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := persistence.NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("create database: %v", err)
	}

	// Use auto-migrate instead of manual SQL
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	logger := slog.Default()

	// Create stores
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

	// Create services
	queueService := service.NewQueue(taskStore, logger)
	queryService := service.NewRepositoryQuery(repoStore, commitStore, branchStore, tagStore).
		WithFileStore(fileStore)
	syncService := service.NewRepositorySync(repoStore, queueService, logger)

	// Create search service with fakes (SQLite doesn't have vector extensions)
	fakeBM25 := &fakeBM25Store{}
	fakeVector := &fakeVectorStore{}
	searchService := service.NewCodeSearch(fakeBM25, fakeVector, snippetStore, enrichmentStore, logger)

	// Create API server
	server := api.NewServer(":0", logger)
	router := server.Router()

	// Apply middleware
	router.Use(apimiddleware.Logging(logger))
	router.Use(apimiddleware.CorrelationID)

	// Create query services
	trackingQuery := service.NewTrackingQuery(taskStatusStore, taskStore)
	enrichmentQuery := service.NewEnrichmentQuery(enrichmentStore, associationStore)

	// Register routes
	router.Route("/api/v1", func(r chi.Router) {
		reposRouter := v1.NewRepositoriesRouter(queryService, syncService, logger).
			WithTrackingQueryService(trackingQuery).
			WithEnrichmentServices(enrichmentQuery, enrichmentStore, associationStore).
			WithIndexingServices(snippetStore, fakeVector)
		r.Mount("/repositories", reposRouter.Routes())

		commitsRouter := v1.NewCommitsRouter(queryService, logger)
		r.Mount("/commits", commitsRouter.Routes())

		searchRouter := v1.NewSearchRouter(searchService, logger)
		r.Mount("/search", searchRouter.Routes())

		enrichmentsRouter := v1.NewEnrichmentsRouter(enrichmentStore, logger)
		r.Mount("/enrichments", enrichmentsRouter.Routes())

		queueRouter := v1.NewQueueRouter(queueService, taskStore, taskStatusStore, logger)
		r.Mount("/queue", queueRouter.Routes())
	})

	// Health check
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create httptest server
	httpServer := httptest.NewServer(router)

	ts := &TestServer{
		t:                t,
		db:               db,
		httpServer:       httpServer,
		logger:           logger,
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
		queueService:     queueService,
		queryService:     queryService,
		syncService:      syncService,
		searchService:    searchService,
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
	if err := ts.queueService.Enqueue(ctx, tsk); err != nil {
		ts.t.Fatalf("enqueue task: %v", err)
	}

	// Get the task back to have the ID
	tasks, err := ts.taskStore.FindAll(ctx)
	if err != nil {
		ts.t.Fatalf("find tasks: %v", err)
	}
	if len(tasks) == 0 {
		ts.t.Fatalf("no tasks found")
	}
	return tasks[len(tasks)-1]
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

// fakeBM25Store is a fake BM25 store for testing (SQLite doesn't have VectorChord).
type fakeBM25Store struct{}

func (f *fakeBM25Store) Index(_ context.Context, _ search.IndexRequest) error {
	return nil
}

func (f *fakeBM25Store) Search(_ context.Context, _ search.Request) ([]search.Result, error) {
	return []search.Result{}, nil
}

func (f *fakeBM25Store) Delete(_ context.Context, _ search.DeleteRequest) error {
	return nil
}

// fakeVectorStore is a fake vector store for testing (SQLite doesn't have VectorChord).
type fakeVectorStore struct{}

func (f *fakeVectorStore) Index(_ context.Context, _ search.IndexRequest) error {
	return nil
}

func (f *fakeVectorStore) Search(_ context.Context, _ search.Request) ([]search.Result, error) {
	return []search.Result{}, nil
}

func (f *fakeVectorStore) HasEmbedding(_ context.Context, _ string, _ snippet.EmbeddingType) (bool, error) {
	return false, nil
}

func (f *fakeVectorStore) Delete(_ context.Context, _ search.DeleteRequest) error {
	return nil
}

func (f *fakeVectorStore) EmbeddingsForSnippets(_ context.Context, _ []string) ([]snippet.EmbeddingInfo, error) {
	return []snippet.EmbeddingInfo{}, nil
}
