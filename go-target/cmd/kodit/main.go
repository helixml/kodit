// Package main is the entry point for the kodit CLI.
//
//	@title						Kodit API
//	@version					1.0
//	@description				Code understanding platform with hybrid search and LLM-powered enrichments
//	@host						localhost:8080
//	@BasePath					/api/v1
//	@securityDefinitions.apikey	APIKeyAuth
//	@in							header
//	@name						X-API-KEY
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
	"github.com/helixml/kodit/internal/api"
	apimiddleware "github.com/helixml/kodit/internal/api/middleware"
	v1 "github.com/helixml/kodit/internal/api/v1"
	"github.com/helixml/kodit/internal/config"
	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/enrichment/example"
	enrichmentpg "github.com/helixml/kodit/internal/enrichment/postgres"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/git/gitadapter"
	gitpg "github.com/helixml/kodit/internal/git/postgres"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/indexing/bm25"
	indexingpg "github.com/helixml/kodit/internal/indexing/postgres"
	"github.com/helixml/kodit/internal/indexing/slicer"
	"github.com/helixml/kodit/internal/indexing/slicer/analyzers"
	"github.com/helixml/kodit/internal/indexing/vector"
	"github.com/helixml/kodit/internal/log"
	"github.com/helixml/kodit/internal/mcp"
	"github.com/helixml/kodit/internal/provider"
	"github.com/helixml/kodit/internal/queue"
	"github.com/helixml/kodit/internal/queue/handler"
	enrichmenthandler "github.com/helixml/kodit/internal/queue/handler/enrichment"
	queuepg "github.com/helixml/kodit/internal/queue/postgres"
	"github.com/helixml/kodit/internal/repository"
	"github.com/helixml/kodit/internal/search"
	"github.com/helixml/kodit/internal/tracking"
	"github.com/spf13/cobra"
)

// Version information set via ldflags during build.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kodit",
		Short: "Kodit code intelligence server",
		Long:  `Kodit is a code understanding platform that indexes Git repositories and provides hybrid search with LLM-powered enrichments.`,
	}

	cmd.AddCommand(serveCmd())
	cmd.AddCommand(stdioCmd())
	cmd.AddCommand(versionCmd())

	return cmd
}

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

func stdioCmd() *cobra.Command {
	var (
		envFile  string
		dataDir  string
		dbURL    string
		logLevel string
	)

	cmd := &cobra.Command{
		Use:   "stdio",
		Short: "Start MCP server on stdio",
		Long: `Start the MCP (Model Context Protocol) server on stdio.

This allows AI assistants to interact with Kodit for code search and understanding.
Configuration is loaded from environment variables and .env file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStdio(envFile, dataDir, dbURL, logLevel)
		},
	}

	cmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory")
	cmd.Flags().StringVar(&dbURL, "db-url", "", "Database URL")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "Log level")

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kodit version %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		},
	}
}

// loadConfig loads configuration from .env file and environment variables,
// then applies CLI flag overrides.
func loadConfig(envFile, dataDir, dbURL, logLevel string) (config.AppConfig, error) {
	// Load config from .env file and environment variables
	cfg, err := config.LoadConfig(envFile)
	if err != nil {
		return config.AppConfig{}, fmt.Errorf("load config: %w", err)
	}

	// Apply CLI flag overrides (CLI flags take precedence)
	if dataDir != "" {
		config.WithDataDir(dataDir)(&cfg)
	}
	if dbURL != "" {
		config.WithDBURL(dbURL)(&cfg)
	}
	if logLevel != "" {
		config.WithLogLevel(logLevel)(&cfg)
	}

	return cfg, nil
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
	db, err := database.NewDatabase(ctx, cfg.DBURL())
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slogger.Error("failed to close database", slog.Any("error", err))
		}
	}()

	// Run database migrations (AutoMigrate for all GORM entities)
	if err := runAutoMigrate(db, slogger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Create git repositories
	repoRepo := gitpg.NewRepoRepository(db)
	commitRepo := gitpg.NewCommitRepository(db)
	branchRepo := gitpg.NewBranchRepository(db)
	tagRepo := gitpg.NewTagRepository(db)
	fileRepo := gitpg.NewFileRepository(db)

	// Create queue repositories
	taskRepo := queuepg.NewTaskRepository(db)
	taskStatusRepo := queuepg.NewTaskStatusRepository(db)

	// Create enrichment repositories
	enrichmentRepo := enrichmentpg.NewEnrichmentRepository(db)
	associationRepo := enrichmentpg.NewAssociationRepository(db)

	// Create indexing repositories (use *gorm.DB directly)
	snippetRepo := indexingpg.NewSnippetRepository(db.GORM())

	// Create services
	queueService := queue.NewService(taskRepo, slogger)
	queryService := repository.NewQueryService(repoRepo, commitRepo, branchRepo, tagRepo).WithFileRepository(fileRepo)
	syncService := repository.NewSyncService(repoRepo, queueService, slogger)
	enrichmentQueryService := enrichment.NewQueryService(enrichmentRepo, associationRepo)

	// Create git adapter and helpers
	gitAdapter := gitadapter.NewGoGit(slogger)
	cloner := git.NewCloner(gitAdapter, cfg.CloneDir(), slogger)
	scanner := git.NewScanner(gitAdapter, slogger)

	// Create slicer for code extraction
	langConfig := slicer.NewLanguageConfig()
	analyzerFactory := analyzers.NewFactory(langConfig)
	slicerInstance := slicer.NewSlicer(langConfig, analyzerFactory)

	// Create search service (requires BM25 and Vector repositories)
	var searchService search.Service
	var vectorRepo indexing.VectorSearchRepository
	var bm25Repo indexing.BM25Repository
	var embeddingProvider *provider.OpenAIProvider
	var textGenerator provider.TextGenerator

	if db.IsPostgres() {
		// Create optional AI provider for embeddings (if configured)
		embEndpoint := cfg.EmbeddingEndpoint()
		if embEndpoint != nil && embEndpoint.BaseURL() != "" && embEndpoint.APIKey() != "" {
			embeddingProvider = provider.NewOpenAIProviderFromEndpoint(*embEndpoint)
		}

		// Create optional AI provider for text generation (if configured)
		enrichEndpoint := cfg.EnrichmentEndpoint()
		if enrichEndpoint != nil && enrichEndpoint.BaseURL() != "" && enrichEndpoint.APIKey() != "" {
			textGenerator = provider.NewOpenAIProviderFromEndpoint(*enrichEndpoint)
		}

		// Create VectorChord repositories (PostgreSQL only)
		bm25Repo = bm25.NewVectorChordRepository(db.GORM(), slogger)
		vectorRepo = vector.NewVectorChordRepository(db.GORM(), vector.TaskNameCode, embeddingProvider, slogger)

		searchService = search.NewService(bm25Repo, vectorRepo, snippetRepo, enrichmentRepo, slogger)
	} else {
		// SQLite mode: search service with nil repos (search disabled)
		slogger.Warn("using SQLite database - search features will be limited")
		searchService = search.NewService(nil, nil, snippetRepo, enrichmentRepo, slogger)
	}

	// Create tracker factory for progress reporting
	dbReporter := tracking.NewDBReporter(taskStatusRepo, slogger)
	trackerFactory := &trackerFactoryImpl{
		dbReporter: dbReporter,
		logger:     slogger,
	}

	// Create handler registry and register handlers
	registry := queue.NewRegistry()
	registerHandlers(
		registry, cfg, slogger,
		repoRepo, commitRepo, branchRepo, fileRepo, tagRepo,
		snippetRepo, bm25Repo, vectorRepo,
		enrichmentRepo, associationRepo, enrichmentQueryService,
		queueService, gitAdapter, cloner, scanner, slicerInstance,
		embeddingProvider, textGenerator, trackerFactory,
	)

	// Create and start queue worker
	worker := queue.NewWorker(taskRepo, registry, slogger)
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
		reposRouter = reposRouter.WithEnrichmentServices(enrichmentQueryService, enrichmentRepo)
		reposRouter = reposRouter.WithIndexingServices(snippetRepo, vectorRepo)
		r.Mount("/repositories", reposRouter.Routes())

		// Commits router
		commitsRouter := v1.NewCommitsRouter(queryService, slogger)
		r.Mount("/commits", commitsRouter.Routes())

		// Search router
		searchRouter := v1.NewSearchRouter(searchService, slogger)
		r.Mount("/search", searchRouter.Routes())

		// Enrichments router
		enrichmentsRouter := v1.NewEnrichmentsRouter(enrichmentRepo, slogger)
		r.Mount("/enrichments", enrichmentsRouter.Routes())

		// Queue router
		queueRouter := v1.NewQueueRouter(taskRepo, taskStatusRepo, slogger)
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
func dbType(db database.Database) string {
	if db.IsPostgres() {
		return "postgresql"
	}
	if db.IsSQLite() {
		return "sqlite"
	}
	return "unknown"
}

func runStdio(envFile, dataDir, dbURL, logLevel string) error {
	// Load configuration
	cfg, err := loadConfig(envFile, dataDir, dbURL, logLevel)
	if err != nil {
		return err
	}

	// Ensure directories exist
	if err := cfg.EnsureDataDir(); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Setup logger to file (can't use stdout for MCP)
	logger := log.NewLogger(cfg)
	slogger := logger.Slog()

	slogger.Info("starting MCP server",
		slog.String("version", version),
		slog.String("data_dir", cfg.DataDir()),
	)

	// Setup context
	ctx := context.Background()

	// Connect to database
	db, err := database.NewDatabase(ctx, cfg.DBURL())
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slogger.Error("failed to close database", slog.Any("error", err))
		}
	}()

	// Create repositories
	snippetRepo := indexingpg.NewSnippetRepository(db.GORM())
	enrichmentRepo := enrichmentpg.NewEnrichmentRepository(db)

	// Create search service
	var searchService search.Service
	if db.IsPostgres() {
		var embedder provider.Embedder
		embEndpoint := cfg.EmbeddingEndpoint()
		if embEndpoint != nil && embEndpoint.BaseURL() != "" && embEndpoint.APIKey() != "" {
			embedder = provider.NewOpenAIProviderFromEndpoint(*embEndpoint)
		}

		bm25Repo := bm25.NewVectorChordRepository(db.GORM(), slogger)
		vectorRepo := vector.NewVectorChordRepository(db.GORM(), vector.TaskNameCode, embedder, slogger)
		searchService = search.NewService(bm25Repo, vectorRepo, snippetRepo, enrichmentRepo, slogger)
	} else {
		searchService = search.NewService(nil, nil, snippetRepo, enrichmentRepo, slogger)
	}

	// Create MCP server with database-backed search
	mcpServer := mcp.NewServer(searchService, slogger)

	// Run on stdio
	return mcpServer.ServeStdio()
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy"}`))
}

// runAutoMigrate runs GORM AutoMigrate for all entity types.
func runAutoMigrate(db database.Database, logger *slog.Logger) error {
	logger.Info("running database migrations")

	// AutoMigrate all entities
	err := db.GORM().AutoMigrate(
		// Git entities
		&gitpg.RepoEntity{},
		&gitpg.CommitEntity{},
		&gitpg.BranchEntity{},
		&gitpg.TagEntity{},
		&gitpg.FileEntity{},
		// Queue entities
		&queuepg.TaskEntity{},
		&queuepg.TaskStatusEntity{},
		// Indexing entities
		&indexingpg.CommitIndexEntity{},
		&indexingpg.SnippetEntity{},
		&indexingpg.SnippetCommitAssociationEntity{},
		&indexingpg.SnippetFileDerivationEntity{},
		// Enrichment entities
		&enrichmentpg.EnrichmentEntity{},
		&enrichmentpg.AssociationEntity{},
	)
	if err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	logger.Info("database migrations complete")
	return nil
}

// trackerFactoryImpl implements handler.TrackerFactory.
type trackerFactoryImpl struct {
	dbReporter *tracking.DBReporter
	logger     *slog.Logger
}

// ForOperation creates a Tracker for the given operation.
func (f *trackerFactoryImpl) ForOperation(operation queue.TaskOperation, trackableType domain.TrackableType, trackableID int64) *tracking.Tracker {
	tracker := tracking.TrackerForOperation(operation, f.logger, trackableType, trackableID)
	if f.dbReporter != nil {
		tracker.Subscribe(f.dbReporter)
	}
	return tracker
}

// registerHandlers registers all task handlers with the registry.
func registerHandlers(
	registry *queue.Registry,
	_ config.AppConfig,
	logger *slog.Logger,
	repoRepo git.RepoRepository,
	commitRepo git.CommitRepository,
	branchRepo git.BranchRepository,
	fileRepo git.FileRepository,
	tagRepo git.TagRepository,
	snippetRepo indexing.SnippetRepository,
	bm25Repo indexing.BM25Repository,
	vectorRepo indexing.VectorSearchRepository,
	enrichmentRepo enrichment.EnrichmentRepository,
	associationRepo enrichment.AssociationRepository,
	enrichmentQueryService *enrichment.QueryService,
	queueService *queue.Service,
	gitAdapter git.Adapter,
	cloner git.Cloner,
	scanner git.Scanner,
	slicerInstance *slicer.Slicer,
	embeddingProvider *provider.OpenAIProvider,
	textGenerator provider.TextGenerator,
	trackerFactory handler.TrackerFactory,
) {
	// Repository handlers
	registry.Register(queue.OperationCloneRepository, handler.NewCloneRepository(
		repoRepo, cloner, queueService, trackerFactory, logger,
	))
	registry.Register(queue.OperationSyncRepository, handler.NewSyncRepository(
		repoRepo, branchRepo, cloner, scanner, queueService, trackerFactory, logger,
	))
	registry.Register(queue.OperationDeleteRepository, handler.NewDeleteRepository(
		repoRepo, commitRepo, branchRepo, tagRepo, fileRepo, snippetRepo, trackerFactory, logger,
	))
	registry.Register(queue.OperationScanCommit, handler.NewScanCommit(
		repoRepo, commitRepo, fileRepo, scanner, trackerFactory, logger,
	))

	// Indexing handlers
	registry.Register(queue.OperationExtractSnippetsForCommit, handler.NewExtractSnippets(
		repoRepo, commitRepo, snippetRepo, gitAdapter, slicerInstance, trackerFactory, logger,
	))

	// BM25 handler requires BM25 service
	if bm25Repo != nil {
		bm25Service := indexing.NewBM25Service(bm25Repo)
		registry.Register(queue.OperationCreateBM25IndexForCommit, handler.NewCreateBM25Index(
			bm25Service, snippetRepo, trackerFactory, logger,
		))
	}

	// Embeddings handler requires embedding service and vector repo
	if vectorRepo != nil && embeddingProvider != nil {
		embeddingService := indexing.NewEmbeddingService(embeddingProvider, vectorRepo)
		registry.Register(queue.OperationCreateCodeEmbeddingsForCommit, handler.NewCreateCodeEmbeddings(
			embeddingService, snippetRepo, vectorRepo, trackerFactory, logger,
		))
	}

	// Enrichment handlers (only if text generator is configured)
	if textGenerator != nil {
		enricher := enrichment.NewProviderEnricher(textGenerator, logger)

		// Summary enrichment
		registry.Register(queue.OperationCreateSummaryEnrichmentForCommit, enrichmenthandler.NewCreateSummary(
			snippetRepo, enrichmentRepo, associationRepo, enrichmentQueryService, enricher, trackerFactory, logger,
		))

		// Commit description
		registry.Register(queue.OperationCreateCommitDescriptionForCommit, enrichmenthandler.NewCommitDescription(
			repoRepo, enrichmentRepo, associationRepo, enrichmentQueryService, gitAdapter, enricher, trackerFactory, logger,
		))

		// Architecture discovery
		archDiscoverer := enrichment.NewPhysicalArchitectureService()
		registry.Register(queue.OperationCreateArchitectureEnrichmentForCommit, enrichmenthandler.NewArchitectureDiscovery(
			repoRepo, enrichmentRepo, associationRepo, enrichmentQueryService, archDiscoverer, enricher, trackerFactory, logger,
		))

		// Example summary
		registry.Register(queue.OperationCreateExampleSummaryForCommit, enrichmenthandler.NewExampleSummary(
			enrichmentRepo, associationRepo, enrichmentQueryService, enricher, trackerFactory, logger,
		))
	}

	// Example extraction handler (doesn't require LLM)
	exampleDiscoverer := example.NewDiscovery()
	registry.Register(queue.OperationExtractExamplesForCommit, enrichmenthandler.NewExtractExamples(
		repoRepo, commitRepo, gitAdapter, enrichmentRepo, associationRepo, enrichmentQueryService, exampleDiscoverer, trackerFactory, logger,
	))

	logger.Info("registered task handlers", slog.Int("count", len(registry.Operations())))
}
