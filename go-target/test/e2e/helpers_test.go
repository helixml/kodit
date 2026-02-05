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
	domainrepo "github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/api"
	apimiddleware "github.com/helixml/kodit/infrastructure/api/middleware"
	v1 "github.com/helixml/kodit/infrastructure/api/v1"
	"github.com/helixml/kodit/infrastructure/persistence"
)

// testSchema contains the SQL to create all tables for e2e tests.
const testSchema = `
CREATE TABLE IF NOT EXISTS git_repos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sanitized_remote_uri TEXT NOT NULL UNIQUE,
    remote_uri TEXT NOT NULL,
    cloned_path TEXT,
    last_scanned_at DATETIME,
    num_commits INTEGER DEFAULT 0,
    num_branches INTEGER DEFAULT 0,
    num_tags INTEGER DEFAULT 0,
    tracking_type TEXT DEFAULT '',
    tracking_name TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS git_commits (
    commit_sha TEXT PRIMARY KEY,
    repo_id INTEGER NOT NULL,
    date DATETIME NOT NULL,
    message TEXT,
    parent_commit_sha TEXT,
    author TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (repo_id) REFERENCES git_repos(id)
);

CREATE TABLE IF NOT EXISTS git_branches (
    repo_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    head_commit_sha TEXT NOT NULL,
    is_default INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (repo_id, name),
    FOREIGN KEY (repo_id) REFERENCES git_repos(id)
);

CREATE TABLE IF NOT EXISTS git_tags (
    repo_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    target_commit_sha TEXT NOT NULL,
    message TEXT,
    tagger_name TEXT,
    tagger_email TEXT,
    tagged_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (repo_id, name),
    FOREIGN KEY (repo_id) REFERENCES git_repos(id)
);

CREATE TABLE IF NOT EXISTS git_commit_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    commit_sha TEXT NOT NULL,
    path TEXT NOT NULL,
    blob_sha TEXT NOT NULL,
    mime_type TEXT DEFAULT '',
    extension TEXT DEFAULT '',
    size INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (commit_sha, path)
);

CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dedup_key TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    payload TEXT,
    priority INTEGER NOT NULL DEFAULT 100,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS task_status (
    id TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    operation TEXT NOT NULL,
    trackable_id INTEGER,
    trackable_type TEXT,
    parent TEXT,
    message TEXT DEFAULT '',
    state TEXT DEFAULT '',
    error TEXT DEFAULT '',
    total INTEGER DEFAULT 0,
    current INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS snippets (
    sha TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    extension TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS snippet_commit_associations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snippet_sha TEXT NOT NULL,
    commit_sha TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS snippet_file_derivations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snippet_sha TEXT NOT NULL,
    file_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS commit_index (
    commit_sha TEXT PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'pending',
    indexed_at DATETIME,
    error_message TEXT,
    files_processed INTEGER DEFAULT 0,
    processing_time_seconds REAL DEFAULT 0.0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS enrichments_v2 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    subtype TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS enrichment_associations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    enrichment_id INTEGER NOT NULL,
    entity_type TEXT NOT NULL DEFAULT '',
    entity_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (enrichment_id) REFERENCES enrichments_v2(id)
);
`

// TestServer wraps the API server for e2e testing.
type TestServer struct {
	t          *testing.T
	db         persistence.Database
	httpServer *httptest.Server
	server     api.Server
	logger     *slog.Logger

	// Stores
	repoStore       domainrepo.RepositoryStore
	commitStore     domainrepo.CommitStore
	branchStore     domainrepo.BranchStore
	tagStore        domainrepo.TagStore
	fileStore       domainrepo.FileStore
	taskStore       task.TaskStore
	taskStatusStore task.StatusStore
	enrichmentStore enrichment.EnrichmentStore

	// Services
	queueService  *service.Queue
	queryService  *service.RepositoryQuery
	syncService   *service.RepositorySync
	searchService service.CodeSearch
}

// NewTestServer creates a new test server with all dependencies wired up.
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

	// Create schema
	if err := db.Session(ctx).Exec(testSchema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
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
	snippetStore := persistence.NewSnippetStore(db)

	// Create services
	queueService := service.NewQueue(taskStore, logger)
	queryService := service.NewRepositoryQuery(repoStore, commitStore, branchStore, tagStore).
		WithFileStore(fileStore)
	syncService := service.NewRepositorySync(repoStore, queueService, logger)

	// Create search service with fakes for BM25 and Vector (no extensions in SQLite)
	fakeBM25 := &fakeBM25Store{}
	fakeVector := &fakeVectorStore{}
	searchService := service.NewCodeSearch(fakeBM25, fakeVector, snippetStore, enrichmentStore, logger)

	// Create API server
	server := api.NewServer(":0", logger)
	router := server.Router()

	// Apply middleware
	router.Use(apimiddleware.Logging(logger))
	router.Use(apimiddleware.CorrelationID)

	// Create association store for enrichment query
	associationStore := persistence.NewAssociationStore(db)

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

	// Health check (matches Python API /healthz endpoint)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create httptest server
	httpServer := httptest.NewServer(router)

	ts := &TestServer{
		t:               t,
		db:              db,
		httpServer:      httpServer,
		server:          server,
		logger:          logger,
		repoStore:       repoStore,
		commitStore:     commitStore,
		branchStore:     branchStore,
		tagStore:        tagStore,
		fileStore:       fileStore,
		taskStore:       taskStore,
		taskStatusStore: taskStatusStore,
		enrichmentStore: enrichmentStore,
		queueService:    queueService,
		queryService:    queryService,
		syncService:     syncService,
		searchService:   searchService,
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
func (ts *TestServer) POST(path string, body interface{}) *http.Response {
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
func (ts *TestServer) DecodeJSON(resp *http.Response, v interface{}) {
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
func (ts *TestServer) CreateRepository(remoteURL string) domainrepo.Repository {
	ts.t.Helper()
	ctx := context.Background()

	repo, err := domainrepo.NewRepository(remoteURL)
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
func (ts *TestServer) CreateCommit(repo domainrepo.Repository, sha, message string) domainrepo.Commit {
	ts.t.Helper()
	ctx := context.Background()

	author := domainrepo.NewAuthor("Test User", "test@example.com")
	now := time.Now()
	commit := domainrepo.NewCommit(sha, repo.ID(), message, author, author, now, now)
	saved, err := ts.commitStore.Save(ctx, commit)
	if err != nil {
		ts.t.Fatalf("save commit: %v", err)
	}
	return saved
}

// CreateFile creates a file in the database directly.
func (ts *TestServer) CreateFile(commitSHA, path, blobSHA, mimeType, extension string, size int64) domainrepo.File {
	ts.t.Helper()
	ctx := context.Background()

	file := domainrepo.NewFileWithDetails(commitSHA, path, blobSHA, mimeType, extension, size)
	saved, err := ts.fileStore.Save(ctx, file)
	if err != nil {
		ts.t.Fatalf("save file: %v", err)
	}
	return saved
}

// CreateSnippet creates a snippet in the database directly.
func (ts *TestServer) CreateSnippet(sha, content, extension string) snippet.Snippet {
	ts.t.Helper()
	ctx := context.Background()

	snip := snippet.NewSnippet(content, extension, nil)
	// Create with specific SHA by saving directly
	if err := ts.db.Session(ctx).Exec(
		"INSERT INTO snippets (sha, content, extension, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		sha, content, extension, time.Now(), time.Now(),
	).Error; err != nil {
		ts.t.Fatalf("save snippet: %v", err)
	}
	return snip
}

// CreateSnippetAssociation links a snippet to a commit.
func (ts *TestServer) CreateSnippetAssociation(snippetSHA, commitSHA string) {
	ts.t.Helper()
	ctx := context.Background()

	if err := ts.db.Session(ctx).Exec(
		"INSERT INTO snippet_commit_associations (snippet_sha, commit_sha, created_at) VALUES (?, ?, ?)",
		snippetSHA, commitSHA, time.Now(),
	).Error; err != nil {
		ts.t.Fatalf("save snippet association: %v", err)
	}
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
