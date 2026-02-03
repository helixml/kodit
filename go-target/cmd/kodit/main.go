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
		addr     string
		dataDir  string
		dbURL    string
		logLevel string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(addr, dataDir, dbURL, logLevel)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "Address to listen on")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (default: ~/.kodit)")
	cmd.Flags().StringVar(&dbURL, "db-url", "", "Database URL (default: sqlite:///~/.kodit/kodit.db)")
	cmd.Flags().StringVar(&logLevel, "log-level", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")

	return cmd
}

func stdioCmd() *cobra.Command {
	var (
		dataDir  string
		dbURL    string
		logLevel string
	)

	cmd := &cobra.Command{
		Use:   "stdio",
		Short: "Start MCP server on stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStdio(dataDir, dbURL, logLevel)
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (default: ~/.kodit)")
	cmd.Flags().StringVar(&dbURL, "db-url", "", "Database URL")
	cmd.Flags().StringVar(&logLevel, "log-level", "INFO", "Log level")

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("kodit version 0.1.0")
		},
	}
}

func runServe(addr, dataDir, dbURL, logLevel string) error {
	// Build configuration
	opts := []config.AppConfigOption{}
	if dataDir != "" {
		opts = append(opts, config.WithDataDir(dataDir))
	}
	if dbURL != "" {
		opts = append(opts, config.WithDBURL(dbURL))
	}
	if logLevel != "" {
		opts = append(opts, config.WithLogLevel(logLevel))
	}

	cfg := config.NewAppConfigWithOptions(opts...)

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

	// Register API routes (minimal setup without database)
	router.Route("/api/v1", func(r chi.Router) {
		// Placeholder - in production, use ServerFactory with proper dependencies
	})

	// Health check
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	logger.Info("starting server", slog.String("addr", addr))

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

func runStdio(dataDir, dbURL, logLevel string) error {
	// Build configuration
	opts := []config.AppConfigOption{}
	if dataDir != "" {
		opts = append(opts, config.WithDataDir(dataDir))
	}
	if dbURL != "" {
		opts = append(opts, config.WithDBURL(dbURL))
	}
	if logLevel != "" {
		opts = append(opts, config.WithLogLevel(logLevel))
	}

	_ = config.NewAppConfigWithOptions(opts...)

	// Note: In a full implementation, we would connect to the database here
	// and create the search service with proper dependencies.
	// For now, we create a minimal MCP server with an empty search service.
	var searchService search.Service

	// Create MCP server
	mcpServer := mcp.NewServer(searchService, slog.Default())

	// Run on stdio
	return mcpServer.ServeStdio()
}
