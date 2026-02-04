package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/application/handler"
	commithandler "github.com/helixml/kodit/application/handler/commit"
	enrichmenthandler "github.com/helixml/kodit/application/handler/enrichment"
	indexinghandler "github.com/helixml/kodit/application/handler/indexing"
	repohandler "github.com/helixml/kodit/application/handler/repository"
	"github.com/helixml/kodit/application/service"
	domainservice "github.com/helixml/kodit/domain/service"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/api"
	apimiddleware "github.com/helixml/kodit/infrastructure/api/middleware"
	v1 "github.com/helixml/kodit/infrastructure/api/v1"
	"github.com/helixml/kodit/infrastructure/enricher"
	"github.com/helixml/kodit/infrastructure/enricher/example"
	"github.com/helixml/kodit/infrastructure/git"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/infrastructure/provider"
	infraSearch "github.com/helixml/kodit/infrastructure/search"
	"github.com/helixml/kodit/infrastructure/slicing"
	"github.com/helixml/kodit/infrastructure/slicing/language"
	"github.com/helixml/kodit/infrastructure/tracking"
	"github.com/helixml/kodit/internal/log"
	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	var (
		addr    string
		envFile string
		// CLI flags override env vars
		dataDir  string
		dbURL    string
		logLevel string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		Long: `Start the HTTP API server.

Configuration is loaded in the following order (later sources override earlier):
  1. Default values
  2. .env file (if --env-file specified or .env exists in current directory)
  3. Environment variables
  4. CLI flags

Environment variables:
  DATA_DIR                     Data directory (default: ~/.kodit)
  DB_URL                       Database URL (default: sqlite:///{data_dir}/kodit.db)
  LOG_LEVEL                    Log level: DEBUG, INFO, WARN, ERROR (default: INFO)
  LOG_FORMAT                   Log format: pretty, json (default: pretty)
  DISABLE_TELEMETRY            Disable telemetry (default: false)
  API_KEYS                     Comma-separated list of valid API keys

  EMBEDDING_ENDPOINT_*         Embedding AI service configuration
    BASE_URL                   Base URL (e.g., https://api.openai.com/v1)
    MODEL                      Model identifier (e.g., text-embedding-3-small)
    API_KEY                    API key for authentication
    NUM_PARALLEL_TASKS         Concurrent requests (default: 10)
    TIMEOUT                    Request timeout in seconds (default: 60)
    MAX_RETRIES                Retry attempts (default: 5)

  ENRICHMENT_ENDPOINT_*        Enrichment AI service configuration
    (same fields as EMBEDDING_ENDPOINT)

  DEFAULT_SEARCH_PROVIDER      Search backend: sqlite, vectorchord (default: sqlite)
  GIT_PROVIDER                 Git library: dulwich, pygit2, gitpython (default: dulwich)

  PERIODIC_SYNC_ENABLED        Enable periodic sync (default: true)
  PERIODIC_SYNC_INTERVAL_SECONDS  Sync interval (default: 1800)

  REMOTE_SERVER_URL            Remote Kodit server URL
  REMOTE_API_KEY               Remote server API key`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(addr, envFile, dataDir, dbURL, logLevel)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "Address to listen on")
	cmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file (default: .env in current directory)")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (overrides DATA_DIR env var)")
	cmd.Flags().StringVar(&dbURL, "db-url", "", "Database URL (overrides DB_URL env var)")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "Log level (overrides LOG_LEVEL env var)")

	return cmd
}

func runServe(addr, envFile, dataDir, dbURL, logLevel string) error {
	// Load configuration
	cfg, err := loadConfig(envFile, dataDir, dbURL, logLevel)
	if err != nil {
		return err
	}

	// Ensure directories exist
	if err := cfg.EnsureDataDir(); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	if err := cfg.EnsureCloneDir(); err != nil {
		return fmt.Errorf("create clone directory: %w", err)
	}

	// Setup logger
	logger := log.NewLogger(cfg)
	slogger := logger.Slog()

	// Setup graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to database
	slogger.Info("connecting to database", slog.String("url", maskDBURL(cfg.DBURL())))
	db, err := persistence.NewDatabase(ctx, cfg.DBURL())
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slogger.Error("failed to close database", slog.Any("error", err))
		}
	}()

	// Run database migrations (AutoMigrate for all GORM entities)
	if err := db.AutoMigrate(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Create persistence stores
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

	// Create application services
	queueService := service.NewQueue(taskStore, slogger)
	queryService := service.NewRepositoryQuery(repoStore, commitStore, branchStore, tagStore).WithFileStore(fileStore)
	syncService := service.NewRepositorySync(repoStore, queueService, slogger)
	enrichmentQueryService := service.NewEnrichmentQuery(enrichmentStore, associationStore)
	trackingQueryService := service.NewTrackingQuery(statusStore, taskStore)

	// Create git adapter and helpers
	gitAdapter := git.NewGoGitAdapter(slogger)
	cloner := git.NewRepositoryCloner(gitAdapter, cfg.CloneDir(), slogger)
	scanner := git.NewRepositoryScanner(gitAdapter, slogger)

	// Create slicer for code extraction
	langConfig := slicing.NewLanguageConfig()
	analyzerFactory := language.NewFactory(langConfig)
	slicerInstance := slicing.NewSlicer(langConfig, analyzerFactory)

	// Create search service (requires BM25 and Vector stores)
	var searchService service.CodeSearch
	var vectorStore *infraSearch.VectorChordVectorStore
	var bm25Store *infraSearch.VectorChordBM25Store
	var embeddingProvider *provider.OpenAIProvider
	var textGenerator provider.TextGenerator

	if db.IsPostgres() {
		// Create optional AI provider for embeddings (if configured)
		embEndpoint := cfg.EmbeddingEndpoint()
		if embEndpoint != nil && embEndpoint.BaseURL() != "" && embEndpoint.APIKey() != "" {
			embeddingProvider = provider.NewOpenAIProviderFromConfig(provider.OpenAIConfig{
				APIKey:         embEndpoint.APIKey(),
				BaseURL:        embEndpoint.BaseURL(),
				EmbeddingModel: embEndpoint.Model(),
				Timeout:        embEndpoint.Timeout(),
				MaxRetries:     embEndpoint.MaxRetries(),
			})
		}

		// Create optional AI provider for text generation (if configured)
		enrichEndpoint := cfg.EnrichmentEndpoint()
		if enrichEndpoint != nil && enrichEndpoint.BaseURL() != "" && enrichEndpoint.APIKey() != "" {
			textGenerator = provider.NewOpenAIProviderFromConfig(provider.OpenAIConfig{
				APIKey:     enrichEndpoint.APIKey(),
				BaseURL:    enrichEndpoint.BaseURL(),
				ChatModel:  enrichEndpoint.Model(),
				Timeout:    enrichEndpoint.Timeout(),
				MaxRetries: enrichEndpoint.MaxRetries(),
			})
		}

		// Create VectorChord stores (PostgreSQL only)
		bm25Store = infraSearch.NewVectorChordBM25Store(db.GORM(), slogger)
		if embeddingProvider != nil {
			vectorStore = infraSearch.NewVectorChordVectorStore(db.GORM(), infraSearch.TaskNameCode, embeddingProvider, slogger)
		}

		searchService = service.NewCodeSearch(bm25Store, vectorStore, snippetStore, enrichmentStore, slogger)
	} else {
		// SQLite mode: search service with nil stores (search disabled)
		slogger.Warn("using SQLite database - search features will be limited")
		searchService = service.NewCodeSearch(nil, nil, snippetStore, enrichmentStore, slogger)
	}

	// Create tracker factory for progress reporting
	dbReporter := tracking.NewDBReporter(statusStore, slogger)
	trackerFactory := &trackerFactoryImpl{
		dbReporter: dbReporter,
		logger:     slogger,
	}

	// Create handler registry and register handlers
	registry := handler.NewRegistry()
	registerHandlers(
		registry, slogger,
		repoStore, commitStore, branchStore, fileStore, tagStore,
		snippetStore, bm25Store, vectorStore,
		enrichmentStore, associationStore, enrichmentQueryService,
		queueService, gitAdapter, cloner, scanner, slicerInstance,
		embeddingProvider, textGenerator, trackerFactory,
	)

	// Create and start queue worker
	worker := service.NewWorker(taskStore, toServiceRegistry(registry), slogger)
	worker.Start(ctx)
	defer worker.Stop()

	// Create server
	server := api.NewServer(addr, slogger)
	router := server.Router()

	// Apply middleware
	router.Use(apimiddleware.Logging(slogger))
	router.Use(apimiddleware.CorrelationID)

	// Health check endpoints
	router.Get("/health", healthHandler)
	router.Get("/healthz", healthHandler)

	// Root endpoint with API info
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"name":"kodit","version":"%s","docs":"/docs"}`, version)
	})

	// Documentation routes
	docsRouter := api.NewDocsRouter("/docs/openapi.json")
	router.Mount("/docs", docsRouter.Routes())

	// Register API v1 routes with full database-backed services
	router.Route("/api/v1", func(r chi.Router) {
		// Apply API key authentication if keys are configured
		if len(cfg.APIKeys()) > 0 {
			r.Use(apimiddleware.APIKeyAuth(cfg.APIKeys()))
		}

		// Repositories router
		reposRouter := v1.NewRepositoriesRouter(queryService, syncService, slogger)
		reposRouter = reposRouter.WithTrackingQueryService(trackingQueryService)
		reposRouter = reposRouter.WithEnrichmentServices(enrichmentQueryService, enrichmentStore)
		reposRouter = reposRouter.WithIndexingServices(snippetStore, vectorStore)
		r.Mount("/repositories", reposRouter.Routes())

		// Commits router
		commitsRouter := v1.NewCommitsRouter(queryService, slogger)
		r.Mount("/commits", commitsRouter.Routes())

		// Search router
		searchRouter := v1.NewSearchRouter(searchService, slogger)
		r.Mount("/search", searchRouter.Routes())

		// Enrichments router
		enrichmentsRouter := v1.NewEnrichmentsRouter(enrichmentStore, slogger)
		r.Mount("/enrichments", enrichmentsRouter.Routes())

		// Queue router
		queueRouter := v1.NewQueueRouter(queueService, taskStore, statusStore, slogger)
		r.Mount("/queue", queueRouter.Routes())
	})

	slogger.Info("starting server",
		slog.String("addr", addr),
		slog.String("version", version),
		slog.String("data_dir", cfg.DataDir()),
		slog.String("log_level", cfg.LogLevel()),
		slog.String("db_type", dbType(db)),
		slog.Int("handlers", len(registry.Operations())),
	)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		slogger.Info("shutting down server")
		cancel()
		if err := server.Shutdown(ctx); err != nil {
			slogger.Error("shutdown error", slog.Any("error", err))
		}
	}()

	if err := server.Start(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// maskDBURL masks sensitive information in database URLs for logging.
func maskDBURL(url string) string {
	// Simple masking: just show the driver type
	if len(url) > 10 {
		if url[:7] == "sqlite:" {
			return url // SQLite paths are not sensitive
		}
		// For PostgreSQL, show driver and masked credentials
		return "postgres://***@***"
	}
	return "***"
}

// dbType returns a string describing the database type.
func dbType(db persistence.Database) string {
	if db.IsPostgres() {
		return "postgresql"
	}
	if db.IsSQLite() {
		return "sqlite"
	}
	return "unknown"
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy"}`))
}

// trackerFactoryImpl implements handler.TrackerFactory.
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

// toServiceRegistry converts handler.Registry to service.Registry for the worker.
func toServiceRegistry(hr *handler.Registry) *service.Registry {
	sr := service.NewRegistry()
	for _, op := range hr.Operations() {
		h, err := hr.Handler(op)
		if err == nil {
			sr.Register(op, h)
		}
	}
	return sr
}

// registerHandlers registers all task handlers with the registry.
func registerHandlers(
	registry *handler.Registry,
	logger *slog.Logger,
	repoStore persistence.RepositoryStore,
	commitStore persistence.CommitStore,
	branchStore persistence.BranchStore,
	fileStore persistence.FileStore,
	tagStore persistence.TagStore,
	snippetStore persistence.SnippetStore,
	bm25Store *infraSearch.VectorChordBM25Store,
	vectorStore *infraSearch.VectorChordVectorStore,
	enrichmentStore persistence.EnrichmentStore,
	associationStore persistence.AssociationStore,
	enrichmentQueryService *service.EnrichmentQuery,
	queueService *service.Queue,
	gitAdapter git.Adapter,
	cloner domainservice.Cloner,
	scanner domainservice.Scanner,
	slicerInstance *slicing.Slicer,
	embeddingProvider *provider.OpenAIProvider,
	textGenerator provider.TextGenerator,
	trackerFactory handler.TrackerFactory,
) {
	// Repository handlers
	registry.Register(task.OperationCloneRepository, repohandler.NewClone(
		repoStore, cloner, queueService, trackerFactory, logger,
	))
	registry.Register(task.OperationSyncRepository, repohandler.NewSync(
		repoStore, branchStore, cloner, scanner, queueService, trackerFactory, logger,
	))
	registry.Register(task.OperationDeleteRepository, repohandler.NewDelete(
		repoStore, commitStore, branchStore, tagStore, fileStore, snippetStore, trackerFactory, logger,
	))
	registry.Register(task.OperationScanCommit, commithandler.NewScan(
		repoStore, commitStore, fileStore, scanner, trackerFactory, logger,
	))

	// Indexing handlers
	registry.Register(task.OperationExtractSnippetsForCommit, indexinghandler.NewExtractSnippets(
		repoStore, snippetStore, gitAdapter, slicerInstance, trackerFactory, logger,
	))

	// BM25 handler requires BM25 service
	if bm25Store != nil {
		bm25Service := domainservice.NewBM25(bm25Store)
		registry.Register(task.OperationCreateBM25IndexForCommit, indexinghandler.NewCreateBM25Index(
			bm25Service, snippetStore, trackerFactory, logger,
		))
	}

	// Embeddings handler requires embedding service and vector store
	if vectorStore != nil && embeddingProvider != nil {
		embeddingService := domainservice.NewEmbedding(vectorStore)
		registry.Register(task.OperationCreateCodeEmbeddingsForCommit, indexinghandler.NewCreateCodeEmbeddings(
			embeddingService, snippetStore, vectorStore, trackerFactory, logger,
		))
	}

	// Enrichment handlers (only if text generator is configured)
	if textGenerator != nil {
		providerEnricher := enricher.NewProviderEnricher(textGenerator, logger)

		// Summary enrichment
		registry.Register(task.OperationCreateSummaryEnrichmentForCommit, enrichmenthandler.NewCreateSummary(
			snippetStore, enrichmentStore, associationStore, enrichmentQueryService, providerEnricher, trackerFactory, logger,
		))

		// Commit description
		registry.Register(task.OperationCreateCommitDescriptionForCommit, enrichmenthandler.NewCommitDescription(
			repoStore, enrichmentStore, associationStore, enrichmentQueryService, gitAdapter, providerEnricher, trackerFactory, logger,
		))

		// Architecture discovery
		archDiscoverer := enricher.NewPhysicalArchitectureService()
		registry.Register(task.OperationCreateArchitectureEnrichmentForCommit, enrichmenthandler.NewArchitectureDiscovery(
			repoStore, enrichmentStore, associationStore, enrichmentQueryService, archDiscoverer, providerEnricher, trackerFactory, logger,
		))

		// Example summary
		registry.Register(task.OperationCreateExampleSummaryForCommit, enrichmenthandler.NewExampleSummary(
			enrichmentStore, associationStore, enrichmentQueryService, providerEnricher, trackerFactory, logger,
		))
	}

	// Example extraction handler (doesn't require LLM)
	exampleDiscoverer := example.NewDiscovery()
	registry.Register(task.OperationExtractExamplesForCommit, enrichmenthandler.NewExtractExamples(
		repoStore, commitStore, gitAdapter, enrichmentStore, associationStore, enrichmentQueryService, exampleDiscoverer, trackerFactory, logger,
	))

	logger.Info("registered task handlers", slog.Int("count", len(registry.Operations())))
}
