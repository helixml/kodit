// Package kodit provides a library for code understanding, indexing, and search.
//
// Kodit indexes Git repositories, extracts semantic code snippets using AST parsing,
// and provides hybrid search (BM25 + vector embeddings) with LLM-powered enrichments.
//
// Basic usage:
//
//	client, err := kodit.New(
//	    kodit.WithSQLite(".kodit/data.db"),
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
	"github.com/helixml/kodit/application/handler"
	commithandler "github.com/helixml/kodit/application/handler/commit"
	enrichmenthandler "github.com/helixml/kodit/application/handler/enrichment"
	indexinghandler "github.com/helixml/kodit/application/handler/indexing"
	repohandler "github.com/helixml/kodit/application/handler/repository"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/api"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	v1 "github.com/helixml/kodit/infrastructure/api/v1"
	"github.com/helixml/kodit/infrastructure/enricher"
	"github.com/helixml/kodit/infrastructure/enricher/example"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/infrastructure/persistence"
	infraSearch "github.com/helixml/kodit/infrastructure/search"
	"github.com/helixml/kodit/infrastructure/slicing"
	"github.com/helixml/kodit/infrastructure/slicing/language"
	"github.com/helixml/kodit/infrastructure/tracking"
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
	fileStore        persistence.FileStore
	snippetStore     persistence.SnippetStore
	enrichmentStore  persistence.EnrichmentStore
	associationStore persistence.AssociationStore
	taskStore        persistence.TaskStore
	statusStore      persistence.StatusStore

	// Search stores (may be nil if not configured)
	textVectorStore search.VectorStore
	codeVectorStore search.VectorStore

	// Application services
	repoSync      *service.RepositorySync
	repoQuery     *service.RepositoryQuery
	enrichQ       *service.EnrichmentQuery
	trackingQuery *service.TrackingQuery
	codeSearch    *service.CodeSearch
	queue         *service.Queue
	worker        *service.Worker
	registry      *service.Registry

	// Git infrastructure (internal)
	gitAdapter git.Adapter
	cloner     domainservice.Cloner
	scanner    domainservice.Scanner

	// Code slicing (internal)
	slicer *slicing.Slicer

	// Progress tracking (internal)
	trackerFactory handler.TrackerFactory

	// Enrichment (internal, may be nil)
	enricherImpl      *enricher.ProviderEnricher
	archDiscoverer    *enricher.PhysicalArchitectureService
	exampleDiscoverer *example.Discovery
	schemaDiscoverer  *enricher.DatabaseSchemaService
	apiDocService     *enricher.APIDocService
	cookbookContext   *enricher.CookbookContextService

	logger   *slog.Logger
	dataDir  string
	cloneDir string
	apiKeys  []string
	closed   atomic.Bool
	mu       sync.Mutex
}

// New creates a new Client with the given options.
// The background worker is started automatically.
func New(opts ...Option) (*Client, error) {
	cfg := newClientConfig()

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

	// Set up clone directory
	cloneDir := cfg.cloneDir
	if cloneDir == "" {
		cloneDir = defaultCloneDir(dataDir)
	}

	// Ensure clone directory exists
	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		return nil, fmt.Errorf("create clone directory: %w", err)
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
	fileStore := persistence.NewFileStore(db)
	snippetStore := persistence.NewSnippetStore(db)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	taskStore := persistence.NewTaskStore(db)
	statusStore := persistence.NewStatusStore(db)

	// Create search stores based on storage type
	var textVectorStore search.VectorStore
	var codeVectorStore search.VectorStore

	gormDB := db.GORM()
	switch cfg.storage {
	case storageSQLite:
		if cfg.embeddingProvider != nil {
			textVectorStore = infraSearch.NewSQLiteVectorStore(gormDB, infraSearch.TaskNameText, cfg.embeddingProvider, logger)
			codeVectorStore = infraSearch.NewSQLiteVectorStore(gormDB, infraSearch.TaskNameCode, cfg.embeddingProvider, logger)
		}
	case storagePostgres:
		// pgvector not available in plain Postgres mode
	case storagePostgresPgvector:
		if cfg.embeddingProvider != nil {
			textVectorStore = infraSearch.NewPgvectorStore(gormDB, infraSearch.TaskNameText, cfg.embeddingProvider, logger)
			codeVectorStore = infraSearch.NewPgvectorStore(gormDB, infraSearch.TaskNameCode, cfg.embeddingProvider, logger)
		}
	case storagePostgresVectorchord:
		if cfg.embeddingProvider != nil {
			textVectorStore = infraSearch.NewVectorChordVectorStore(gormDB, infraSearch.TaskNameText, cfg.embeddingProvider, logger)
			codeVectorStore = infraSearch.NewVectorChordVectorStore(gormDB, infraSearch.TaskNameCode, cfg.embeddingProvider, logger)
		}
	}

	// Create application services
	registry := service.NewRegistry()
	queue := service.NewQueue(taskStore, logger)
	worker := service.NewWorker(taskStore, registry, logger)

	repoSyncSvc := service.NewRepositorySync(repoStore, queue, logger)
	repoQuerySvc := service.NewRepositoryQuery(repoStore, commitStore, branchStore, tagStore).WithFileStore(fileStore)
	enrichQSvc := service.NewEnrichmentQuery(enrichmentStore, associationStore)
	trackingQSvc := service.NewTrackingQuery(statusStore, taskStore)

	// Create code search service if vector stores are available
	var codeSearchSvc *service.CodeSearch
	if textVectorStore != nil || codeVectorStore != nil {
		cs := service.NewCodeSearch(textVectorStore, codeVectorStore, snippetStore, enrichmentStore, logger)
		codeSearchSvc = &cs
	}

	// Create git infrastructure
	gitAdapter := git.NewGoGitAdapter(logger)
	cloner := git.NewRepositoryCloner(gitAdapter, cloneDir, logger)
	scanner := git.NewRepositoryScanner(gitAdapter, logger)

	// Create slicer for code extraction
	langConfig := slicing.NewLanguageConfig()
	analyzerFactory := language.NewFactory(langConfig)
	slicer := slicing.NewSlicer(langConfig, analyzerFactory)

	// Create tracker factory for progress reporting
	dbReporter := tracking.NewDBReporter(statusStore, logger)
	trackerFactory := &trackerFactoryImpl{
		dbReporter: dbReporter,
		logger:     logger,
	}

	// Validate required providers (unless skipped for testing)
	if !cfg.skipProviderValidation {
		if cfg.embeddingProvider == nil {
			return nil, fmt.Errorf("embedding provider is required: set EMBEDDING_ENDPOINT_BASE_URL and EMBEDDING_ENDPOINT_API_KEY environment variables")
		}
		if cfg.textProvider == nil {
			return nil, fmt.Errorf("text provider is required: set ENRICHMENT_ENDPOINT_BASE_URL and ENRICHMENT_ENDPOINT_API_KEY environment variables")
		}
		if codeVectorStore == nil {
			return nil, fmt.Errorf("vector store is required: use PostgreSQL with pgvector or VectorChord extension (DB_URL=postgres://... with pgvector or vectorchord storage type)")
		}
	}

	// Create enricher infrastructure (only if text provider is configured)
	var enricherImpl *enricher.ProviderEnricher
	if cfg.textProvider != nil {
		enricherImpl = enricher.NewProviderEnricher(cfg.textProvider, logger)
	}

	// Create enrichment infrastructure (always available)
	archDiscoverer := enricher.NewPhysicalArchitectureService()
	exampleDiscoverer := example.NewDiscovery()
	schemaDiscoverer := enricher.NewDatabaseSchemaService()
	apiDocService := enricher.NewAPIDocService()
	cookbookContext := enricher.NewCookbookContextService()

	client := &Client{
		db:                db,
		repositoryStore:   repoStore,
		commitStore:       commitStore,
		branchStore:       branchStore,
		tagStore:          tagStore,
		fileStore:         fileStore,
		snippetStore:      snippetStore,
		enrichmentStore:   enrichmentStore,
		associationStore:  associationStore,
		taskStore:         taskStore,
		statusStore:       statusStore,
		textVectorStore:   textVectorStore,
		codeVectorStore:   codeVectorStore,
		repoSync:          repoSyncSvc,
		repoQuery:         repoQuerySvc,
		enrichQ:           enrichQSvc,
		trackingQuery:     trackingQSvc,
		codeSearch:        codeSearchSvc,
		queue:             queue,
		worker:            worker,
		registry:          registry,
		gitAdapter:        gitAdapter,
		cloner:            cloner,
		scanner:           scanner,
		slicer:            slicer,
		trackerFactory:    trackerFactory,
		enricherImpl:      enricherImpl,
		archDiscoverer:    archDiscoverer,
		exampleDiscoverer: exampleDiscoverer,
		schemaDiscoverer:  schemaDiscoverer,
		apiDocService:     apiDocService,
		cookbookContext:   cookbookContext,
		logger:            logger,
		dataDir:           dataDir,
		cloneDir:          cloneDir,
		apiKeys:           cfg.apiKeys,
	}

	// Register task handlers
	client.registerHandlers()

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

// registerHandlers registers all task handlers with the worker registry.
func (c *Client) registerHandlers() {
	// Repository handlers (always registered)
	c.registry.Register(task.OperationCloneRepository, repohandler.NewClone(
		c.repositoryStore, c.cloner, c.queue, c.trackerFactory, c.logger,
	))
	c.registry.Register(task.OperationSyncRepository, repohandler.NewSync(
		c.repositoryStore, c.branchStore, c.cloner, c.scanner, c.queue, c.trackerFactory, c.logger,
	))
	c.registry.Register(task.OperationDeleteRepository, repohandler.NewDelete(
		c.repositoryStore, c.commitStore, c.branchStore, c.tagStore, c.fileStore, c.snippetStore, c.trackerFactory, c.logger,
	))
	c.registry.Register(task.OperationScanCommit, commithandler.NewScan(
		c.repositoryStore, c.commitStore, c.fileStore, c.scanner, c.trackerFactory, c.logger,
	))
	c.registry.Register(task.OperationRescanCommit, commithandler.NewRescan(
		c.snippetStore, c.associationStore, c.trackerFactory, c.logger,
	))

	// Indexing handlers (always registered for snippet extraction)
	c.registry.Register(task.OperationExtractSnippetsForCommit, indexinghandler.NewExtractSnippets(
		c.repositoryStore, c.snippetStore, c.gitAdapter, c.slicer, c.trackerFactory, c.logger,
	))

	// Code embeddings handlers (require code vector store)
	if c.codeVectorStore != nil {
		codeEmbeddingService := domainservice.NewEmbedding(c.codeVectorStore)

		// Code embeddings for snippets
		c.registry.Register(task.OperationCreateCodeEmbeddingsForCommit, indexinghandler.NewCreateCodeEmbeddings(
			codeEmbeddingService, c.snippetStore, c.codeVectorStore, c.trackerFactory, c.logger,
		))

		// Example code embeddings (enrichment content from extracted examples)
		c.registry.Register(task.OperationCreateExampleCodeEmbeddingsForCommit, indexinghandler.NewCreateExampleCodeEmbeddings(
			codeEmbeddingService, c.enrichQ, c.codeVectorStore, c.trackerFactory, c.logger,
		))
	}

	// Text embeddings handlers (require text vector store)
	if c.textVectorStore != nil {
		textEmbeddingService := domainservice.NewEmbedding(c.textVectorStore)

		// Summary embeddings (enrichment content from snippet summaries)
		c.registry.Register(task.OperationCreateSummaryEmbeddingsForCommit, indexinghandler.NewCreateSummaryEmbeddings(
			textEmbeddingService, c.enrichQ, c.associationStore, c.textVectorStore, c.trackerFactory, c.logger,
		))

		// Example summary embeddings (enrichment content from example summaries)
		c.registry.Register(task.OperationCreateExampleSummaryEmbeddingsForCommit, indexinghandler.NewCreateExampleSummaryEmbeddings(
			textEmbeddingService, c.enrichQ, c.textVectorStore, c.trackerFactory, c.logger,
		))
	}

	// Enrichment handlers (require text provider)
	if c.enricherImpl != nil {
		// Summary enrichment
		c.registry.Register(task.OperationCreateSummaryEnrichmentForCommit, enrichmenthandler.NewCreateSummary(
			c.snippetStore, c.enrichmentStore, c.associationStore, c.enrichQ, c.enricherImpl, c.trackerFactory, c.logger,
		))

		// Commit description
		c.registry.Register(task.OperationCreateCommitDescriptionForCommit, enrichmenthandler.NewCommitDescription(
			c.repositoryStore, c.enrichmentStore, c.associationStore, c.enrichQ, c.gitAdapter, c.enricherImpl, c.trackerFactory, c.logger,
		))

		// Architecture discovery
		c.registry.Register(task.OperationCreateArchitectureEnrichmentForCommit, enrichmenthandler.NewArchitectureDiscovery(
			c.repositoryStore, c.enrichmentStore, c.associationStore, c.enrichQ, c.archDiscoverer, c.enricherImpl, c.trackerFactory, c.logger,
		))

		// Example summary
		c.registry.Register(task.OperationCreateExampleSummaryForCommit, enrichmenthandler.NewExampleSummary(
			c.enrichmentStore, c.associationStore, c.enrichQ, c.enricherImpl, c.trackerFactory, c.logger,
		))

		// Database schema enrichment
		c.registry.Register(task.OperationCreateDatabaseSchemaForCommit, enrichmenthandler.NewDatabaseSchema(
			c.repositoryStore, c.enrichmentStore, c.associationStore, c.enrichQ, c.schemaDiscoverer, c.enricherImpl, c.trackerFactory, c.logger,
		))

		// Cookbook enrichment
		c.registry.Register(task.OperationCreateCookbookForCommit, enrichmenthandler.NewCookbook(
			c.repositoryStore, c.fileStore, c.enrichmentStore, c.associationStore, c.enrichQ, c.cookbookContext, c.enricherImpl, c.trackerFactory, c.logger,
		))

		// API docs enrichment
		c.registry.Register(task.OperationCreatePublicAPIDocsForCommit, enrichmenthandler.NewAPIDocs(
			c.repositoryStore, c.fileStore, c.enrichmentStore, c.associationStore, c.enrichQ, c.apiDocService, c.trackerFactory, c.logger,
		))
	}

	// Example extraction handler (doesn't require LLM, always registered)
	c.registry.Register(task.OperationExtractExamplesForCommit, enrichmenthandler.NewExtractExamples(
		c.repositoryStore, c.commitStore, c.gitAdapter, c.enrichmentStore, c.associationStore, c.enrichQ, c.exampleDiscoverer, c.trackerFactory, c.logger,
	))

	c.logger.Info("registered task handlers", slog.Int("count", len(c.registry.Operations())))
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
	searchCfg := newSearchConfig()
	for _, opt := range opts {
		opt(searchCfg)
	}

	// If no search service configured, return empty result
	if c.codeSearch == nil {
		return SearchResult{}, nil
	}

	// Build filters from options
	var filterOpts []search.FiltersOption
	if len(searchCfg.languages) > 0 && len(searchCfg.languages) == 1 {
		filterOpts = append(filterOpts, search.WithLanguage(searchCfg.languages[0]))
	}
	filters := search.NewFilters(filterOpts...)

	// Build multi-search request
	// Use query for both text (BM25) and code (vector) queries
	request := search.NewMultiRequest(searchCfg.limit, query, query, nil, filters)

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

// NewAPIServer creates an HTTP API server that uses the given Client.
// The API server provides REST endpoints for repository management, search, and more.
func NewAPIServer(client *Client) APIServer {
	return &apiServerImpl{
		client: client,
		logger: client.logger,
	}
}

// CodeSearchService returns the underlying code search service.
// This is useful for advanced callers like MCP servers that need
// full control over search requests (e.g., MultiSearchRequest).
// Returns nil if no search stores are configured.
func (c *Client) CodeSearchService() *service.CodeSearch {
	return c.codeSearch
}

// SnippetStore returns access to the snippet storage.
// This is useful for callers that need to retrieve snippets by SHA.
func (c *Client) SnippetStore() snippet.SnippetStore {
	return c.snippetStore
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
	// Router returns the chi router for customization before starting.
	// Call this first, add custom middleware with router.Use(), then call MountRoutes().
	// If not called, ListenAndServe creates a default router with all standard routes.
	Router() chi.Router

	// MountRoutes wires up all v1 API routes on the router.
	// Call this after adding any custom middleware via Router().Use().
	MountRoutes()

	// DocsRouter returns a router for Swagger UI and OpenAPI spec.
	// The specURL parameter is the URL path where the OpenAPI spec will be served.
	// Mount the returned router at your preferred path (e.g., router.Mount("/docs", api.DocsRouter("/docs/openapi.json").Routes())).
	DocsRouter(specURL string) *api.DocsRouter

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
	client       *Client
	server       *api.Server
	router       chi.Router
	routerCalled bool
	logger       *slog.Logger
}

// Router returns the chi router for customization.
// Call this first, add custom middleware with router.Use(), then call MountRoutes().
func (a *apiServerImpl) Router() chi.Router {
	if a.router != nil {
		return a.router
	}

	// Create a standalone chi router for customization
	router := chi.NewRouter()

	// Apply auth middleware if API keys configured
	if len(a.client.apiKeys) > 0 {
		router.Use(middleware.APIKeyAuth(a.client.apiKeys))
	}

	a.router = router
	a.routerCalled = true
	return router
}

// MountRoutes wires up all v1 API routes on the router.
// Call this after adding any custom middleware via Router().Use().
func (a *apiServerImpl) MountRoutes() {
	if a.router == nil {
		a.Router() // Ensure router is created
	}
	a.mountAPIRoutes(a.router)
}

// mountAPIRoutes wires up all v1 API routes on the given router.
func (a *apiServerImpl) mountAPIRoutes(router chi.Router) {
	// Repositories router
	reposRouter := v1.NewRepositoriesRouter(
		a.client.repoQuery,
		a.client.repoSync,
		a.logger,
	)
	reposRouter.WithTrackingQueryService(a.client.trackingQuery)
	reposRouter.WithEnrichmentServices(a.client.enrichQ, a.client.enrichmentStore, a.client.associationStore)
	reposRouter.WithIndexingServices(a.client.snippetStore, a.client.codeVectorStore)

	// Queue router
	queueRouter := v1.NewQueueRouter(
		a.client.queue,
		a.client.taskStore,
		a.client.statusStore,
		a.logger,
	)

	// Enrichments router
	enrichmentsRouter := v1.NewEnrichmentsRouter(a.client.enrichmentStore, a.logger)

	// Commits router
	commitsRouter := v1.NewCommitsRouter(
		a.client.repoQuery,
		a.logger,
	)

	// Mount routes
	router.Route("/api/v1", func(r chi.Router) {
		r.Mount("/repositories", reposRouter.Routes())
		r.Mount("/commits", commitsRouter.Routes())
		r.Mount("/queue", queueRouter.Routes())
		r.Mount("/enrichments", enrichmentsRouter.Routes())

		// Mount search router if search service is configured
		if a.client.codeSearch != nil {
			searchRouter := v1.NewSearchRouter(*a.client.codeSearch, a.logger)
			r.Mount("/search", searchRouter.Routes())
		}
	})
}

func (a *apiServerImpl) ListenAndServe(addr string) error {
	server := api.NewServer(addr, a.logger)
	a.server = &server

	if a.routerCalled && a.router != nil {
		// Use the pre-configured router by mounting it on the server's router
		server.Router().Mount("/", a.router)
	} else {
		// Mount routes directly on the server's router
		router := server.Router()

		// Apply auth middleware if API keys configured
		if len(a.client.apiKeys) > 0 {
			router.Use(middleware.APIKeyAuth(a.client.apiKeys))
		}

		a.mountAPIRoutes(router)
	}

	return server.Start()
}

// DocsRouter returns a router for Swagger UI and OpenAPI spec.
func (a *apiServerImpl) DocsRouter(specURL string) *api.DocsRouter {
	return api.NewDocsRouter(specURL)
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

// trackerFactoryImpl implements handler.TrackerFactory for progress reporting.
type trackerFactoryImpl struct {
	dbReporter *tracking.DBReporter
	logger     *slog.Logger
}

// ForOperation creates a Tracker for the given operation.
func (f *trackerFactoryImpl) ForOperation(operation task.Operation, trackableType task.TrackableType, trackableID int64) handler.Tracker {
	tracker := tracking.TrackerForOperation(operation, f.logger, trackableType, trackableID)
	if f.dbReporter != nil {
		tracker.Subscribe(f.dbReporter)
	}
	return tracker
}
