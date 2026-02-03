package middleware

import (
	"net/http"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	apiKey  string
	enabled bool
}

// NewAuthConfig creates a new AuthConfig.
func NewAuthConfig(apiKey string) AuthConfig {
	return AuthConfig{
		apiKey:  apiKey,
		enabled: apiKey != "",
	}
}

// Enabled returns true if authentication is enabled.
func (c AuthConfig) Enabled() bool { return c.enabled }

// APIKey returns a middleware that requires X-API-KEY header authentication.
// If the config has no API key set, the middleware passes all requests through.
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

			if apiKey != config.apiKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"errors":[{"status":"401","title":"Unauthorized","detail":"Invalid API key"}]}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
