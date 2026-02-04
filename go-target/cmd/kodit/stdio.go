package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/internal/database"
	enrichmentpg "github.com/helixml/kodit/internal/enrichment/postgres"
	"github.com/helixml/kodit/internal/indexing/bm25"
	indexingpg "github.com/helixml/kodit/internal/indexing/postgres"
	"github.com/helixml/kodit/internal/indexing/vector"
	"github.com/helixml/kodit/internal/log"
	"github.com/helixml/kodit/internal/mcp"
	"github.com/helixml/kodit/internal/provider"
	"github.com/helixml/kodit/internal/search"
	"github.com/spf13/cobra"
)

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

	// Create MCP server with database-backed search and snippet repository
	mcpServer := mcp.NewServer(searchService, snippetRepo, slogger)

	// Run on stdio
	return mcpServer.ServeStdio()
}
