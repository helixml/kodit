package main

import (
	"fmt"
	"log/slog"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/log"
	"github.com/helixml/kodit/internal/mcp"
	"github.com/spf13/cobra"
)

func stdioCmd() *cobra.Command {
	var envFile string

	cmd := &cobra.Command{
		Use:   "stdio",
		Short: "Start MCP server on stdio",
		Long: `Start the MCP (Model Context Protocol) server on stdio.

This allows AI assistants to interact with Kodit for code search and understanding.
Configuration is loaded from environment variables and .env file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStdio(envFile)
		},
	}

	cmd.Flags().StringVar(&envFile, "env-file", "", "Path to .env file")

	return cmd
}

func runStdio(envFile string) error {
	// Load configuration
	cfg, err := loadConfig(envFile)
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

	// Build kodit client options
	opts := []kodit.Option{
		kodit.WithDataDir(cfg.DataDir()),
		kodit.WithLogger(slogger),
	}

	// Configure storage based on database URL
	if cfg.DBURL() != "" {
		// Assume VectorChord for PostgreSQL databases (default for kodit)
		opts = append(opts, kodit.WithPostgresVectorchord(cfg.DBURL()))
	} else {
		// Fall back to SQLite
		opts = append(opts, kodit.WithSQLite(cfg.DataDir()+"/kodit.db"))
	}

	// Configure embedding provider if available
	embEndpoint := cfg.EmbeddingEndpoint()
	if embEndpoint != nil && embEndpoint.BaseURL() != "" && embEndpoint.APIKey() != "" {
		opts = append(opts, kodit.WithOpenAIConfig(provider.OpenAIConfig{
			APIKey:         embEndpoint.APIKey(),
			BaseURL:        embEndpoint.BaseURL(),
			EmbeddingModel: embEndpoint.Model(),
			Timeout:        embEndpoint.Timeout(),
			MaxRetries:     embEndpoint.MaxRetries(),
		}))
	}

	// Create kodit client
	client, err := kodit.New(opts...)
	if err != nil {
		return fmt.Errorf("create kodit client: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			slogger.Error("failed to close kodit client", slog.Any("error", err))
		}
	}()

	// Check code search availability
	if !client.Search.Available() {
		slogger.Warn("code search service not available - search will not work")
		return fmt.Errorf("code search service not available: configure database and embedding provider")
	}

	// Create MCP server
	mcpServer := mcp.NewServer(client.Search, client.Enrichments, slogger)

	// Run on stdio
	return mcpServer.ServeStdio()
}
