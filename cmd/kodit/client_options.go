package main

import (
	"fmt"
	"net/http"
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
func clientOptions(cfg config.AppConfig) ([]kodit.Option, error) {
	var opts []kodit.Option

	opts = append(opts, storageOptions(cfg)...)

	embOpts, err := embeddingOptions(cfg)
	if err != nil {
		return nil, fmt.Errorf("embedding config: %w", err)
	}
	opts = append(opts, embOpts...)

	txtOpts, err := textOptions(cfg)
	if err != nil {
		return nil, fmt.Errorf("text config: %w", err)
	}
	opts = append(opts, txtOpts...)

	return opts, nil
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
func embeddingOptions(cfg config.AppConfig) ([]kodit.Option, error) {
	endpoint := cfg.EmbeddingEndpoint()
	if endpoint == nil || endpoint.BaseURL() == "" || endpoint.APIKey() == "" {
		return nil, nil
	}

	openaiCfg := provider.OpenAIConfig{
		APIKey:         endpoint.APIKey(),
		BaseURL:        endpoint.BaseURL(),
		EmbeddingModel: endpoint.Model(),
		Timeout:        endpoint.Timeout(),
		MaxRetries:     endpoint.MaxRetries(),
	}
	if cacheDir := cfg.HTTPCacheDir(); cacheDir != "" {
		openaiCfg.HTTPClient = &http.Client{
			Timeout:   endpoint.Timeout(),
			Transport: provider.NewCachingTransport(cacheDir, nil),
		}
	}
	p := provider.NewOpenAIProviderFromConfig(openaiCfg)

	budget, err := search.NewTokenBudget(endpoint.MaxBatchChars())
	if err != nil {
		return nil, fmt.Errorf("max batch chars: %w", err)
	}

	opts := []kodit.Option{
		kodit.WithEmbeddingProvider(p),
		kodit.WithEmbeddingBudget(budget),
		kodit.WithEmbeddingParallelism(endpoint.NumParallelTasks()),
	}

	return opts, nil
}

// textOptions returns a kodit.Option for the text generation provider when the
// enrichment endpoint is fully configured, or an empty slice otherwise.
func textOptions(cfg config.AppConfig) ([]kodit.Option, error) {
	endpoint := cfg.EnrichmentEndpoint()
	if endpoint == nil || endpoint.BaseURL() == "" || endpoint.APIKey() == "" {
		return nil, nil
	}

	txtCfg := provider.OpenAIConfig{
		APIKey:     endpoint.APIKey(),
		BaseURL:    endpoint.BaseURL(),
		ChatModel:  endpoint.Model(),
		Timeout:    endpoint.Timeout(),
		MaxRetries: endpoint.MaxRetries(),
	}
	if cacheDir := cfg.HTTPCacheDir(); cacheDir != "" {
		txtCfg.HTTPClient = &http.Client{
			Timeout:   endpoint.Timeout(),
			Transport: provider.NewCachingTransport(cacheDir, nil),
		}
	}
	p := provider.NewOpenAIProviderFromConfig(txtCfg)

	budget, err := search.NewTokenBudget(endpoint.MaxBatchChars())
	if err != nil {
		return nil, fmt.Errorf("max batch chars: %w", err)
	}

	opts := []kodit.Option{
		kodit.WithTextProvider(p),
		kodit.WithEnrichmentBudget(budget),
		kodit.WithEnrichmentParallelism(endpoint.NumParallelTasks()),
		kodit.WithEnricherParallelism(endpoint.NumParallelTasks()),
	}

	return opts, nil
}

// isSQLite checks if the database URL is for SQLite.
func isSQLite(url string) bool {
	return strings.HasPrefix(url, "sqlite:")
}
