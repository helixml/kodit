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

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/internal/api"
	apimiddleware "github.com/helixml/kodit/internal/api/middleware"
	v1 "github.com/helixml/kodit/internal/api/v1"
	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	enrichmentpostgres "github.com/helixml/kodit/internal/enrichment/postgres"
	"github.com/helixml/kodit/internal/git"
	gitpostgres "github.com/helixml/kodit/internal/git/postgres"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/queue"
	queuepostgres "github.com/helixml/kodit/internal/queue/postgres"
	"github.com/helixml/kodit/internal/repository"
	"github.com/helixml/kodit/internal/search"
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
    commit_sha TEXT NOT NULL,
    path TEXT NOT NULL,
    blob_sha TEXT NOT NULL,
    mime_type TEXT DEFAULT '',
    extension TEXT DEFAULT '',
    size INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (commit_sha, path)
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

CREATE TABLE IF NOT EXISTS snippets_v2 (
    sha TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    extension TEXT NOT NULL,
    snippet_type TEXT NOT NULL,
    name TEXT DEFAULT '',
    language TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS commit_index (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    commit_sha TEXT NOT NULL UNIQUE,
    repo_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    snippet_count INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS commit_snippet_associations (
    commit_sha TEXT NOT NULL,
    snippet_sha TEXT NOT NULL,
    file_path TEXT NOT NULL,
    start_line INTEGER DEFAULT 0,
    end_line INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (commit_sha, snippet_sha, file_path)
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
    enrichment_id INTEGER NOT NULL,
    snippet_sha TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (enrichment_id, snippet_sha),
    FOREIGN KEY (enrichment_id) REFERENCES enrichments_v2(id)
);
`

// TestServer wraps the API server for e2e testing.
type TestServer struct {
	t          *testing.T
	db         database.Database
	httpServer *httptest.Server
	server     api.Server
	logger     *slog.Logger

	// Repositories
	repoRepo       git.RepoRepository
	commitRepo     git.CommitRepository
	branchRepo     git.BranchRepository
	tagRepo        git.TagRepository
	fileRepo       git.FileRepository
	taskRepo       queue.TaskRepository
	taskStatusRepo queue.TaskStatusRepository
	enrichmentRepo enrichment.EnrichmentRepository

	// Services
	queueService  *queue.Service
	queryService  *repository.QueryService
	syncService   *repository.SyncService
	searchService search.Service
}

// NewTestServer creates a new test server with all dependencies wired up.
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := database.NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("create database: %v", err)
	}

	// Create schema
	if err := db.Session(ctx).Exec(testSchema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}

	logger := slog.Default()

	// Create repositories
	repoRepo := gitpostgres.NewRepoRepository(db)
	commitRepo := gitpostgres.NewCommitRepository(db)
	branchRepo := gitpostgres.NewBranchRepository(db)
	tagRepo := gitpostgres.NewTagRepository(db)
	fileRepo := gitpostgres.NewFileRepository(db)
	taskRepo := queuepostgres.NewTaskRepository(db)
	taskStatusRepo := queuepostgres.NewTaskStatusRepository(db)
	enrichmentRepo := enrichmentpostgres.NewEnrichmentRepository(db)

	// Create services
	queueService := queue.NewService(taskRepo, logger)
	queryService := repository.NewQueryService(repoRepo, commitRepo, branchRepo, tagRepo)
	syncService := repository.NewSyncService(repoRepo, queueService, logger)

	// Create search service with fakes for BM25 and Vector (no extensions in SQLite)
	fakeBM25 := &fakeBM25Repository{}
	fakeVector := &fakeVectorRepository{}
	fakeSnippetRepo := &fakeSnippetRepository{}
	searchService := search.NewService(fakeBM25, fakeVector, fakeSnippetRepo, enrichmentRepo, logger)

	// Create API server
	server := api.NewServer(":0", logger)
	router := server.Router()

	// Apply middleware
	router.Use(apimiddleware.Logging(logger))
	router.Use(apimiddleware.CorrelationID)

	// Register routes
	router.Route("/api/v1", func(r chi.Router) {
		reposRouter := v1.NewRepositoriesRouter(queryService, syncService, logger)
		r.Mount("/repositories", reposRouter.Routes())

		commitsRouter := v1.NewCommitsRouter(queryService, logger)
		r.Mount("/commits", commitsRouter.Routes())

		searchRouter := v1.NewSearchRouter(searchService, logger)
		r.Mount("/search", searchRouter.Routes())

		enrichmentsRouter := v1.NewEnrichmentsRouter(enrichmentRepo, logger)
		r.Mount("/enrichments", enrichmentsRouter.Routes())

		queueRouter := v1.NewQueueRouter(taskRepo, taskStatusRepo, logger)
		r.Mount("/queue", queueRouter.Routes())
	})

	// Health check
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	// Create httptest server
	httpServer := httptest.NewServer(router)

	ts := &TestServer{
		t:              t,
		db:             db,
		httpServer:     httpServer,
		server:         server,
		logger:         logger,
		repoRepo:       repoRepo,
		commitRepo:     commitRepo,
		branchRepo:     branchRepo,
		tagRepo:        tagRepo,
		fileRepo:       fileRepo,
		taskRepo:       taskRepo,
		taskStatusRepo: taskStatusRepo,
		enrichmentRepo: enrichmentRepo,
		queueService:   queueService,
		queryService:   queryService,
		syncService:    syncService,
		searchService:  searchService,
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
func (ts *TestServer) CreateRepository(remoteURL string) git.Repo {
	ts.t.Helper()
	ctx := context.Background()

	repo, err := git.NewRepo(remoteURL)
	if err != nil {
		ts.t.Fatalf("create repo: %v", err)
	}
	saved, err := ts.repoRepo.Save(ctx, repo)
	if err != nil {
		ts.t.Fatalf("save repo: %v", err)
	}
	return saved
}

// CreateTask creates a task in the database directly.
func (ts *TestServer) CreateTask(operation queue.TaskOperation, payload map[string]any) queue.Task {
	ts.t.Helper()
	ctx := context.Background()

	task := queue.NewTask(operation, int(domain.QueuePriorityNormal), payload)
	if err := ts.queueService.Enqueue(ctx, task); err != nil {
		ts.t.Fatalf("enqueue task: %v", err)
	}

	// Get the task back to have the ID
	tasks, err := ts.taskRepo.Find(ctx, database.NewQuery())
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
	saved, err := ts.enrichmentRepo.Save(ctx, e)
	if err != nil {
		ts.t.Fatalf("save enrichment: %v", err)
	}
	return saved
}

// fakeBM25Repository is a fake BM25 repository for testing (SQLite doesn't have VectorChord).
type fakeBM25Repository struct{}

func (f *fakeBM25Repository) Index(_ context.Context, _ domain.IndexRequest) error {
	return nil
}

func (f *fakeBM25Repository) Search(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
	return []domain.SearchResult{}, nil
}

func (f *fakeBM25Repository) Delete(_ context.Context, _ domain.DeleteRequest) error {
	return nil
}

// fakeVectorRepository is a fake vector repository for testing (SQLite doesn't have VectorChord).
type fakeVectorRepository struct{}

func (f *fakeVectorRepository) Index(_ context.Context, _ domain.IndexRequest) error {
	return nil
}

func (f *fakeVectorRepository) Search(_ context.Context, _ domain.SearchRequest) ([]domain.SearchResult, error) {
	return []domain.SearchResult{}, nil
}

func (f *fakeVectorRepository) HasEmbedding(_ context.Context, _ string, _ indexing.EmbeddingType) (bool, error) {
	return false, nil
}

func (f *fakeVectorRepository) Delete(_ context.Context, _ domain.DeleteRequest) error {
	return nil
}

// fakeSnippetRepository is a fake snippet repository for testing.
type fakeSnippetRepository struct{}

func (f *fakeSnippetRepository) Save(_ context.Context, _ string, _ []indexing.Snippet) error {
	return nil
}

func (f *fakeSnippetRepository) SnippetsForCommit(_ context.Context, _ string) ([]indexing.Snippet, error) {
	return []indexing.Snippet{}, nil
}

func (f *fakeSnippetRepository) DeleteForCommit(_ context.Context, _ string) error {
	return nil
}

func (f *fakeSnippetRepository) Search(_ context.Context, _ domain.MultiSearchRequest) ([]indexing.Snippet, error) {
	return []indexing.Snippet{}, nil
}

func (f *fakeSnippetRepository) ByIDs(_ context.Context, _ []string) ([]indexing.Snippet, error) {
	return []indexing.Snippet{}, nil
}
