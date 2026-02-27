package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
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
	apiServer := api.NewAPIServer(client, nil)
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
	if resp.Result.ServerInfo.Version != "1.0.0" {
		t.Errorf("server version = %q, want 1.0.0", resp.Result.ServerInfo.Version)
	}
	if resp.Result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be present")
	}
}

func TestMCPEndpoint_ListTools(t *testing.T) {
	client := newMCPTestClient(t)
	apiServer := api.NewAPIServer(client, nil)
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
		"semantic_search",
		"keyword_search",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing %s tool", name)
		}
	}
	if len(resp.Result.Tools) != 10 {
		t.Errorf("expected 10 tools, got %d", len(resp.Result.Tools))
	}
}

func TestMCPEndpoint_RejectsInvalidContentType(t *testing.T) {
	client := newMCPTestClient(t)
	apiServer := api.NewAPIServer(client, nil)
	handler := apiServer.Handler()

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// initMCPSession sends an initialize request and returns the session ID.
func initMCPSession(t *testing.T, handler http.Handler) string {
	t.Helper()
	body := mcpRequest(t, "initialize", 1, map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})
	w := postMCP(t, handler, body, "")
	if w.Code != http.StatusOK {
		t.Fatalf("initialize: status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	sessionID := w.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize did not return a session ID")
	}
	return sessionID
}

// toolResultText decodes the JSON-RPC response from a tools/call and returns
// the text content and whether the tool reported an error.
func toolResultText(t *testing.T, w *httptest.ResponseRecorder) (string, bool) {
	t.Helper()
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	if len(resp.Result.Content) == 0 {
		return "", resp.Result.IsError
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}

// TestMCPEndpoint_ToolCallResolvesLatestCommit verifies that enrichment tools
// query the database for the latest commit using the correct column name.
// Before the fix, the query used a nonexistent "committed_at" column, causing
// a SQL error: ERROR: column "committed_at" does not exist (SQLSTATE 42703).
func TestMCPEndpoint_ToolCallResolvesLatestCommit(t *testing.T) {
	client := newMCPTestClient(t)
	ctx := context.Background()

	// Add a repository so the tool can find it by URL.
	_, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{
		URL:    "https://github.com/test/commit-column-test",
		Branch: "main",
	})
	if err != nil {
		t.Fatalf("add repository: %v", err)
	}

	apiServer := api.NewAPIServer(client, nil)
	handler := apiServer.Handler()
	sessionID := initMCPSession(t, handler)

	// Call get_architecture_docs WITHOUT a commit_sha. This forces the handler
	// to query for the latest commit, exercising the ORDER BY clause that
	// previously referenced the wrong column.
	body := mcpRequest(t, "tools/call", 2, map[string]any{
		"name": "get_architecture_docs",
		"arguments": map[string]any{
			"repo_url": "https://github.com/test/commit-column-test",
		},
	})
	w := postMCP(t, handler, body, sessionID)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	text, isError := toolResultText(t, w)

	// The repo has no commits, so we expect a "no commits found" tool-level
	// error. The critical assertion is that we do NOT get a SQL error about
	// a missing "committed_at" column.
	if isError && strings.Contains(text, "committed_at") {
		t.Fatalf("query used nonexistent 'committed_at' column: %s", text)
	}
	if isError && !strings.Contains(text, "no commits found") {
		t.Fatalf("unexpected tool error: %s", text)
	}
}

// TestMCPEndpoint_ServerMiddlewareStack verifies that MCP works through the
// full server middleware stack (as built by ListenAndServe). Previously, chi's
// Timeout middleware wrapped the MCP StreamableHTTPServer's ResponseWriter,
// causing "superfluous response.WriteHeader" errors because MCP manages its
// own response headers for session state.
func TestMCPEndpoint_ServerMiddlewareStack(t *testing.T) {
	client := newMCPTestClient(t)
	apiServer := api.NewAPIServer(client, nil)
	apiServer.MountRoutes()

	// Build the same handler stack as ListenAndServe: the Server router
	// (with RequestID, RealIP, Recoverer) wrapping the APIServer routes.
	srv := api.NewServer("", nil)
	srv.Router().Mount("/", apiServer.Router())
	handler := srv.Router()

	// Initialize — must succeed and return a session ID.
	sessionID := initMCPSession(t, handler)

	// List tools using the session — verifies session state survives the
	// middleware stack.
	body := mcpRequest(t, "tools/list", 2, nil)
	w := postMCP(t, handler, body, sessionID)
	if w.Code != http.StatusOK {
		t.Fatalf("tools/list: status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Call a tool to confirm end-to-end through the middleware stack.
	callBody := mcpRequest(t, "tools/call", 3, map[string]any{
		"name":      "get_version",
		"arguments": map[string]any{},
	})
	w = postMCP(t, handler, callBody, sessionID)
	if w.Code != http.StatusOK {
		t.Fatalf("tools/call: status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	text, isError := toolResultText(t, w)
	if isError {
		t.Fatalf("get_version returned error: %s", text)
	}
	if text != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", text)
	}
}
