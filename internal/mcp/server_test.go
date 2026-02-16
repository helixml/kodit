package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/mark3labs/mcp-go/mcp"
)

// fakeSearch implements Searcher with a canned result.
type fakeSearch struct {
	enrichments []enrichment.Enrichment
	scores      map[string]float64
}

func (f *fakeSearch) Search(_ context.Context, _ search.MultiRequest) (service.MultiSearchResult, error) {
	return service.NewMultiSearchResult(f.enrichments, f.scores), nil
}

// fakeEnrichmentLookup implements EnrichmentLookup backed by a map.
type fakeEnrichmentLookup struct {
	items map[int64]enrichment.Enrichment
}

func (f *fakeEnrichmentLookup) Get(_ context.Context, options ...repository.Option) (enrichment.Enrichment, error) {
	q := repository.Build(options...)
	for _, c := range q.Conditions() {
		if c.Field() == "id" {
			id, ok := c.Value().(int64)
			if !ok {
				return enrichment.Enrichment{}, fmt.Errorf("unexpected id type")
			}
			e, found := f.items[id]
			if !found {
				return enrichment.Enrichment{}, fmt.Errorf("enrichment %d not found", id)
			}
			return e, nil
		}
	}
	return enrichment.Enrichment{}, fmt.Errorf("no id condition")
}

// sendMessage marshals a JSON-RPC request, sends it through HandleMessage,
// and returns the JSONRPCResponse. It fatals on marshal failure or unexpected
// response type.
func sendMessage(t *testing.T, srv *Server, method string, id int, params map[string]any) mcp.JSONRPCResponse {
	t.Helper()

	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}

	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	result := srv.MCPServer().HandleMessage(context.Background(), raw)

	resp, ok := result.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T: %+v", result, result)
	}
	return resp
}

// resultJSON re-marshals the Result field through JSON into dst.
func resultJSON(t *testing.T, resp mcp.JSONRPCResponse, dst any) {
	t.Helper()
	b, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatalf("unmarshal result into %T: %v", dst, err)
	}
}

func testEnrichment() enrichment.Enrichment {
	return enrichment.ReconstructEnrichment(
		42,
		enrichment.TypeDevelopment,
		enrichment.SubtypeSnippet,
		enrichment.EntityTypeSnippet,
		"func hello() string { return \"world\" }",
		"go",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
}

func testServer() *Server {
	e := testEnrichment()
	return NewServer(
		&fakeSearch{
			enrichments: []enrichment.Enrichment{e},
			scores:      map[string]float64{"42": 0.95},
		},
		&fakeEnrichmentLookup{
			items: map[int64]enrichment.Enrichment{42: e},
		},
		nil,
	)
}

func initializeParams() map[string]any {
	return map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "test-client",
			"version": "0.0.1",
		},
	}
}

func TestServer_Initialize(t *testing.T) {
	srv := testServer()
	resp := sendMessage(t, srv, "initialize", 1, initializeParams())

	var result mcp.InitializeResult
	resultJSON(t, resp, &result)

	if result.ServerInfo.Name != "kodit" {
		t.Errorf("expected server name kodit, got %s", result.ServerInfo.Name)
	}
	if result.ServerInfo.Version != "0.1.0" {
		t.Errorf("expected version 0.1.0, got %s", result.ServerInfo.Version)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be present")
	}
}

func TestServer_ListTools(t *testing.T) {
	srv := testServer()

	// Must initialize first so that tools/list works.
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/list", 2, nil)

	var result mcp.ListToolsResult
	resultJSON(t, resp, &result)

	if len(result.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result.Tools))
	}

	tools := map[string]mcp.Tool{}
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}

	// Verify search tool
	searchTool, ok := tools["search"]
	if !ok {
		t.Fatal("search tool not found")
	}
	props := searchTool.InputSchema.Properties
	if props == nil {
		t.Fatal("search tool has no properties")
	}
	if _, ok := props["query"]; !ok {
		t.Error("search tool missing query parameter")
	}
	if _, ok := props["top_k"]; !ok {
		t.Error("search tool missing top_k parameter")
	}
	if _, ok := props["language"]; !ok {
		t.Error("search tool missing language parameter")
	}
	required := searchTool.InputSchema.Required
	if !contains(required, "query") {
		t.Error("query should be required")
	}

	// Verify get_snippet tool
	snippetTool, ok := tools["get_snippet"]
	if !ok {
		t.Fatal("get_snippet tool not found")
	}
	snippetProps := snippetTool.InputSchema.Properties
	if snippetProps == nil {
		t.Fatal("get_snippet tool has no properties")
	}
	if _, ok := snippetProps["id"]; !ok {
		t.Error("get_snippet tool missing id parameter")
	}
	if !contains(snippetTool.InputSchema.Required, "id") {
		t.Error("id should be required")
	}
}

func TestServer_Search(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name":      "search",
		"arguments": map[string]any{"query": "hello"},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}

	text := textFromContent(t, result)

	var items []struct {
		ID       string  `json:"id"`
		Content  string  `json:"content"`
		Language string  `json:"language"`
		Score    float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("unmarshal search results: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 result, got %d", len(items))
	}
	if items[0].ID != "42" {
		t.Errorf("expected id 42, got %s", items[0].ID)
	}
	if items[0].Language != "go" {
		t.Errorf("expected language go, got %s", items[0].Language)
	}
	if items[0].Score != 0.95 {
		t.Errorf("expected score 0.95, got %f", items[0].Score)
	}
}

func TestServer_SearchMissingQuery(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name":      "search",
		"arguments": map[string]any{},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Fatal("expected error response")
	}

	text := textFromContent(t, result)
	if text == "" || !containsStr(text, "query is required") {
		t.Errorf("expected error text containing 'query is required', got: %s", text)
	}
}

func TestServer_GetSnippet(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name":      "get_snippet",
		"arguments": map[string]any{"id": "42"},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error")
	}

	text := textFromContent(t, result)

	var snippet struct {
		ID       string `json:"id"`
		Content  string `json:"content"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal([]byte(text), &snippet); err != nil {
		t.Fatalf("unmarshal snippet: %v", err)
	}
	if snippet.ID != "42" {
		t.Errorf("expected id 42, got %s", snippet.ID)
	}
	if snippet.Language != "go" {
		t.Errorf("expected language go, got %s", snippet.Language)
	}
}

func TestServer_GetSnippetNotFound(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name":      "get_snippet",
		"arguments": map[string]any{"id": "999"},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Fatal("expected error response for unknown id")
	}
}

func TestServer_GetSnippetInvalidID(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name":      "get_snippet",
		"arguments": map[string]any{"id": "abc"},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Fatal("expected error response for non-numeric id")
	}

	text := textFromContent(t, result)
	if !containsStr(text, "invalid id") {
		t.Errorf("expected error text containing 'invalid id', got: %s", text)
	}
}

// textFromContent extracts the text string from the first content item
// of a CallToolResult. It round-trips through JSON because in-process
// responses may hold the content as a map rather than a typed struct.
func textFromContent(t *testing.T, result mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("no content in result")
	}
	b, err := json.Marshal(result.Content[0])
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	var tc struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &tc); err != nil {
		t.Fatalf("unmarshal text content: %v", err)
	}
	return tc.Text
}

func contains(items []string, target string) bool {
	for _, s := range items {
		if s == target {
			return true
		}
	}
	return false
}

func containsStr(haystack, needle string) bool {
	return len(haystack) >= len(needle) && searchStr(haystack, needle)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
