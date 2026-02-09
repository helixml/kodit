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

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/snippet"
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

// Repositories returns the repository management interface.
func (c *Client) Repositories() Repositories {
	return &repositoriesImpl{
		repoSync:  c.repoSync,
		repoQuery: c.repoQuery,
	}
}

// Enrichments returns the enrichment query interface.
func (c *Client) Enrichments() Enrichments {
	return &enrichmentsImpl{
		query: c.enrichCtx.Query,
	}
}

// Tasks returns the task queue interface.
func (c *Client) Tasks() Tasks {
	return &tasksImpl{
		queue: c.queue,
	}
}

// Snippets returns the snippet query interface.
func (c *Client) Snippets() Snippets {
	return &snippetsImpl{
		snippetStore: c.snippetStore,
		vectorStore:  c.codeIndex.Store,
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

// Logger returns the client's logger.
func (c *Client) Logger() *slog.Logger {
	return c.logger
}
