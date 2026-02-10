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
//	repo, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{
//	    URL: "https://github.com/kubernetes/kubernetes",
//	})
//
//	// Hybrid search
//	results, err := client.Search.Query(ctx, "create a deployment",
//	    service.WithSemanticWeight(0.7),
//	    service.WithLimit(10),
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
	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/infrastructure/slicing"
	"github.com/helixml/kodit/infrastructure/slicing/language"
	"github.com/helixml/kodit/infrastructure/tracking"
	"github.com/helixml/kodit/internal/config"
)

// Client is the main entry point for the kodit library.
// The background worker starts automatically on creation.
//
// Access resources via struct fields:
//
//	client.Repositories.Find(ctx)
//	client.Commits.Find(ctx, repository.WithRepoID(id))
//	client.Search.Query(ctx, "query")
type Client struct {
	// Public resource fields (direct service access)
	Repositories *service.Repository
	Commits      *service.Commit
	Tags         *service.Tag
	Files        *service.File
	Snippets     *service.Snippet
	Enrichments  *service.Enrichment
	Tasks        *service.Queue
	Tracking     *service.Tracking
	Search       *service.Search

	db         database.Database
	repoStores RepositoryStores

	// Stores not grouped into aggregates
	snippetStore snippet.SnippetStore
	taskStore    persistence.TaskStore
	statusStore  persistence.StatusStore

	// Aggregate dependencies
	enrichCtx EnrichmentContext
	codeIndex VectorIndex
	textIndex VectorIndex
	gitInfra  GitInfrastructure

	// Application services (internal only)
	bm25Service *domainservice.BM25
	queue       *service.Queue
	worker      *service.Worker
	registry    *service.Registry

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
	db, err := database.NewDatabase(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Run auto migration
	if err := persistence.AutoMigrate(db); err != nil {
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
	textVectorStore, codeVectorStore, bm25Store := buildSearchStores(cfg, db, logger)

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

	enrichQSvc := service.NewEnrichment(enrichmentStore, associationStore)
	trackingSvc := service.NewTracking(statusStore, taskStore)

	// Create BM25 service for keyword search (always available)
	bm25Svc := domainservice.NewBM25(bm25Store)

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
	apiDocSvc := enricher.NewAPIDocService()
	cookbookCtx := enricher.NewCookbookContextService()

	client := &Client{
		db:                db,
		repoStores:        repoStores,
		snippetStore:      snippetStore,
		taskStore:         taskStore,
		statusStore:       statusStore,
		enrichCtx:         enrichCtx,
		codeIndex:         codeIndex,
		textIndex:         textIndex,
		gitInfra:          gitInfra,
		bm25Service:       bm25Svc,
		queue:             queue,
		worker:            worker,
		registry:          registry,
		slicer:            slicer,
		archDiscoverer:    archDiscoverer,
		exampleDiscoverer: exampleDiscoverer,
		schemaDiscoverer:  schemaDiscoverer,
		apiDocService:     apiDocSvc,
		cookbookContext:   cookbookCtx,
		logger:            logger,
		dataDir:           dataDir,
		cloneDir:          cloneDir,
		apiKeys:           cfg.apiKeys,
	}

	// Initialize service fields directly
	client.Repositories = service.NewRepository(repoStore, commitStore, branchStore, tagStore, queue, logger)
	client.Commits = service.NewCommit(commitStore)
	client.Tags = service.NewTag(tagStore)
	client.Files = service.NewFile(fileStore)
	client.Snippets = service.NewSnippet(snippetStore, codeIndex.Store)
	client.Enrichments = enrichQSvc
	client.Tasks = queue
	client.Tracking = trackingSvc
	client.Search = service.NewSearch(textVectorStore, codeVectorStore, snippetStore, enrichmentStore, &client.closed, logger)

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

// Logger returns the client's logger.
func (c *Client) Logger() *slog.Logger {
	return c.logger
}

// buildSearchStores creates the search stores based on config.
func buildSearchStores(cfg *clientConfig, db database.Database, logger *slog.Logger) (textVectorStore, codeVectorStore search.VectorStore, bm25Store search.BM25Store) {
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
	}
	return
}
