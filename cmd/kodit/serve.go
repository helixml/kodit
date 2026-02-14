package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/infrastructure/api"
	apimiddleware "github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/config"
	"github.com/helixml/kodit/internal/log"
	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	var (
		envFile string
		host    string
		port    int
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		Long: `Start the HTTP API server.

Configuration is loaded in the following order (later sources override earlier):
  1. Default values
  2. .env file (if --env-file specified or .env exists in current directory)
  3. Environment variables
  4. Command line flags

Environment variables:
  HOST                         Server host to bind to (default: 0.0.0.0)
  PORT                         Server port to listen on (default: 8080)
  DATA_DIR                     Data directory (default: .kodit)
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
			return runServe(envFile, host, port)
		},
	}

	cmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file (default: .env in current directory)")
	cmd.Flags().StringVar(&host, "host", "", "Server host to bind to (default: 0.0.0.0)")
	cmd.Flags().IntVar(&port, "port", 0, "Server port to listen on (default: 8080)")

	return cmd
}

func runServe(envFile, host string, port int) error {
	// Load configuration
	cfg, err := loadConfig(envFile)
	if err != nil {
		return err
	}

	// Apply command line overrides (flags take precedence over env vars)
	cfg = applyServeOverrides(cfg, host, port)

	addr := cfg.Addr()

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

	// Build kodit client options
	opts := []kodit.Option{
		kodit.WithDataDir(cfg.DataDir()),
		kodit.WithCloneDir(cfg.CloneDir()),
		kodit.WithLogger(slogger),
	}

	// Configure storage based on database URL
	dbURLStr := cfg.DBURL()
	if dbURLStr != "" && !isSQLite(dbURLStr) {
		// Assume VectorChord for PostgreSQL databases
		opts = append(opts, kodit.WithPostgresVectorchord(dbURLStr))
	} else {
		// Default to SQLite
		dbPath := cfg.DataDir() + "/kodit.db"
		if dbURLStr != "" && isSQLite(dbURLStr) {
			// Extract path from sqlite URL
			dbPath = dbURLStr[10:] // Remove "sqlite:///" prefix
		}
		opts = append(opts, kodit.WithSQLite(dbPath))
	}

	// Configure embedding provider if available
	embEndpoint := cfg.EmbeddingEndpoint()
	if embEndpoint != nil && embEndpoint.BaseURL() != "" && embEndpoint.APIKey() != "" {
		opts = append(opts, kodit.WithEmbeddingProvider(provider.NewOpenAIProviderFromConfig(provider.OpenAIConfig{
			APIKey:         embEndpoint.APIKey(),
			BaseURL:        embEndpoint.BaseURL(),
			EmbeddingModel: embEndpoint.Model(),
			Timeout:        embEndpoint.Timeout(),
			MaxRetries:     embEndpoint.MaxRetries(),
		})))
	}

	// Configure text generation provider if available
	enrichEndpoint := cfg.EnrichmentEndpoint()
	if enrichEndpoint != nil && enrichEndpoint.BaseURL() != "" && enrichEndpoint.APIKey() != "" {
		opts = append(opts, kodit.WithTextProvider(provider.NewOpenAIProviderFromConfig(provider.OpenAIConfig{
			APIKey:     enrichEndpoint.APIKey(),
			BaseURL:    enrichEndpoint.BaseURL(),
			ChatModel:  enrichEndpoint.Model(),
			Timeout:    enrichEndpoint.Timeout(),
			MaxRetries: enrichEndpoint.MaxRetries(),
		})))
	}

	// Configure API keys
	if keys := cfg.APIKeys(); len(keys) > 0 {
		opts = append(opts, kodit.WithAPIKeys(keys...))
	}

	// Configure worker count
	if cfg.WorkerCount() > 0 {
		opts = append(opts, kodit.WithWorkerCount(cfg.WorkerCount()))
	}

	// Configure periodic sync
	opts = append(opts, kodit.WithPeriodicSyncConfig(cfg.PeriodicSync()))

	// Skip provider validation if explicitly disabled (for testing)
	if cfg.SkipProviderValidation() {
		opts = append(opts, kodit.WithSkipProviderValidation())
	}

	// Create kodit client and log settings
	attrs := append([]slog.Attr{slog.String("version", version)}, cfg.LogAttrs()...)
	slogger.LogAttrs(context.Background(), slog.LevelInfo, "starting kodit", attrs...)

	client, err := kodit.New(opts...)
	if err != nil {
		return fmt.Errorf("create kodit client: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			slogger.Error("failed to close kodit client", slog.Any("error", err))
		}
	}()

	// Create API server with the client's services
	apiServer := api.NewAPIServer(client)
	router := apiServer.Router()

	// Apply custom middleware (MUST be done before MountRoutes)
	router.Use(apimiddleware.Logging(slogger))
	router.Use(apimiddleware.CorrelationID)

	// Mount API routes after middleware is configured
	apiServer.MountRoutes()

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
	docsRouter := apiServer.DocsRouter("/docs/openapi.json")
	router.Mount("/docs", docsRouter.Routes())

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create standalone server for custom router
	server := api.NewServer(addr, slogger)
	server.Router().Mount("/", router)

	go func() {
		<-sigChan
		slogger.Info("shutting down server")
		cancel()
		if err := server.Shutdown(ctx); err != nil {
			slogger.Error("shutdown error", slog.Any("error", err))
		}
	}()

	slogger.Info("starting server", slog.String("addr", addr))
	if err := server.Start(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// isSQLite checks if the database URL is for SQLite.
func isSQLite(url string) bool {
	return len(url) >= 7 && url[:7] == "sqlite:"
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy"}`))
}

// applyServeOverrides applies command line flag overrides to the config.
func applyServeOverrides(cfg config.AppConfig, host string, port int) config.AppConfig {
	var opts []config.AppConfigOption

	if host != "" {
		opts = append(opts, config.WithHost(host))
	}
	if port != 0 {
		opts = append(opts, config.WithPort(port))
	}

	return cfg.Apply(opts...)
}
