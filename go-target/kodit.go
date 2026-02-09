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
	"sync"
	"sync/atomic"

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
	"github.com/helixml/kodit/infrastructure/enricher"
	"github.com/helixml/kodit/infrastructure/enricher/example"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/infrastructure/persistence"
	infraSearch "github.com/helixml/kodit/infrastructure/search"
	"github.com/helixml/kodit/infrastructure/slicing"
	"github.com/helixml/kodit/infrastructure/slicing/language"
	"github.com/helixml/kodit/infrastructure/tracking"
	"github.com/helixml/kodit/internal/config"
)

// Client is the main entry point for the kodit library.
// The background worker starts automatically on creation.
type Client struct {
	db         persistence.Database
	repoStores RepositoryStores

	// Stores not grouped into aggregates
	snippetStore snippet.SnippetStore
	taskStore    persistence.TaskStore
	statusStore  persistence.StatusStore
	bm25Store    search.BM25Store

	// Aggregate dependencies
	enrichCtx EnrichmentContext
	codeIndex VectorIndex
	textIndex VectorIndex
	gitInfra  GitInfrastructure

	// Application services
	repoSync      *service.RepositorySync
	repoQuery     *service.RepositoryQuery
	trackingQuery *service.TrackingQuery
	codeSearch    *service.CodeSearch
	bm25Service   *domainservice.BM25
	queue         *service.Queue
	worker        *service.Worker
	registry      *service.Registry

	// Code slicing (internal)
	slicer *slicing.Slicer

	// Discovery services (each used by exactly one handler)
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

	if cfg.database == databaseUnset {
		return nil, ErrNoDatabase
	}

	// Set up logger
	logger := cfg.logger
	if logger == nil {
		logger = config.DefaultLogger()
	}

	// Set up data directory
	dataDir, err := config.PrepareDataDir(cfg.dataDir)
	if err != nil {
		return nil, err
	}

	// Set up clone directory
	cloneDir, err := config.PrepareCloneDir(cfg.cloneDir, dataDir)
	if err != nil {
		return nil, err
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

	// Group repository stores
	repoStores := RepositoryStores{
		Repositories: repoStore,
		Commits:      commitStore,
		Branches:     branchStore,
		Tags:         tagStore,
		Files:        fileStore,
	}

	// Create search stores based on storage type
	var textVectorStore search.VectorStore
	var codeVectorStore search.VectorStore
	var bm25Store search.BM25Store

	switch cfg.database {
	case databaseSQLite:
		bm25Store = infraSearch.NewSQLiteBM25Store(db.GORM(), logger)
		if cfg.embeddingProvider != nil {
			textVectorStore = infraSearch.NewSQLiteVectorStore(db.GORM(), infraSearch.TaskNameText, cfg.embeddingProvider, logger)
			codeVectorStore = infraSearch.NewSQLiteVectorStore(db.GORM(), infraSearch.TaskNameCode, cfg.embeddingProvider, logger)
		}
	case databasePostgres:
		bm25Store = infraSearch.NewPostgresBM25Store(db.GORM(), logger)
	case databasePostgresPgvector:
		bm25Store = infraSearch.NewPostgresBM25Store(db.GORM(), logger)
		if cfg.embeddingProvider != nil {
			textVectorStore = infraSearch.NewPgvectorStore(db.GORM(), infraSearch.TaskNameText, cfg.embeddingProvider, logger)
			codeVectorStore = infraSearch.NewPgvectorStore(db.GORM(), infraSearch.TaskNameCode, cfg.embeddingProvider, logger)
		}
	case databasePostgresVectorchord:
		bm25Store = infraSearch.NewVectorChordBM25Store(db.GORM(), logger)
		if cfg.embeddingProvider != nil {
			textVectorStore = infraSearch.NewVectorChordVectorStore(db.GORM(), infraSearch.TaskNameText, cfg.embeddingProvider, logger)
			codeVectorStore = infraSearch.NewVectorChordVectorStore(db.GORM(), infraSearch.TaskNameCode, cfg.embeddingProvider, logger)
		}
	default:
		return nil, errors.New("unsupported database type")
	}

	// Create vector indices (pairing embedding services with their stores)
	codeIndex := VectorIndex{
		Embedding: domainservice.NewEmbedding(codeVectorStore),
		Store:     codeVectorStore,
	}
	textIndex := VectorIndex{
		Embedding: domainservice.NewEmbedding(textVectorStore),
		Store:     textVectorStore,
	}

	// Create application services
	registry := service.NewRegistry()
	queue := service.NewQueue(taskStore, logger)
	worker := service.NewWorker(taskStore, registry, logger)

	repoSyncSvc := service.NewRepositorySync(repoStore, queue, logger)
	repoQuerySvc := service.NewRepositoryQuery(repoStore, commitStore, branchStore, tagStore).WithFileStore(fileStore)
	enrichQSvc := service.NewEnrichmentQuery(enrichmentStore, associationStore)
	trackingQSvc := service.NewTrackingQuery(statusStore, taskStore)

	// Create BM25 service for keyword search (always available)
	bm25Svc := domainservice.NewBM25(bm25Store)

	// Create code search service if vector stores are available
	codeSearchSvc := service.NewCodeSearch(textVectorStore, codeVectorStore, snippetStore, enrichmentStore, logger)

	// Create git infrastructure
	gitAdapter := git.NewGoGitAdapter(logger)
	clonerSvc := git.NewRepositoryCloner(gitAdapter, cloneDir, logger)
	scannerSvc := git.NewRepositoryScanner(gitAdapter, logger)

	gitInfra := GitInfrastructure{
		Adapter: gitAdapter,
		Cloner:  clonerSvc,
		Scanner: scannerSvc,
	}

	// Create slicer for code extraction
	langConfig := slicing.NewLanguageConfig()
	analyzerFactory := language.NewFactory(langConfig)
	slicer := slicing.NewSlicer(langConfig, analyzerFactory)

	// Create tracker factory for progress reporting
	reporters := []tracking.Reporter{
		tracking.NewDBReporter(statusStore, logger),
		tracking.NewLoggingReporter(logger),
	}
	trackerFactory := &trackerFactoryImpl{
		reporters: reporters,
		logger:    logger,
	}

	// Create enricher infrastructure (only if text provider is configured)
	var enricherImpl domainservice.Enricher
	if cfg.textProvider != nil {
		enricherImpl = enricher.NewProviderEnricher(cfg.textProvider, logger)
	}

	// Build enrichment context
	enrichCtx := EnrichmentContext{
		Enrichments:  enrichmentStore,
		Associations: associationStore,
		Query:        enrichQSvc,
		Enricher:     enricherImpl,
		Tracker:      trackerFactory,
		Logger:       logger,
	}

	// Create enrichment infrastructure (always available)
	archDiscoverer := enricher.NewPhysicalArchitectureService()
	exampleDiscoverer := example.NewDiscovery()
	schemaDiscoverer := enricher.NewDatabaseSchemaService()
	apiDocService := enricher.NewAPIDocService()
	cookbookContext := enricher.NewCookbookContextService()

	client := &Client{
		db:                db,
		repoStores:        repoStores,
		snippetStore:      snippetStore,
		taskStore:         taskStore,
		statusStore:       statusStore,
		bm25Store:         bm25Store,
		enrichCtx:         enrichCtx,
		codeIndex:         codeIndex,
		textIndex:         textIndex,
		gitInfra:          gitInfra,
		repoSync:          repoSyncSvc,
		repoQuery:         repoQuerySvc,
		trackingQuery:     trackingQSvc,
		codeSearch:        codeSearchSvc,
		bm25Service:       bm25Svc,
		queue:             queue,
		worker:            worker,
		registry:          registry,
		slicer:            slicer,
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
		c.repoStores.Repositories, c.gitInfra.Cloner, c.queue, c.enrichCtx.Tracker, c.logger,
	))
	c.registry.Register(task.OperationSyncRepository, repohandler.NewSync(
		c.repoStores.Repositories, c.repoStores.Branches, c.gitInfra.Cloner, c.gitInfra.Scanner, c.queue, c.enrichCtx.Tracker, c.logger,
	))
	c.registry.Register(task.OperationDeleteRepository, repohandler.NewDelete(
		c.repoStores, c.snippetStore, c.enrichCtx.Tracker, c.logger,
	))
	c.registry.Register(task.OperationScanCommit, commithandler.NewScan(
		c.repoStores.Repositories, c.repoStores.Commits, c.repoStores.Files, c.gitInfra.Scanner, c.enrichCtx.Tracker, c.logger,
	))
	c.registry.Register(task.OperationRescanCommit, commithandler.NewRescan(
		c.snippetStore, c.enrichCtx.Associations, c.enrichCtx.Tracker, c.logger,
	))

	// Indexing handlers (always registered for snippet extraction)
	c.registry.Register(task.OperationExtractSnippetsForCommit, indexinghandler.NewExtractSnippets(
		c.repoStores.Repositories, c.snippetStore, c.repoStores.Files, c.slicer, c.enrichCtx.Tracker, c.logger,
	))

	// BM25 index handler
	c.registry.Register(task.OperationCreateBM25IndexForCommit, indexinghandler.NewCreateBM25Index(
		c.bm25Service, c.snippetStore, c.enrichCtx.Tracker, c.logger,
	))

	// Code embeddings for snippets
	c.registry.Register(task.OperationCreateCodeEmbeddingsForCommit, indexinghandler.NewCreateCodeEmbeddings(
		c.codeIndex, c.snippetStore, c.enrichCtx.Tracker, c.logger,
	))

	// Example code embeddings (enrichment content from extracted examples)
	c.registry.Register(task.OperationCreateExampleCodeEmbeddingsForCommit, indexinghandler.NewCreateExampleCodeEmbeddings(
		c.codeIndex, c.enrichCtx.Query, c.enrichCtx.Tracker, c.logger,
	))

	// Summary embeddings (enrichment content from snippet summaries)
	c.registry.Register(task.OperationCreateSummaryEmbeddingsForCommit, indexinghandler.NewCreateSummaryEmbeddings(
		c.textIndex, c.enrichCtx.Query, c.enrichCtx.Associations, c.enrichCtx.Tracker, c.logger,
	))

	// Example summary embeddings (enrichment content from example summaries)
	c.registry.Register(task.OperationCreateExampleSummaryEmbeddingsForCommit, indexinghandler.NewCreateExampleSummaryEmbeddings(
		c.textIndex, c.enrichCtx.Query, c.enrichCtx.Tracker, c.logger,
	))

	// Enrichment handlers
	// Summary enrichment
	c.registry.Register(task.OperationCreateSummaryEnrichmentForCommit, enrichmenthandler.NewCreateSummary(
		c.snippetStore, c.enrichCtx,
	))

	// Commit description
	c.registry.Register(task.OperationCreateCommitDescriptionForCommit, enrichmenthandler.NewCommitDescription(
		c.repoStores.Repositories, c.enrichCtx, c.gitInfra.Adapter,
	))

	// Architecture discovery
	c.registry.Register(task.OperationCreateArchitectureEnrichmentForCommit, enrichmenthandler.NewArchitectureDiscovery(
		c.repoStores.Repositories, c.enrichCtx, c.archDiscoverer,
	))

	// Example summary
	c.registry.Register(task.OperationCreateExampleSummaryForCommit, enrichmenthandler.NewExampleSummary(
		c.enrichCtx,
	))

	// Database schema enrichment
	c.registry.Register(task.OperationCreateDatabaseSchemaForCommit, enrichmenthandler.NewDatabaseSchema(
		c.repoStores.Repositories, c.enrichCtx, c.schemaDiscoverer,
	))

	// Cookbook enrichment
	c.registry.Register(task.OperationCreateCookbookForCommit, enrichmenthandler.NewCookbook(
		c.repoStores.Repositories, c.repoStores.Files, c.enrichCtx, c.cookbookContext,
	))

	// API docs enrichment
	c.registry.Register(task.OperationCreatePublicAPIDocsForCommit, enrichmenthandler.NewAPIDocs(
		c.repoStores.Repositories, c.repoStores.Files, c.enrichCtx, c.apiDocService,
	))

	// Example extraction handler
	c.registry.Register(task.OperationExtractExamplesForCommit, enrichmenthandler.NewExtractExamples(
		c.repoStores.Repositories, c.repoStores.Commits, c.gitInfra.Adapter, c.enrichCtx, c.exampleDiscoverer,
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
		enrichQ:         c.enrichCtx.Query,
		enrichmentStore: c.enrichCtx.Enrichments,
	}
}

// Tasks returns the task queue interface.
func (c *Client) Tasks() Tasks {
	return &tasksImpl{
		queue:     c.queue,
		taskStore: c.taskStore,
	}
}

// CodeSearchService returns the underlying code search service.
// This is useful for advanced callers like MCP servers that need
// full control over search requests (e.g., MultiSearchRequest).
// Returns nil if no search stores are configured.
func (c *Client) CodeSearchService() *service.CodeSearch {
	return c.codeSearch
}

// RepositoryQuery returns the repository query service.
func (c *Client) RepositoryQuery() *service.RepositoryQuery {
	return c.repoQuery
}

// RepositorySync returns the repository sync service.
func (c *Client) RepositorySync() *service.RepositorySync {
	return c.repoSync
}

// TrackingQuery returns the task tracking query service.
func (c *Client) TrackingQuery() *service.TrackingQuery {
	return c.trackingQuery
}

// EnrichmentQuery returns the enrichment query service.
func (c *Client) EnrichmentQuery() *service.EnrichmentQuery {
	return c.enrichCtx.Query
}

// EnrichmentStore returns the enrichment persistence store.
func (c *Client) EnrichmentStore() enrichment.EnrichmentStore {
	return c.enrichCtx.Enrichments
}

// AssociationStore returns the enrichment association store.
func (c *Client) AssociationStore() enrichment.AssociationStore {
	return c.enrichCtx.Associations
}

// SnippetStore returns access to the snippet storage.
func (c *Client) SnippetStore() snippet.SnippetStore {
	return c.snippetStore
}

// CodeVectorStore returns the code vector store for embedding lookups.
func (c *Client) CodeVectorStore() search.VectorStore {
	return c.codeIndex.Store
}

// Queue returns the task queue service.
func (c *Client) Queue() *service.Queue {
	return c.queue
}

// TaskStore returns the task persistence store.
func (c *Client) TaskStore() task.TaskStore {
	return c.taskStore
}

// StatusStore returns the task status persistence store.
func (c *Client) StatusStore() task.StatusStore {
	return c.statusStore
}

// Logger returns the client's logger.
func (c *Client) Logger() *slog.Logger {
	return c.logger
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
	enrichmentStore enrichment.EnrichmentStore
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

// buildDatabaseURL constructs the database URL from configuration.
func buildDatabaseURL(cfg *clientConfig) (string, error) {
	switch cfg.database {
	case databaseSQLite:
		return "sqlite:///" + cfg.dbPath, nil
	case databasePostgres, databasePostgresPgvector, databasePostgresVectorchord:
		return cfg.dbDSN, nil
	default:
		return "", ErrNoDatabase
	}
}

// trackerFactoryImpl implements handler.TrackerFactory for progress reporting.
type trackerFactoryImpl struct {
	reporters []tracking.Reporter
	logger    *slog.Logger
}

// ForOperation creates a Tracker for the given operation.
func (f *trackerFactoryImpl) ForOperation(operation task.Operation, trackableType task.TrackableType, trackableID int64) handler.Tracker {
	tracker := tracking.TrackerForOperation(operation, f.logger, trackableType, trackableID)
	for _, reporter := range f.reporters {
		tracker.Subscribe(reporter)
	}
	return tracker
}
