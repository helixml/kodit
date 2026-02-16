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
	"fmt"
	"os"

	"github.com/helixml/kodit/internal/config"
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
	cmd.AddCommand(versionCmd())

	return cmd
}

// loadConfig loads configuration from .env file and environment variables.
func loadConfig(envFile string) (config.AppConfig, error) {
	cfg, err := config.LoadConfig(envFile)
	if err != nil {
		return config.AppConfig{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
