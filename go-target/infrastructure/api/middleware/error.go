package middleware

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/helixml/kodit/infrastructure/api"
	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/domain"
)

// JSONAPIError represents a JSON:API error response.
type JSONAPIError struct {
	Status string `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	ID     string `json:"id,omitempty"`
}

// JSONAPIErrorResponse represents a JSON:API error response wrapper.
type JSONAPIErrorResponse struct {
	Errors []JSONAPIError `json:"errors"`
}

// ErrorHandler returns middleware that provides centralized error handling.
func ErrorHandler(logger *slog.Logger) func(http.Handler) http.Handler {
	log := logger
	if log == nil {
		log = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Capture logger for potential panic recovery
			_ = log
			next.ServeHTTP(w, r)
		})
	}
}

// WriteError writes a JSON:API formatted error response.
func WriteError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	status := http.StatusInternalServerError
	title := "Internal Server Error"
	detail := err.Error()

	// Determine status code based on error type
	var apiErr *api.APIError
	var serverErr *api.ServerError
	var authErr *api.AuthenticationError

	switch {
	case errors.As(err, &apiErr):
		status = apiErr.Code()
		title = "API Error"
		detail = apiErr.Message()
	case errors.As(err, &serverErr):
		status = serverErr.StatusCode()
		title = "Server Error"
		detail = serverErr.Message()
	case errors.As(err, &authErr):
		status = http.StatusUnauthorized
		title = "Authentication Failed"
		detail = authErr.Error()
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, database.ErrNotFound):
		status = http.StatusNotFound
		title = "Not Found"
	case errors.Is(err, domain.ErrValidation):
		status = http.StatusBadRequest
		title = "Validation Error"
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
		title = "Conflict"
	}

	correlationID := GetCorrelationID(r.Context())

	if logger != nil {
		logger.Error("request error",
			"correlation_id", correlationID,
			"status", status,
			"error", err.Error(),
			"path", r.URL.Path,
		)
	}

	resp := JSONAPIErrorResponse{
		Errors: []JSONAPIError{
			{
				Status: http.StatusText(status),
				Title:  title,
				Detail: detail,
				ID:     correlationID,
			},
		},
	}

	w.Header().Set("Content-Type", "application/vnd.api+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
