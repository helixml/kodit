package main

import (
	"strings"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/provider"
	"github.com/helixml/kodit/internal/config"
)

// clientOptions returns the kodit.Option slice derived from the shared parts
// of AppConfig: database storage, embedding provider, and text provider.
// Callers append entrypoint-specific options (API keys, worker count, etc.)
// before passing the full slice to kodit.New.
func clientOptions(cfg config.AppConfig) []kodit.Option {
	var opts []kodit.Option

	opts = append(opts, storageOptions(cfg)...)
	opts = append(opts, embeddingOptions(cfg)...)
	opts = append(opts, textOptions(cfg)...)

	return opts
}

// storageOptions returns the kodit.Option for the configured database backend.
func storageOptions(cfg config.AppConfig) []kodit.Option {
	dbURL := cfg.DBURL()

	if dbURL != "" && !isSQLite(dbURL) {
		return []kodit.Option{kodit.WithPostgresVectorchord(dbURL)}
	}

	dbPath := cfg.DataDir() + "/kodit.db"
	if dbURL != "" && isSQLite(dbURL) {
		dbPath = strings.TrimPrefix(dbURL, "sqlite:///")
		if dbPath == dbURL {
			dbPath = strings.TrimPrefix(dbURL, "sqlite:")
		}
	}

	return []kodit.Option{kodit.WithSQLite(dbPath)}
}

// embeddingOptions returns a kodit.Option for the embedding provider when the
// embedding endpoint is fully configured, or an empty slice otherwise.
func embeddingOptions(cfg config.AppConfig) []kodit.Option {
	endpoint := cfg.EmbeddingEndpoint()
	if endpoint == nil || endpoint.BaseURL() == "" || endpoint.APIKey() == "" {
		return nil
	}

	p := provider.NewOpenAIProviderFromConfig(provider.OpenAIConfig{
		APIKey:         endpoint.APIKey(),
		BaseURL:        endpoint.BaseURL(),
		EmbeddingModel: endpoint.Model(),
		Timeout:        endpoint.Timeout(),
		MaxRetries:     endpoint.MaxRetries(),
	})

	opts := []kodit.Option{kodit.WithEmbeddingProvider(p)}

	if budget, err := search.NewTokenBudget(endpoint.MaxBatchChars()); err == nil {
		opts = append(opts, kodit.WithEmbeddingBudget(budget))
	}

	opts = append(opts, kodit.WithEmbeddingParallelism(endpoint.NumParallelTasks()))

	return opts
}

// textOptions returns a kodit.Option for the text generation provider when the
// enrichment endpoint is fully configured, or an empty slice otherwise.
func textOptions(cfg config.AppConfig) []kodit.Option {
	endpoint := cfg.EnrichmentEndpoint()
	if endpoint == nil || endpoint.BaseURL() == "" || endpoint.APIKey() == "" {
		return nil
	}

	p := provider.NewOpenAIProviderFromConfig(provider.OpenAIConfig{
		APIKey:     endpoint.APIKey(),
		BaseURL:    endpoint.BaseURL(),
		ChatModel:  endpoint.Model(),
		Timeout:    endpoint.Timeout(),
		MaxRetries: endpoint.MaxRetries(),
	})

	opts := []kodit.Option{kodit.WithTextProvider(p)}

	if budget, err := search.NewTokenBudget(endpoint.MaxBatchChars()); err == nil {
		opts = append(opts, kodit.WithEnrichmentBudget(budget))
	}

	opts = append(opts, kodit.WithEnrichmentParallelism(endpoint.NumParallelTasks()))

	return opts
}

// isSQLite checks if the database URL is for SQLite.
func isSQLite(url string) bool {
	return strings.HasPrefix(url, "sqlite:")
}
