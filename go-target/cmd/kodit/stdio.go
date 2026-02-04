package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/infrastructure/persistence"
	"github.com/helixml/kodit/infrastructure/provider"
	infraSearch "github.com/helixml/kodit/infrastructure/search"
	"github.com/helixml/kodit/internal/log"
	"github.com/helixml/kodit/internal/mcp"
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
	db, err := persistence.NewDatabase(ctx, cfg.DBURL())
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slogger.Error("failed to close database", slog.Any("error", err))
		}
	}()

	// Create stores
	snippetStore := persistence.NewSnippetStore(db)
	enrichmentStore := persistence.NewEnrichmentStore(db)

	// Create search service
	var searchService service.CodeSearch
	if db.IsPostgres() {
		var embedder provider.Embedder
		embEndpoint := cfg.EmbeddingEndpoint()
		if embEndpoint != nil && embEndpoint.BaseURL() != "" && embEndpoint.APIKey() != "" {
			embedder = provider.NewOpenAIProviderFromConfig(provider.OpenAIConfig{
				APIKey:         embEndpoint.APIKey(),
				BaseURL:        embEndpoint.BaseURL(),
				EmbeddingModel: embEndpoint.Model(),
				Timeout:        embEndpoint.Timeout(),
				MaxRetries:     embEndpoint.MaxRetries(),
			})
		}

		bm25Store := infraSearch.NewVectorChordBM25Store(db.GORM(), slogger)
		vectorStore := infraSearch.NewVectorChordVectorStore(db.GORM(), infraSearch.TaskNameCode, embedder, slogger)
		searchService = service.NewCodeSearch(bm25Store, vectorStore, snippetStore, enrichmentStore, slogger)
	} else {
		searchService = service.NewCodeSearch(nil, nil, snippetStore, enrichmentStore, slogger)
	}

	// Create MCP server with database-backed search and snippet store
	mcpServer := mcp.NewServer(searchService, snippetStore, slogger)

	// Run on stdio
	return mcpServer.ServeStdio()
}
