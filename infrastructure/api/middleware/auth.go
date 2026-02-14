package middleware

import (
	"net/http"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	apiKeys map[string]struct{}
	enabled bool
}

// NewAuthConfig creates a new AuthConfig with a single API key.
func NewAuthConfig(apiKey string) AuthConfig {
	if apiKey == "" {
		return AuthConfig{enabled: false}
	}
	return AuthConfig{
		apiKeys: map[string]struct{}{apiKey: {}},
		enabled: true,
	}
}

// NewAuthConfigWithKeys creates a new AuthConfig with multiple API keys.
func NewAuthConfigWithKeys(apiKeys []string) AuthConfig {
	if len(apiKeys) == 0 {
		return AuthConfig{enabled: false}
	}
	keys := make(map[string]struct{}, len(apiKeys))
	for _, k := range apiKeys {
		if k != "" {
			keys[k] = struct{}{}
		}
	}
	if len(keys) == 0 {
		return AuthConfig{enabled: false}
	}
	return AuthConfig{
		apiKeys: keys,
		enabled: true,
	}
}

// Enabled returns true if authentication is enabled.
func (c AuthConfig) Enabled() bool { return c.enabled }

// APIKey returns a middleware that requires X-API-KEY header authentication.
// If the config has no API keys set, the middleware passes all requests through.
func APIKey(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if not enabled
			if !config.enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Check X-API-KEY header
			apiKey := r.Header.Get("X-API-KEY")
			if apiKey == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"errors":[{"status":"401","title":"Unauthorized","detail":"X-API-KEY header is required"}]}`))
				return
			}

			if _, ok := config.apiKeys[apiKey]; !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"errors":[{"status":"401","title":"Unauthorized","detail":"Invalid API key"}]}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// APIKeyAuth is a convenience function that creates auth middleware from a slice of API keys.
func APIKeyAuth(apiKeys []string) func(http.Handler) http.Handler {
	return APIKey(NewAuthConfigWithKeys(apiKeys))
}
