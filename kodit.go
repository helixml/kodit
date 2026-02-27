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
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/chunking"
	"github.com/helixml/kodit/infrastructure/enricher"
	"github.com/helixml/kodit/infrastructure/enricher/example"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/infrastructure/slicing"
	"github.com/helixml/kodit/infrastructure/slicing/language"
	"github.com/helixml/kodit/infrastructure/tracking"
	"github.com/helixml/kodit/internal/config"
	"github.com/helixml/kodit/internal/database"
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
	Blobs        *service.Blob
	Enrichments  *service.Enrichment
	Tasks        *service.Queue
	Tracking     *service.Tracking
	Search       *service.Search

	db         database.Database
	repoStores handler.RepositoryStores

	// Stores not grouped into aggregates
	taskStore      persistence.TaskStore
	statusStore    persistence.StatusStore
	lineRangeStore persistence.ChunkLineRangeStore

	// Aggregate dependencies
	enrichCtx handler.EnrichmentContext
	codeIndex handler.VectorIndex
	textIndex handler.VectorIndex
	gitInfra  handler.GitInfrastructure

	// Application services (internal only)
	bm25Service  *domainservice.BM25
	queue        *service.Queue
	worker       *service.Worker
	periodicSync *service.PeriodicSync
	registry     *service.Registry

	// Code slicing (internal)
	slicer *slicing.Slicer

	// Discovery services (each used by exactly one handler)
	archDiscoverer    *enricher.PhysicalArchitectureService
	exampleDiscoverer *example.Discovery
	schemaDiscoverer  *enricher.DatabaseSchemaService
	apiDocService     *enricher.APIDocService
	cookbookContext   *enricher.CookbookContextService

	hugotEmbedding *provider.HugotEmbedding
	closers        []io.Closer

	logger         *slog.Logger
	dataDir        string
	cloneDir       string
	apiKeys        []string
	simpleChunking bool
	chunkParams    chunking.ChunkParams
	prescribedOps  task.PrescribedOperations
	closed         atomic.Bool
	mu             sync.Mutex
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

	// Create built-in embedding provider if no external provider is configured
	var hugotEmbedding *provider.HugotEmbedding
	if cfg.embeddingProvider == nil {
		modelDir := cfg.modelDir
		if modelDir == "" {
			modelDir = filepath.Join(dataDir, "models")
		}
		hugotEmbedding = provider.NewHugotEmbedding(modelDir)
		if hugotEmbedding.Available() {
			cfg.embeddingProvider = hugotEmbedding
			logger.Info("built-in embedding provider enabled", slog.String("model_dir", modelDir))
		} else {
			return nil, fmt.Errorf("no embedding model found in %s — run 'make download-model' or configure an external embedding provider", modelDir)
		}
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

	// One-time schema conversions from Python-era database
	if err := persistence.PreMigrate(db); err != nil {
		errClose := db.Close()
		return nil, errors.Join(fmt.Errorf("pre migrate: %w", err), errClose)
	}

	// Run auto migration
	if err := persistence.AutoMigrate(db); err != nil {
		errClose := db.Close()
		return nil, errors.Join(fmt.Errorf("auto migrate: %w", err), errClose)
	}

	// Validate schema matches GORM models
	if err := persistence.ValidateSchema(db); err != nil {
		errClose := db.Close()
		return nil, errors.Join(fmt.Errorf("validate schema: %w", err), errClose)
	}

	// Create stores
	repoStore := persistence.NewRepositoryStore(db)
	commitStore := persistence.NewCommitStore(db)
	branchStore := persistence.NewBranchStore(db)
	tagStore := persistence.NewTagStore(db)
	fileStore := persistence.NewFileStore(db)
	enrichmentStore := persistence.NewEnrichmentStore(db)
	associationStore := persistence.NewAssociationStore(db)
	lineRangeStore := persistence.NewChunkLineRangeStore(db)
	taskStore := persistence.NewTaskStore(db)
	statusStore := persistence.NewStatusStore(db)

	// Group repository stores
	repoStores := handler.RepositoryStores{
		Repositories: repoStore,
		Commits:      commitStore,
		Branches:     branchStore,
		Tags:         tagStore,
		Files:        fileStore,
	}

	// Probe embedding dimension once (only needed for PostgreSQL vector stores
	// that require VECTOR(N) column declarations; SQLite stores JSON and needs
	// no dimension up front).
	var dimension int
	needsDimensionProbe := cfg.embeddingProvider != nil &&
		cfg.database == databasePostgresVectorchord
	if needsDimensionProbe {
		resp, err := cfg.embeddingProvider.Embed(ctx, provider.NewEmbeddingRequest([]string{"dimension probe"}))
		if err != nil {
			errClose := db.Close()
			return nil, errors.Join(fmt.Errorf("probe embedding dimension: %w", err), errClose)
		}
		probeEmbeddings := resp.Embeddings()
		if len(probeEmbeddings) == 0 || len(probeEmbeddings[0]) == 0 {
			errClose := db.Close()
			return nil, errors.Join(fmt.Errorf("failed to obtain embedding dimension from provider"), errClose)
		}
		dimension = len(probeEmbeddings[0])
	}

	// Create search stores based on storage type
	textEmbeddingStore, codeEmbeddingStore, bm25Store, err := buildSearchStores(ctx, cfg, db, dimension, logger)
	if err != nil {
		errClose := db.Close()
		return nil, errors.Join(fmt.Errorf("search stores: %w", err), errClose)
	}

	// Create domain embedder from infrastructure provider
	var domainEmbedder search.Embedder
	if cfg.embeddingProvider != nil {
		domainEmbedder = &embeddingAdapter{inner: cfg.embeddingProvider}
	}

	// Create vector indices (pairing embedding services with their stores)
	var codeIndex handler.VectorIndex
	if codeEmbeddingStore != nil {
		embSvc, err := domainservice.NewEmbedding(codeEmbeddingStore, domainEmbedder, cfg.embeddingBudget, cfg.embeddingParallelism)
		if err != nil {
			return nil, fmt.Errorf("create code embedding service: %w", err)
		}
		codeIndex = handler.VectorIndex{
			Embedding: embSvc,
			Store:     codeEmbeddingStore,
		}
	}
	var textIndex handler.VectorIndex
	if textEmbeddingStore != nil {
		embSvc, err := domainservice.NewEmbedding(textEmbeddingStore, domainEmbedder, cfg.enrichmentBudget, cfg.enrichmentParallelism)
		if err != nil {
			return nil, fmt.Errorf("create text embedding service: %w", err)
		}
		textIndex = handler.VectorIndex{
			Embedding: embSvc,
			Store:     textEmbeddingStore,
		}
	}

	// Create application services
	registry := service.NewRegistry()
	queue := service.NewQueue(taskStore, logger)

	enrichQSvc := service.NewEnrichment(enrichmentStore, associationStore, bm25Store, codeEmbeddingStore, textEmbeddingStore)
	trackingSvc := service.NewTracking(statusStore, taskStore)

	// Create BM25 service for keyword search (always available)
	bm25Svc, err := domainservice.NewBM25(bm25Store)
	if err != nil {
		return nil, fmt.Errorf("create bm25 service: %w", err)
	}

	// Create git infrastructure
	gitAdapter, err := git.NewGiteaAdapter(logger)
	if err != nil {
		return nil, fmt.Errorf("create git adapter: %w", err)
	}
	clonerSvc := git.NewRepositoryCloner(gitAdapter, cloneDir, logger)
	scannerSvc := git.NewRepositoryScanner(gitAdapter, logger)

	gitInfra := handler.GitInfrastructure{
		Adapter: gitAdapter,
		Cloner:  clonerSvc,
		Scanner: scannerSvc,
	}

	// Create slicer for code extraction
	langConfig := slicing.NewLanguageConfig()
	analyzerFactory := language.NewFactory(langConfig)
	slicer := slicing.NewSlicer(langConfig, analyzerFactory)

	// Create tracker factory for progress reporting.
	// Wrap reporters in cooldowns to limit database writes and log output
	// to at most once per second per status ID during high-frequency updates.
	dbCooldown := tracking.NewCooldown(tracking.NewDBReporter(statusStore, logger), time.Second)
	logCooldown := tracking.NewCooldown(tracking.NewLoggingReporter(logger), time.Second)
	reporters := []tracking.Reporter{dbCooldown, logCooldown}
	trackerFactory := &trackerFactoryImpl{
		reporters: reporters,
		logger:    logger,
	}
	worker := service.NewWorker(taskStore, registry, &workerTrackerAdapter{trackerFactory}, logger)
	if cfg.workerPollPeriod > 0 {
		worker.WithPollPeriod(cfg.workerPollPeriod)
	}
	prescribedOps := task.NewPrescribedOperations(!cfg.simpleChunking)
	periodicSync := service.NewPeriodicSync(cfg.periodicSync, repoStore, queue, prescribedOps, logger)

	// Create enricher infrastructure (only if text provider is configured)
	var enricherImpl domainservice.Enricher
	if cfg.textProvider != nil {
		enricherImpl = enricher.NewProviderEnricher(cfg.textProvider).
			WithParallelism(cfg.enricherParallelism)
	}

	// Build enrichment context
	enrichCtx := handler.EnrichmentContext{
		Enrichments:  enrichmentStore,
		Associations: associationStore,
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

	// Register cooldowns for cleanup on close so pending statuses are flushed.
	cfg.closers = append(cfg.closers, dbCooldown, logCooldown)

	client := &Client{
		db:                db,
		repoStores:        repoStores,
		taskStore:         taskStore,
		statusStore:       statusStore,
		lineRangeStore:    lineRangeStore,
		enrichCtx:         enrichCtx,
		codeIndex:         codeIndex,
		textIndex:         textIndex,
		gitInfra:          gitInfra,
		bm25Service:       bm25Svc,
		queue:             queue,
		worker:            worker,
		periodicSync:      periodicSync,
		registry:          registry,
		slicer:            slicer,
		archDiscoverer:    archDiscoverer,
		exampleDiscoverer: exampleDiscoverer,
		schemaDiscoverer:  schemaDiscoverer,
		apiDocService:     apiDocSvc,
		cookbookContext:   cookbookCtx,
		hugotEmbedding:    hugotEmbedding,
		closers:           cfg.closers,
		logger:            logger,
		dataDir:           dataDir,
		cloneDir:          cloneDir,
		apiKeys:           cfg.apiKeys,
		simpleChunking:    cfg.simpleChunking,
		chunkParams:       cfg.chunkParams,
		prescribedOps:     prescribedOps,
	}

	// Initialize service fields directly
	client.Repositories = service.NewRepository(repoStore, commitStore, branchStore, tagStore, queue, client.prescribedOps, logger)
	client.Commits = service.NewCommit(commitStore)
	client.Tags = service.NewTag(tagStore)
	client.Files = service.NewFile(fileStore)
	client.Blobs = service.NewBlob(repoStore, commitStore, tagStore, branchStore, gitAdapter)
	client.Enrichments = enrichQSvc
	client.Tasks = queue
	client.Tracking = trackingSvc
	client.Search = service.NewSearch(domainEmbedder, textEmbeddingStore, codeEmbeddingStore, bm25Store, enrichmentStore, &client.closed, logger)

	// Register task handlers
	if err := client.registerHandlers(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("register handlers: %w", err)
	}

	// Validate all prescribed operations have handlers
	if !cfg.skipProviderValidation {
		if err := client.validateHandlers(); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	// Start the background worker and periodic sync
	worker.Start(ctx)
	periodicSync.Start(ctx)

	return client, nil
}

// Close releases all resources and stops the background worker.
func (c *Client) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return ErrClientClosed
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop the periodic sync and worker
	c.periodicSync.Stop()
	c.worker.Stop()

	// Close built-in embedding provider
	if c.hugotEmbedding != nil {
		if err := c.hugotEmbedding.Close(); err != nil {
			c.logger.Error("failed to close hugot embedding", slog.Any("error", err))
		}
	}

	// Close registered resources (e.g. caching transports)
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil {
			c.logger.Error("failed to close resource", slog.Any("error", err))
		}
	}

	// Close the database
	if err := c.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	c.logger.Info("kodit client closed")
	return nil
}

// WorkerIdle reports whether the background worker has no in-flight tasks.
func (c *Client) WorkerIdle() bool {
	return !c.worker.Busy()
}

// Logger returns the client's logger.
func (c *Client) Logger() *slog.Logger {
	return c.logger
}

// embeddingAdapter adapts provider.Embedder to the domain search.Embedder interface.
type embeddingAdapter struct {
	inner provider.Embedder
}

func (a *embeddingAdapter) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	resp, err := a.inner.Embed(ctx, provider.NewEmbeddingRequest(texts))
	if err != nil {
		return nil, err
	}
	return resp.Embeddings(), nil
}

// buildSearchStores creates the search stores based on config.
func buildSearchStores(ctx context.Context, cfg *clientConfig, db database.Database, dimension int, logger *slog.Logger) (textEmbeddingStore, codeEmbeddingStore search.EmbeddingStore, bm25Store search.BM25Store, err error) {
	switch cfg.database {
	case databaseSQLite:
		bm25Store, err = persistence.NewSQLiteBM25Store(db, logger)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("bm25 store: %w", err)
		}
		if cfg.embeddingProvider != nil {
			textStore, textErr := persistence.NewSQLiteEmbeddingStore(db, persistence.TaskNameText, logger)
			if textErr != nil {
				return nil, nil, nil, fmt.Errorf("text embedding store: %w", textErr)
			}
			textEmbeddingStore = textStore
			codeStore, codeErr := persistence.NewSQLiteEmbeddingStore(db, persistence.TaskNameCode, logger)
			if codeErr != nil {
				return nil, nil, nil, fmt.Errorf("code embedding store: %w", codeErr)
			}
			codeEmbeddingStore = codeStore
		}
	case databasePostgresVectorchord:
		bm25Store, err = persistence.NewVectorChordBM25Store(db, logger)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("bm25 store: %w", err)
		}
		if cfg.embeddingProvider != nil {
			textEmbeddingStore, err = persistence.NewVectorChordEmbeddingStore(ctx, db, persistence.TaskNameText, dimension, logger)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("text embedding store: %w", err)
			}
			codeEmbeddingStore, err = persistence.NewVectorChordEmbeddingStore(ctx, db, persistence.TaskNameCode, dimension, logger)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("code embedding store: %w", err)
			}
		}
	}
	return
}
