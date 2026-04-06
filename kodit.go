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
//
// # Pipeline presets
//
// By default, kodit runs all indexing operations including LLM enrichments when
// a text provider is configured. Use [WithRAGPipeline] to skip LLM enrichments
// and run only the operations needed for retrieval-augmented generation:
//
//	client, err := kodit.New(
//	    kodit.WithSQLite(".kodit/data.db"),
//	    kodit.WithRAGPipeline(), // skip wiki, summaries, architecture docs, etc.
//	)
//
// Use [WithFullPipeline] to explicitly require all enrichments (returns an error
// if no text provider is configured):
//
//	client, err := kodit.New(
//	    kodit.WithSQLite(".kodit/data.db"),
//	    kodit.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
//	    kodit.WithFullPipeline(),
//	)
package kodit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/handler"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/search"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/chunking"
	"github.com/helixml/kodit/infrastructure/enricher"
	"github.com/helixml/kodit/infrastructure/extraction"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/infrastructure/rasterization"
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
	Grep         *service.Grep
	Pipelines    *service.Pipeline

	// MCPServer describes the MCP server's tools and instructions.
	MCPServer MCPServer

	db         database.Database
	repoStores handler.RepositoryStores

	// Stores not grouped into aggregates
	taskStore      persistence.TaskStore
	statusStore    persistence.StatusStore
	lineRangeStore persistence.SourceLocationStore

	// Aggregate dependencies
	enrichCtx   handler.EnrichmentContext
	codeIndex   handler.VectorIndex
	textIndex   handler.VectorIndex
	visionIndex handler.VectorIndex
	gitInfra    handler.GitInfrastructure

	// Application services (internal only)
	bm25Service  *domainservice.BM25
	queue        *service.Queue
	worker       *service.Worker
	periodicSync *service.PeriodicSync
	registry     *service.Registry

	// Document text extraction (internal)
	documentText *extraction.DocumentText

	// Document rasterization (internal)
	rasterizers *rasterization.Registry

	// Discovery services (each used by exactly one handler)
	archDiscoverer   *enricher.PhysicalArchitectureService
	schemaDiscoverer *enricher.DatabaseSchemaService
	apiDocService    *enricher.APIDocService
	cookbookContext  *enricher.CookbookContextService
	wikiContext      *enricher.WikiContextService

	hugotEmbedding  *provider.HugotEmbedding
	visionEmbedding *provider.LocalVisionEmbedding
	closers         []io.Closer

	logger      zerolog.Logger
	dataDir     string
	cloneDir    string
	apiKeys     []string
	chunkParams chunking.ChunkParams
	closed      atomic.Bool
	mu          sync.Mutex
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

	// Early pipeline validation: if the prescribed ops require a text provider,
	// fail fast before any expensive setup (DB open, model load, etc.).
	if cfg.prescribedOpsFactory != nil && cfg.textProvider == nil {
		probe := cfg.prescribedOpsFactory(false)
		if probe.RequiresTextProvider() {
			return nil, fmt.Errorf("WithFullPipeline requires a text provider (WithOpenAI, WithAnthropic, or WithTextProvider)")
		}
	}

	// Set up logger
	logger := cfg.logger

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
			logger.Info().Str("model_dir", modelDir).Msg("built-in embedding provider enabled")
		} else {
			return nil, fmt.Errorf("no embedding model found in %s — run 'make download-model' or configure an external embedding provider", modelDir)
		}
	}

	// Create vision embedding model (SigLIP2).
	modelDir := cfg.modelDir
	if modelDir == "" {
		modelDir = filepath.Join(dataDir, "models")
	}
	visionEmbedding := provider.NewLocalVisionEmbedding(provider.SigLIP2BaseConfig, modelDir)
	if visionEmbedding.Available() {
		logger.Info().Msg("vision embedding model (SigLIP2) available")
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
	lineRangeStore := persistence.NewSourceLocationStore(db)
	pipelineStore := persistence.NewPipelineStore(db)
	stepStore := persistence.NewStepStore(db)
	pipelineStepStore := persistence.NewPipelineStepStore(db)
	stepDependencyStore := persistence.NewStepDependencyStore(db)
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
		resp, err := cfg.embeddingProvider.Embed(ctx, provider.NewTextEmbeddingRequest([]string{"dimension probe"}))
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
	textEmbeddingStore, codeEmbeddingStore, bm25Store, embeddingsRebuilt, err := buildSearchStores(ctx, cfg, db, dimension, logger)
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

	// Create vision embedding store and index (only if vision model is available)
	var visionEmbeddingStore search.EmbeddingStore
	var visionIndex handler.VectorIndex
	if visionEmbedding.Available() {
		switch cfg.database {
		case databaseSQLite:
			vs, vsErr := persistence.NewSQLiteEmbeddingStore(db, persistence.TaskNameVision, logger)
			if vsErr != nil {
				return nil, fmt.Errorf("vision embedding store: %w", vsErr)
			}
			visionEmbeddingStore = vs
		case databasePostgresVectorchord:
			vs, _, vsErr := persistence.NewVectorChordEmbeddingStore(ctx, db, persistence.TaskNameVision, 768, logger)
			if vsErr != nil {
				return nil, fmt.Errorf("vision embedding store: %w", vsErr)
			}
			visionEmbeddingStore = vs
		}
		if visionEmbeddingStore != nil {
			visionIndex = handler.VectorIndex{
				Store: visionEmbeddingStore,
			}
		}
	}

	// Create application services
	registry := service.NewRegistry()
	queue := service.NewQueue(taskStore, logger)

	enrichQSvc := service.NewEnrichment(enrichmentStore, associationStore, bm25Store, codeEmbeddingStore, textEmbeddingStore, visionEmbeddingStore, lineRangeStore)
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
	worker := service.NewWorker(taskStore, statusStore, registry, &workerTrackerAdapter{trackerFactory}, logger)
	if cfg.workerPollPeriod > 0 {
		worker.WithPollPeriod(cfg.workerPollPeriod)
	}
	if cfg.prescribedOpsFactory == nil {
		cfg.prescribedOpsFactory = task.DefaultPrescribedOperations
		if cfg.textProvider == nil {
			logger.Warn().Msg("enrichment endpoint not configured — LLM-based enrichments (summaries, architecture docs, commit descriptions, cookbooks, wiki) will be disabled; set ENRICHMENT_ENDPOINT_* environment variables to enable them")
		}
	}
	prescribedOps := cfg.prescribedOpsFactory(cfg.textProvider != nil).
		WithVision(visionEmbedding.Available() && visionEmbeddingStore != nil)
	if prescribedOps.RequiresTextProvider() && cfg.textProvider == nil {
		return nil, fmt.Errorf("WithFullPipeline requires a text provider (WithOpenAI, WithAnthropic, or WithTextProvider)")
	}
	periodicSync := service.NewPeriodicSync(cfg.periodicSync, repoStore, queue, logger)

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

	// Create document text extractor
	documentText := extraction.NewDocumentText()

	// Create rasterization registry for document-to-image conversion.
	rasterizers := rasterization.NewRegistry()
	pdfRast, err := rasterization.NewPdfiumRasterizer()
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create pdfium rasterizer: %w", err)
	}
	if pdfRast != nil {
		rasterizers.Register(".pdf", pdfRast)
		cfg.closers = append(cfg.closers, pdfRast)
		logger.Info().Msg("PDF rasterizer (pdfium WASM) registered")
	} else {
		logger.Warn().Msg("PDF rasterizer not available — page image extraction will skip PDF files")
	}

	// Create enrichment infrastructure (always available)
	archDiscoverer := enricher.NewPhysicalArchitectureService()
	schemaDiscoverer := enricher.NewDatabaseSchemaService()
	apiDocSvc := enricher.NewAPIDocService()
	cookbookCtx := enricher.NewCookbookContextService()
	wikiCtx := enricher.NewWikiContextService()

	// Register cooldowns for cleanup on close so pending statuses are flushed.
	cfg.closers = append(cfg.closers, dbCooldown, logCooldown)

	client := &Client{
		db:               db,
		repoStores:       repoStores,
		taskStore:        taskStore,
		statusStore:      statusStore,
		lineRangeStore:   lineRangeStore,
		enrichCtx:        enrichCtx,
		codeIndex:        codeIndex,
		textIndex:        textIndex,
		visionIndex:      visionIndex,
		gitInfra:         gitInfra,
		bm25Service:      bm25Svc,
		queue:            queue,
		worker:           worker,
		periodicSync:     periodicSync,
		registry:         registry,
		documentText:     documentText,
		rasterizers:      rasterizers,
		archDiscoverer:   archDiscoverer,
		schemaDiscoverer: schemaDiscoverer,
		apiDocService:    apiDocSvc,
		cookbookContext:  cookbookCtx,
		wikiContext:      wikiCtx,
		hugotEmbedding:   hugotEmbedding,
		visionEmbedding:  visionEmbedding,
		closers:          cfg.closers,
		logger:           logger,
		dataDir:          dataDir,
		cloneDir:         cloneDir,
		apiKeys:          cfg.apiKeys,
		chunkParams:      cfg.chunkParams,
	}

	// Populate MCP server metadata
	client.MCPServer = mcpServerFromDefinitions()

	// Initialize Pipeline service first — Repository depends on it as resolver.
	client.Pipelines = service.NewPipeline(pipelineStore, stepStore, pipelineStepStore, stepDependencyStore, prescribedOps)
	if err := client.Pipelines.Initialise(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialise pipelines: %w", err)
	}

	// Initialize remaining service fields
	client.Repositories = service.NewRepository(repoStore, pipelineStore, commitStore, branchStore, tagStore, queue, client.Pipelines, logger)
	if err := client.Repositories.BackfillDefaultPipeline(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("backfill default pipeline: %w", err)
	}
	client.Commits = service.NewCommit(commitStore)
	client.Tags = service.NewTag(tagStore)
	client.Files = service.NewFile(fileStore)
	client.Blobs = service.NewBlob(repoStore, commitStore, tagStore, branchStore, gitAdapter)
	client.Enrichments = enrichQSvc
	client.Tasks = queue
	client.Tracking = trackingSvc
	client.Search = service.NewSearch(domainEmbedder, textEmbeddingStore, codeEmbeddingStore, bm25Store, enrichmentStore, &client.closed, logger)
	client.Grep = service.NewGrep(repoStore, commitStore, gitAdapter)

	// Register task handlers
	if err := client.registerHandlers(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("register handlers: %w", err)
	}

	// Validate all prescribed operations have handlers
	if cfg.skipProviderValidation {
		logger.Warn().Msg("SKIP_PROVIDER_VALIDATION is deprecated and will be removed in a future release — enrichments are now automatically disabled when no enrichment endpoint is configured")
	}
	if err := client.validateHandlers(client.Pipelines.RequiredOperations()); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Start the background worker and periodic sync
	if err := worker.Start(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("start worker: %w", err)
	}
	periodicSync.Start(ctx)

	// If embedding tables were rebuilt (dimension change), enqueue a sync for
	// every repository so enrichments get re-embedded with the new model.
	if embeddingsRebuilt {
		repos, repoErr := repoStore.Find(ctx)
		if repoErr != nil {
			_ = db.Close()
			return nil, fmt.Errorf("find repositories for re-index: %w", repoErr)
		}
		operations := []task.Operation{task.OperationCloneRepository, task.OperationSyncRepository}
		for _, repo := range repos {
			payload := map[string]any{"repository_id": repo.ID()}
			if enqErr := queue.EnqueueOperations(ctx, operations, task.PriorityNormal, payload); enqErr != nil {
				_ = db.Close()
				return nil, fmt.Errorf("enqueue re-index for repository %d: %w", repo.ID(), enqErr)
			}
		}
		logger.Info().Int("repositories", len(repos)).Msg("embedding dimension changed, enqueued sync for re-indexing")
	}

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
			c.logger.Error().Interface("error", err).Msg("failed to close hugot embedding")
		}
	}

	// Close registered resources (e.g. caching transports)
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil {
			c.logger.Error().Interface("error", err).Msg("failed to close resource")
		}
	}

	// Close the database
	if err := c.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	c.logger.Info().Msg("kodit client closed")
	return nil
}

// WorkerIdle reports whether the background worker has no in-flight tasks.
func (c *Client) WorkerIdle() bool {
	return !c.worker.Busy()
}

// Logger returns the client's logger.
func (c *Client) Logger() zerolog.Logger {
	return c.logger
}

// embeddingAdapter adapts provider.Embedder to the domain search.Embedder interface.
type embeddingAdapter struct {
	inner provider.Embedder
}

func (a *embeddingAdapter) Embed(ctx context.Context, inputs [][]byte) ([][]float64, error) {
	resp, err := a.inner.Embed(ctx, provider.NewEmbeddingRequest(inputs))
	if err != nil {
		return nil, err
	}
	return resp.Embeddings(), nil
}

// buildSearchStores creates the search stores based on config.
// The returned rebuilt bool is true when any VectorChord embedding table was
// dropped and recreated due to a dimension change (requiring a full re-index).
func buildSearchStores(ctx context.Context, cfg *clientConfig, db database.Database, dimension int, logger zerolog.Logger) (textEmbeddingStore, codeEmbeddingStore search.EmbeddingStore, bm25Store search.BM25Store, rebuilt bool, err error) {
	switch cfg.database {
	case databaseSQLite:
		bm25Store, err = persistence.NewSQLiteBM25Store(db, logger)
		if err != nil {
			return nil, nil, nil, false, fmt.Errorf("bm25 store: %w", err)
		}
		if cfg.embeddingProvider != nil {
			textStore, textErr := persistence.NewSQLiteEmbeddingStore(db, persistence.TaskNameText, logger)
			if textErr != nil {
				return nil, nil, nil, false, fmt.Errorf("text embedding store: %w", textErr)
			}
			textEmbeddingStore = textStore
			codeStore, codeErr := persistence.NewSQLiteEmbeddingStore(db, persistence.TaskNameCode, logger)
			if codeErr != nil {
				return nil, nil, nil, false, fmt.Errorf("code embedding store: %w", codeErr)
			}
			codeEmbeddingStore = codeStore
		}
	case databasePostgresVectorchord:
		bm25Store, err = persistence.NewVectorChordBM25Store(db, logger)
		if err != nil {
			return nil, nil, nil, false, fmt.Errorf("bm25 store: %w", err)
		}
		if cfg.embeddingProvider != nil {
			var textRebuilt, codeRebuilt bool
			textEmbeddingStore, textRebuilt, err = persistence.NewVectorChordEmbeddingStore(ctx, db, persistence.TaskNameText, dimension, logger)
			if err != nil {
				return nil, nil, nil, false, fmt.Errorf("text embedding store: %w", err)
			}
			codeEmbeddingStore, codeRebuilt, err = persistence.NewVectorChordEmbeddingStore(ctx, db, persistence.TaskNameCode, dimension, logger)
			if err != nil {
				return nil, nil, nil, false, fmt.Errorf("code embedding store: %w", err)
			}
			rebuilt = textRebuilt || codeRebuilt
		}
	}
	return
}
