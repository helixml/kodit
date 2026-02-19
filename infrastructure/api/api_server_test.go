package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helixml/kodit/infrastructure/api"
)

func TestAPIServer_ReadEndpointsOpen_WriteEndpointsProtected(t *testing.T) {
	client := newMCPTestClient(t)
	apiKeys := []string{"test-secret-key"}
	apiServer := api.NewAPIServer(client, apiKeys)
	router := apiServer.Router()

	apiServer.MountRoutes()

	docsRouter := apiServer.DocsRouter("/docs/openapi.json")
	router.Mount("/docs", docsRouter.Routes())

	handler := router

	t.Run("GET /docs returns 200 without API key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("GET /api/v1/repositories returns 200 without API key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/repositories", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("POST /api/v1/repositories without key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"url":"https://github.com/test/repo","branch":"main"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusUnauthorized, w.Body.String())
		}
	})

	t.Run("POST /api/v1/repositories with valid key passes auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/repositories", strings.NewReader(`{"url":"https://github.com/test/repo","branch":"main"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-KEY", "test-secret-key")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Should pass auth — may get a different status depending on the handler
		// logic, but definitely not 401.
		if w.Code == http.StatusUnauthorized {
			t.Errorf("status = %d, should not be 401 with valid key", w.Code)
		}
	})

	t.Run("DELETE /api/v1/repositories/nonexistent without key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/repositories/nonexistent", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusUnauthorized, w.Body.String())
		}
	})

	t.Run("POST /api/v1/search without key returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(`{"query":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Search is open — should not be 401.
		if w.Code == http.StatusUnauthorized {
			t.Errorf("search should be open but got 401")
		}
	})
}
