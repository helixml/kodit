package config

import (
	"testing"
)

func TestNormalizeDBURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "asyncpg suffix",
			raw:  "postgresql+asyncpg://user:pass@host:5432/db",
			want: "postgresql://user:pass@host:5432/db",
		},
		{
			name: "psycopg2 suffix",
			raw:  "postgresql+psycopg2://user:pass@host:5432/db",
			want: "postgresql://user:pass@host:5432/db",
		},
		{
			name: "aiosqlite suffix",
			raw:  "sqlite+aiosqlite:///path/to/db",
			want: "sqlite:///path/to/db",
		},
		{
			name: "already correct postgresql",
			raw:  "postgresql://user:pass@host:5432/db",
			want: "postgresql://user:pass@host:5432/db",
		},
		{
			name: "already correct sqlite",
			raw:  "sqlite:///path/to/db",
			want: "sqlite:///path/to/db",
		},
		{
			name: "empty string",
			raw:  "",
			want: "",
		},
		{
			name: "no scheme",
			raw:  "/just/a/path",
			want: "/just/a/path",
		},
		{
			name: "plus in password not stripped",
			raw:  "postgresql://user:p+ss@host:5432/db",
			want: "postgresql://user:p+ss@host:5432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeDBURL(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeDBURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeModel(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "openrouter prefix",
			raw:  "openrouter/mistralai/mistral-7b",
			want: "mistralai/mistral-7b",
		},
		{
			name: "openai prefix",
			raw:  "openai/text-embedding-3-small",
			want: "text-embedding-3-small",
		},
		{
			name: "anthropic prefix",
			raw:  "anthropic/claude-3-sonnet",
			want: "claude-3-sonnet",
		},
		{
			name: "azure prefix",
			raw:  "azure/gpt-4",
			want: "gpt-4",
		},
		{
			name: "ollama prefix",
			raw:  "ollama/llama2",
			want: "llama2",
		},
		{
			name: "already correct with slash",
			raw:  "mistralai/mistral-7b",
			want: "mistralai/mistral-7b",
		},
		{
			name: "already correct bare model",
			raw:  "text-embedding-3-small",
			want: "text-embedding-3-small",
		},
		{
			name: "empty string",
			raw:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeModel(tt.raw)
			if got != tt.want {
				t.Errorf("normalizeModel(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestEnvConfigNormalize(t *testing.T) {
	env := EnvConfig{
		DBURL: "postgresql+asyncpg://user:pass@host:5432/db",
		EmbeddingEndpoint: EndpointEnv{
			Model: "openai/text-embedding-3-small",
		},
		EnrichmentEndpoint: EndpointEnv{
			Model: "openrouter/mistralai/mistral-7b",
		},
	}

	normalized := env.Normalize()

	if normalized.DBURL != "postgresql://user:pass@host:5432/db" {
		t.Errorf("DBURL = %q, want %q", normalized.DBURL, "postgresql://user:pass@host:5432/db")
	}
	if normalized.EmbeddingEndpoint.Model != "text-embedding-3-small" {
		t.Errorf("EmbeddingEndpoint.Model = %q, want %q",
			normalized.EmbeddingEndpoint.Model, "text-embedding-3-small")
	}
	if normalized.EnrichmentEndpoint.Model != "mistralai/mistral-7b" {
		t.Errorf("EnrichmentEndpoint.Model = %q, want %q",
			normalized.EnrichmentEndpoint.Model, "mistralai/mistral-7b")
	}
}

func TestEnvConfigNormalizeAlreadyCorrect(t *testing.T) {
	env := EnvConfig{
		DBURL: "postgresql://user:pass@host:5432/db",
		EmbeddingEndpoint: EndpointEnv{
			Model: "text-embedding-3-small",
		},
		EnrichmentEndpoint: EndpointEnv{
			Model: "mistralai/mistral-7b",
		},
	}

	normalized := env.Normalize()

	if normalized.DBURL != env.DBURL {
		t.Errorf("DBURL changed: %q -> %q", env.DBURL, normalized.DBURL)
	}
	if normalized.EmbeddingEndpoint.Model != env.EmbeddingEndpoint.Model {
		t.Errorf("EmbeddingEndpoint.Model changed: %q -> %q",
			env.EmbeddingEndpoint.Model, normalized.EmbeddingEndpoint.Model)
	}
	if normalized.EnrichmentEndpoint.Model != env.EnrichmentEndpoint.Model {
		t.Errorf("EnrichmentEndpoint.Model changed: %q -> %q",
			env.EnrichmentEndpoint.Model, normalized.EnrichmentEndpoint.Model)
	}
}

func TestEnvConfigNormalizeEmptyFields(t *testing.T) {
	env := EnvConfig{}
	normalized := env.Normalize()

	if normalized.DBURL != "" {
		t.Errorf("DBURL = %q, want empty", normalized.DBURL)
	}
	if normalized.EmbeddingEndpoint.Model != "" {
		t.Errorf("EmbeddingEndpoint.Model = %q, want empty", normalized.EmbeddingEndpoint.Model)
	}
	if normalized.EnrichmentEndpoint.Model != "" {
		t.Errorf("EnrichmentEndpoint.Model = %q, want empty", normalized.EnrichmentEndpoint.Model)
	}
}
