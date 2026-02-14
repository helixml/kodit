package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	logger := slog.Default()
	server := NewServer(":8080", logger)

	if server.Addr() != ":8080" {
		t.Errorf("Addr() = %v, want :8080", server.Addr())
	}

	router := server.Router()
	if router == nil {
		t.Error("Router() returned nil")
	}
}

func TestServer_HealthCheck(t *testing.T) {
	logger := slog.Default()
	server := NewServer(":0", logger)
	router := server.Router()

	// Add a health endpoint
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	// Test using httptest
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	expected := `{"status":"healthy"}`
	if body != expected {
		t.Errorf("body = %v, want %v", body, expected)
	}
}

func TestServer_NotFound(t *testing.T) {
	logger := slog.Default()
	server := NewServer(":0", logger)
	router := server.Router()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestServer_Shutdown(t *testing.T) {
	logger := slog.Default()
	server := NewServer(":0", logger)

	// Test that shutdown works without starting
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v, want nil", err)
	}
}
