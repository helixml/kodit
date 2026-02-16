package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/infrastructure/api"
)

func newMCPTestClient(t *testing.T) *kodit.Client {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(tmpDir),
		kodit.WithSkipProviderValidation(),
	)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func mcpRequest(t *testing.T, method string, id int, params map[string]any) []byte {
	t.Helper()
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return b
}

func postMCP(t *testing.T, handler http.Handler, body []byte, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestMCPEndpoint_Initialize(t *testing.T) {
	client := newMCPTestClient(t)
	apiServer := api.NewAPIServer(client)
	handler := apiServer.Handler()

	body := mcpRequest(t, "initialize", 1, map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})

	w := postMCP(t, handler, body, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
			Capabilities struct {
				Tools json.RawMessage `json:"tools"`
			} `json:"capabilities"`
		} `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Result.ServerInfo.Name != "kodit" {
		t.Errorf("server name = %q, want kodit", resp.Result.ServerInfo.Name)
	}
	if resp.Result.ServerInfo.Version != "0.1.0" {
		t.Errorf("server version = %q, want 0.1.0", resp.Result.ServerInfo.Version)
	}
	if resp.Result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be present")
	}
}

func TestMCPEndpoint_ListTools(t *testing.T) {
	client := newMCPTestClient(t)
	apiServer := api.NewAPIServer(client)
	handler := apiServer.Handler()

	// Initialize first and capture session ID
	initBody := mcpRequest(t, "initialize", 1, map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})
	initResp := postMCP(t, handler, initBody, "")
	sessionID := initResp.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize did not return a session ID")
	}

	// List tools using the session ID
	body := mcpRequest(t, "tools/list", 2, nil)
	w := postMCP(t, handler, body, sessionID)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	names := map[string]bool{}
	for _, tool := range resp.Result.Tools {
		names[tool.Name] = true
	}

	expected := []string{
		"search",
		"get_version",
		"list_repositories",
		"get_architecture_docs",
		"get_api_docs",
		"get_commit_description",
		"get_database_schema",
		"get_cookbook",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing %s tool", name)
		}
	}
	if len(resp.Result.Tools) != 8 {
		t.Errorf("expected 8 tools, got %d", len(resp.Result.Tools))
	}
}

func TestMCPEndpoint_RejectsInvalidContentType(t *testing.T) {
	client := newMCPTestClient(t)
	apiServer := api.NewAPIServer(client)
	handler := apiServer.Handler()

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
