package config

import (
	"log/slog"
	"strings"
)

// litellmPrefixes are provider prefixes used by Python's litellm library.
// Go endpoints determine the provider from BASE_URL, so these prefixes
// must be stripped from model names.
var litellmPrefixes = []string{
	"openrouter/",
	"openai/",
	"anthropic/",
	"azure/",
	"huggingface/",
	"together_ai/",
	"replicate/",
	"cohere/",
	"bedrock/",
	"vertex_ai/",
	"sagemaker/",
	"ollama/",
}

// normalizeDBURL strips SQLAlchemy async driver suffixes from database URLs.
// Python used drivers like postgresql+asyncpg:// which are invalid in Go.
func normalizeDBURL(raw string) string {
	plus := strings.Index(raw, "+")
	if plus < 0 {
		return raw
	}
	colon := strings.Index(raw, "://")
	if colon < 0 || plus > colon {
		return raw
	}
	scheme := raw[:plus]
	rest := raw[colon:]
	slog.Warn("normalized legacy DB_URL driver suffix",
		"original", raw,
		"normalized", scheme+rest,
	)
	return scheme + rest
}

// normalizeModel strips known litellm provider prefixes from a model name.
// Python's litellm used prefixes like openrouter/ to route to providers,
// but Go determines the provider from BASE_URL.
func normalizeModel(raw string) string {
	for _, prefix := range litellmPrefixes {
		if strings.HasPrefix(raw, prefix) {
			normalized := raw[len(prefix):]
			slog.Warn("normalized legacy litellm model prefix",
				"original", raw,
				"normalized", normalized,
			)
			return normalized
		}
	}
	return raw
}

// Normalize returns a copy of the EnvConfig with legacy Python-format values
// converted to their Go equivalents. It logs warnings for each transformation
// so users know to update their .env files.
func (e EnvConfig) Normalize() EnvConfig {
	if e.DBURL != "" {
		e.DBURL = normalizeDBURL(e.DBURL)
	}
	if e.EmbeddingEndpoint.Model != "" {
		e.EmbeddingEndpoint.Model = normalizeModel(e.EmbeddingEndpoint.Model)
	}
	if e.EnrichmentEndpoint.Model != "" {
		e.EnrichmentEndpoint.Model = normalizeModel(e.EnrichmentEndpoint.Model)
	}
	return e
}
