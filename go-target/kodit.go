// Package kodit provides a library for code understanding, indexing, and search.
//
// Kodit indexes Git repositories, extracts semantic code snippets using AST parsing,
// and provides hybrid search (BM25 + vector embeddings) with LLM-powered enrichments.
//
// Basic usage:
//
//	client, err := kodit.New(
//	    kodit.WithSQLite("~/.kodit/data.db"),
//	    kodit.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Index a repository
//	repo, err := client.Repositories().Clone(ctx, "https://github.com/kubernetes/kubernetes")
//
//	// Hybrid search
//	results, err := client.Search(ctx, "create a deployment",
//	    kodit.WithSemanticWeight(0.7),
//	    kodit.WithLimit(10),
//	)
//
//	// Iterate results
//	for _, snippet := range results.Snippets() {
//	    fmt.Println(snippet.Path(), snippet.Name())
//	}
package kodit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/api"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	v1 "github.com/helixml/kodit/infrastructure/api/v1"
	"github.com/helixml/kodit/infrastructure/persistence"
	infraSearch "github.com/helixml/kodit/infrastructure/search"
)

// Client is the main entry point for the kodit library.
// The background worker starts automatically on creation.
type Client struct {
	db persistence.Database

	// Domain stores
	repositoryStore  persistence.RepositoryStore
	commitStore      persistence.CommitStore
	branchStore      persistence.BranchStore
	tagStore         persistence.TagStore
	snippetStore     persistence.SnippetStore
	enrichmentStore  persistence.EnrichmentStore
	associationStore persistence.AssociationStore
	taskStore        persistence.TaskStore
	statusStore      persistence.StatusStore

	// Search stores (may be nil if not configured)
	bm25Store   search.BM25Store
	vectorStore search.VectorStore

	// Application services
	repoSync      *service.RepositorySync
	repoQuery     *service.RepositoryQuery
	enrichQ       *service.EnrichmentQuery
	trackingQuery *service.TrackingQuery
	codeSearch    *service.CodeSearch
	queue         *service.Queue
	worker        *service.Worker
	registry      *service.Registry

	logger  *slog.Logger
	dataDir string
	apiKeys []string
	closed  atomic.Bool
	mu      sync.Mutex
}

// New creates a new Client with the given options.
// The background worker is started automatically.
func New(opts ...Option) (*Client, error) {
	cfg := &clientConfig{
		workerCount: 1,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.storage == storageUnset {
		return nil, ErrNoStorage
	}

	// Set up logger
	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}

	// Set up data directory
	dataDir := cfg.dataDir
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			dataDir = ".kodit"
		} else {
			dataDir = filepath.Join(home, ".kodit")
		}
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	// Build database URL
	ctx := context.Background()
	dbURL, err := buildDatabaseURL(cfg)
	if err != nil {
		return nil, fmt.Errorf("build database url: %w", err)
	}

	// Open database
	db, err := persistence.NewDatabase(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Run auto migration
	if err := db.AutoMigrate(); err != nil {
		errClose := db.Close()
		return nil, errors.Join(fmt.Errorf("auto migrate: %w", err), errClose)
	}

	// Create stores
	repoStore := persistence.NewRepositoryStore(db)
	commitStore := persistence.NewCommitStore(db)
	branchStore := persistence.NewBranchStore(db)
	tagStore := persistence.NewTagStore(db)
	snippetStore := persistence.NewSnippetStore(db)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	taskStore := persistence.NewTaskStore(db)
	statusStore := persistence.NewStatusStore(db)

	// Create search stores based on storage type
	var bm25Store search.BM25Store
	var vectorStore search.VectorStore

	gormDB := db.GORM()
	switch cfg.storage {
	case storageSQLite:
		bm25Store = infraSearch.NewSQLiteBM25Store(gormDB, logger)
		// SQLite vector search not implemented - would require external library
	case storagePostgres:
		bm25Store = infraSearch.NewPostgresBM25Store(gormDB, logger)
		// pgvector not available in plain Postgres mode
	case storagePostgresPgvector:
		bm25Store = infraSearch.NewPostgresBM25Store(gormDB, logger)
		if cfg.embeddingProvider != nil {
			vectorStore = infraSearch.NewPgvectorStore(gormDB, infraSearch.TaskNameCode, cfg.embeddingProvider, logger)
		}
	case storagePostgresVectorchord:
		bm25Store = infraSearch.NewVectorChordBM25Store(gormDB, logger)
		if cfg.embeddingProvider != nil {
			vectorStore = infraSearch.NewVectorChordVectorStore(gormDB, infraSearch.TaskNameCode, cfg.embeddingProvider, logger)
		}
	}

	// Create application services
	registry := service.NewRegistry()
	queue := service.NewQueue(taskStore, logger)
	worker := service.NewWorker(taskStore, registry, logger)

	repoSyncSvc := service.NewRepositorySync(repoStore, queue, logger)
	repoQuerySvc := service.NewRepositoryQuery(repoStore, commitStore, branchStore, tagStore)
	enrichQSvc := service.NewEnrichmentQuery(enrichmentStore, associationStore)
	trackingQSvc := service.NewTrackingQuery(statusStore, taskStore)

	// Create code search service if search stores are available
	var codeSearchSvc *service.CodeSearch
	if bm25Store != nil || vectorStore != nil {
		cs := service.NewCodeSearch(bm25Store, vectorStore, snippetStore, enrichmentStore, logger)
		codeSearchSvc = &cs
	}

	client := &Client{
		db:               db,
		repositoryStore:  repoStore,
		commitStore:      commitStore,
		branchStore:      branchStore,
		tagStore:         tagStore,
		snippetStore:     snippetStore,
		enrichmentStore:  enrichmentStore,
		associationStore: associationStore,
		taskStore:        taskStore,
		statusStore:      statusStore,
		bm25Store:        bm25Store,
		vectorStore:      vectorStore,
		repoSync:         repoSyncSvc,
		repoQuery:        repoQuerySvc,
		enrichQ:          enrichQSvc,
		trackingQuery:    trackingQSvc,
		codeSearch:       codeSearchSvc,
		queue:            queue,
		worker:           worker,
		registry:         registry,
		logger:           logger,
		dataDir:          dataDir,
		apiKeys:          cfg.apiKeys,
	}

	// Start the background worker
	worker.Start(ctx)

	return client, nil
}

// Close releases all resources and stops the background worker.
func (c *Client) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return ErrClientClosed
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop the worker
	c.worker.Stop()

	// Close the database
	if err := c.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	c.logger.Info("kodit client closed")
	return nil
}

// Repositories returns the repository management interface.
func (c *Client) Repositories() Repositories {
	return &repositoriesImpl{
		repoSync:  c.repoSync,
		repoQuery: c.repoQuery,
	}
}

// Search performs a hybrid code search.
// Returns empty results if search infrastructure is not configured (no embedding provider).
func (c *Client) Search(ctx context.Context, query string, opts ...SearchOption) (SearchResult, error) {
	if c.closed.Load() {
		return SearchResult{}, ErrClientClosed
	}

	// Apply search options
	cfg := &searchConfig{
		limit: 10,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// If no search service configured, return empty result
	if c.codeSearch == nil {
		return SearchResult{}, nil
	}

	// Build filters from options
	var filterOpts []search.FiltersOption
	if len(cfg.languages) > 0 && len(cfg.languages) == 1 {
		filterOpts = append(filterOpts, search.WithLanguage(cfg.languages[0]))
	}
	filters := search.NewFilters(filterOpts...)

	// Build multi-search request
	// Use query for both text (BM25) and code (vector) queries
	request := search.NewMultiRequest(cfg.limit, query, query, nil, filters)

	result, err := c.codeSearch.Search(ctx, request)
	if err != nil {
		return SearchResult{}, err
	}

	return SearchResult{
		snippets:    result.Snippets(),
		enrichments: result.Enrichments(),
		scores:      result.FusedScores(),
	}, nil
}

// Enrichments returns the enrichment query interface.
func (c *Client) Enrichments() Enrichments {
	return &enrichmentsImpl{
		enrichQ:         c.enrichQ,
		enrichmentStore: c.enrichmentStore,
	}
}

// Tasks returns the task queue interface.
func (c *Client) Tasks() Tasks {
	return &tasksImpl{
		queue:     c.queue,
		taskStore: c.taskStore,
	}
}

// API returns an HTTP server that can be started.
func (c *Client) API() APIServer {
	return &apiServerImpl{
		client: c,
		logger: c.logger,
	}
}

// SearchResult represents the result of a hybrid search.
type SearchResult struct {
	snippets    []snippet.Snippet
	enrichments []enrichment.Enrichment
	scores      map[string]float64
}

// Snippets returns the matched code snippets.
func (r SearchResult) Snippets() []snippet.Snippet {
	result := make([]snippet.Snippet, len(r.snippets))
	copy(result, r.snippets)
	return result
}

// Enrichments returns the enrichments associated with matched snippets.
func (r SearchResult) Enrichments() []enrichment.Enrichment {
	result := make([]enrichment.Enrichment, len(r.enrichments))
	copy(result, r.enrichments)
	return result
}

// Scores returns a map of snippet SHA to fused search score.
func (r SearchResult) Scores() map[string]float64 {
	result := make(map[string]float64, len(r.scores))
	for k, v := range r.scores {
		result[k] = v
	}
	return result
}

// Count returns the number of snippets in the result.
func (r SearchResult) Count() int {
	return len(r.snippets)
}

// Repositories provides repository management operations.
type Repositories interface {
	// Clone clones a repository and queues it for indexing.
	Clone(ctx context.Context, url string) (repository.Repository, error)

	// Get retrieves a repository by ID.
	Get(ctx context.Context, id int64) (repository.Repository, error)

	// List returns all repositories.
	List(ctx context.Context) ([]repository.Repository, error)

	// Delete removes a repository and all associated data.
	Delete(ctx context.Context, id int64) error

	// Sync triggers re-indexing of a repository.
	Sync(ctx context.Context, id int64) error
}

// Enrichments provides enrichment query operations.
type Enrichments interface {
	// ForCommit returns enrichments for a specific commit.
	ForCommit(ctx context.Context, commitSHA string, opts ...EnrichmentOption) ([]enrichment.Enrichment, error)

	// Get retrieves a specific enrichment by ID.
	Get(ctx context.Context, id int64) (enrichment.Enrichment, error)
}

// Tasks provides task queue operations.
type Tasks interface {
	// List returns pending tasks.
	List(ctx context.Context, opts ...TaskOption) ([]task.Task, error)

	// Get retrieves a task by ID.
	Get(ctx context.Context, id int64) (task.Task, error)

	// Cancel cancels a pending task.
	Cancel(ctx context.Context, id int64) error
}

// APIServer is an HTTP server.
type APIServer interface {
	// ListenAndServe starts the HTTP server.
	ListenAndServe(addr string) error

	// Shutdown gracefully shuts down the server.
	Shutdown(ctx context.Context) error
}

// repositoriesImpl implements Repositories.
type repositoriesImpl struct {
	repoSync  *service.RepositorySync
	repoQuery *service.RepositoryQuery
}

func (r *repositoriesImpl) Clone(ctx context.Context, url string) (repository.Repository, error) {
	source, err := r.repoSync.AddRepository(ctx, url)
	if err != nil {
		return repository.Repository{}, err
	}
	return source.Repository(), nil
}

func (r *repositoriesImpl) Get(ctx context.Context, id int64) (repository.Repository, error) {
	source, err := r.repoQuery.ByID(ctx, id)
	if err != nil {
		return repository.Repository{}, err
	}
	return source.Repository(), nil
}

func (r *repositoriesImpl) List(ctx context.Context) ([]repository.Repository, error) {
	sources, err := r.repoQuery.All(ctx)
	if err != nil {
		return nil, err
	}
	repos := make([]repository.Repository, len(sources))
	for i, src := range sources {
		repos[i] = src.Repository()
	}
	return repos, nil
}

func (r *repositoriesImpl) Delete(ctx context.Context, id int64) error {
	return r.repoSync.RequestDelete(ctx, id)
}

func (r *repositoriesImpl) Sync(ctx context.Context, id int64) error {
	return r.repoSync.RequestSync(ctx, id)
}

// enrichmentsImpl implements Enrichments.
type enrichmentsImpl struct {
	enrichQ         *service.EnrichmentQuery
	enrichmentStore persistence.EnrichmentStore
}

func (e *enrichmentsImpl) ForCommit(ctx context.Context, commitSHA string, _ ...EnrichmentOption) ([]enrichment.Enrichment, error) {
	return e.enrichQ.EnrichmentsForCommit(ctx, commitSHA, nil, nil)
}

func (e *enrichmentsImpl) Get(ctx context.Context, id int64) (enrichment.Enrichment, error) {
	return e.enrichmentStore.Get(ctx, id)
}

// tasksImpl implements Tasks.
type tasksImpl struct {
	queue     *service.Queue
	taskStore persistence.TaskStore
}

func (t *tasksImpl) List(ctx context.Context, _ ...TaskOption) ([]task.Task, error) {
	return t.queue.List(ctx, nil)
}

func (t *tasksImpl) Get(ctx context.Context, id int64) (task.Task, error) {
	return t.taskStore.Get(ctx, id)
}

func (t *tasksImpl) Cancel(ctx context.Context, id int64) error {
	tsk, err := t.taskStore.Get(ctx, id)
	if err != nil {
		return err
	}
	return t.taskStore.Delete(ctx, tsk)
}

// apiServerImpl implements APIServer.
type apiServerImpl struct {
	client *Client
	server *api.Server
	logger *slog.Logger
}

func (a *apiServerImpl) ListenAndServe(addr string) error {
	server := api.NewServer(addr, a.logger)
	a.server = &server

	router := server.Router()

	// Apply auth middleware if API keys configured
	if len(a.client.apiKeys) > 0 {
		router.Use(middleware.APIKeyAuth(a.client.apiKeys))
	}

	// Repositories router
	reposRouter := v1.NewRepositoriesRouter(
		a.client.repoQuery,
		a.client.repoSync,
		a.logger,
	)
	reposRouter.WithTrackingQueryService(a.client.trackingQuery)
	reposRouter.WithEnrichmentServices(a.client.enrichQ, a.client.enrichmentStore)
	reposRouter.WithIndexingServices(a.client.snippetStore, a.client.vectorStore)

	// Queue router
	queueRouter := v1.NewQueueRouter(
		a.client.queue,
		a.client.taskStore,
		a.client.statusStore,
		a.logger,
	)

	// Enrichments router
	enrichmentsRouter := v1.NewEnrichmentsRouter(a.client.enrichmentStore, a.logger)

	// Mount routes
	router.Route("/api/v1", func(r chi.Router) {
		r.Mount("/repositories", reposRouter.Routes())
		r.Mount("/queue", queueRouter.Routes())
		r.Mount("/enrichments", enrichmentsRouter.Routes())

		// Mount search router if search service is configured
		if a.client.codeSearch != nil {
			searchRouter := v1.NewSearchRouter(*a.client.codeSearch, a.logger)
			r.Mount("/search", searchRouter.Routes())
		}
	})

	return server.Start()
}

func (a *apiServerImpl) Shutdown(ctx context.Context) error {
	if a.server == nil {
		return nil
	}
	return a.server.Shutdown(ctx)
}

// buildDatabaseURL constructs the database URL from configuration.
func buildDatabaseURL(cfg *clientConfig) (string, error) {
	switch cfg.storage {
	case storageSQLite:
		return "sqlite:///" + cfg.dbPath, nil
	case storagePostgres, storagePostgresPgvector, storagePostgresVectorchord:
		return cfg.dbDSN, nil
	default:
		return "", ErrNoStorage
	}
}
