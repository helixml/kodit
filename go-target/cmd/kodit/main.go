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
	"github.com/helixml/kodit/internal/config"
	"github.com/helixml/kodit/internal/log"
	"github.com/helixml/kodit/internal/mcp"
	"github.com/helixml/kodit/internal/search"
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

	// Register API v1 routes (minimal setup without database)
	router.Route("/api/v1", func(r chi.Router) {
		// Apply API key authentication if keys are configured
		if len(cfg.APIKeys()) > 0 {
			r.Use(apimiddleware.APIKeyAuth(cfg.APIKeys()))
		}

		// In production, use ServerFactory with proper dependencies
		// For now, return 501 Not Implemented for API endpoints
		r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":"API not configured - database connection required"}`))
		})
	})

	logger.Info("starting server",
		slog.String("addr", addr),
		slog.String("version", version),
		slog.String("data_dir", cfg.DataDir()),
		slog.String("log_level", cfg.LogLevel()),
	)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("shutting down server")
		cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("shutdown error", slog.Any("error", err))
		}
	}()

	if err := server.Start(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func runStdio(envFile, dataDir, dbURL, logLevel string) error {
	// Load configuration
	cfg, err := loadConfig(envFile, dataDir, dbURL, logLevel)
	if err != nil {
		return err
	}

	// Setup logger to file (can't use stdout for MCP)
	logger := log.NewLogger(cfg)

	logger.Info("starting MCP server",
		slog.String("version", version),
		slog.String("data_dir", cfg.DataDir()),
	)

	// Note: In a full implementation, we would connect to the database here
	// and create the search service with proper dependencies.
	// For now, we create a minimal MCP server with an empty search service.
	var searchService search.Service

	// Create MCP server
	mcpServer := mcp.NewServer(searchService, slog.Default())

	// Run on stdio
	return mcpServer.ServeStdio()
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy"}`))
}
