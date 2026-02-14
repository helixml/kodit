package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// CorrelationIDKey is the context key for the correlation ID.
type CorrelationIDKey struct{}

// CorrelationID returns a middleware that adds correlation ID to the request context.
// Uses chi's RequestID if available, otherwise creates a new one.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get correlation ID from header
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			// Fall back to chi's request ID
			correlationID = middleware.GetReqID(r.Context())
		}

		// Add to response header
		w.Header().Set("X-Correlation-ID", correlationID)

		// Add to context
		ctx := context.WithValue(r.Context(), CorrelationIDKey{}, correlationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetCorrelationID retrieves the correlation ID from the context.
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey{}).(string); ok {
		return id
	}
	return ""
}
