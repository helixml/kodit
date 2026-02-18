package mcp

import (
	"context"
	"encoding/json"
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
	enrichments    []enrichment.Enrichment
	scores         map[string]float64
	originalScores map[string][]float64
}

func (f *fakeSearch) Search(_ context.Context, _ search.MultiRequest) (service.MultiSearchResult, error) {
	return service.NewMultiSearchResult(f.enrichments, f.scores, f.originalScores), nil
}

// fakeRepositoryLister implements RepositoryLister with canned repos.
type fakeRepositoryLister struct {
	repos []repository.Repository
}

func (f *fakeRepositoryLister) Find(_ context.Context, options ...repository.Option) ([]repository.Repository, error) {
	if len(options) == 0 {
		return f.repos, nil
	}
	q := repository.Build(options...)
	for _, c := range q.Conditions() {
		if c.Field() == "sanitized_remote_uri" {
			url, ok := c.Value().(string)
			if !ok {
				continue
			}
			for _, r := range f.repos {
				if r.RemoteURL() == url {
					return []repository.Repository{r}, nil
				}
			}
			return nil, nil
		}
	}
	return f.repos, nil
}

// fakeCommitFinder implements CommitFinder with canned commits.
type fakeCommitFinder struct {
	commits []repository.Commit
}

func (f *fakeCommitFinder) Find(_ context.Context, _ ...repository.Option) ([]repository.Commit, error) {
	return f.commits, nil
}

// fakeEnrichmentQuery implements EnrichmentQuery with canned enrichments.
type fakeEnrichmentQuery struct {
	enrichments []enrichment.Enrichment
}

func (f *fakeEnrichmentQuery) List(_ context.Context, _ *service.EnrichmentListParams) ([]enrichment.Enrichment, error) {
	return f.enrichments, nil
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

func testArchEnrichment() enrichment.Enrichment {
	return enrichment.ReconstructEnrichment(
		100,
		enrichment.TypeArchitecture,
		enrichment.SubtypePhysical,
		enrichment.EntityTypeCommit,
		"# Architecture\nThis is the architecture doc.",
		"",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
}

func testRepo() repository.Repository {
	return repository.ReconstructRepository(
		1,
		"https://github.com/example/repo",
		repository.WorkingCopy{},
		repository.NewTrackingConfigForBranch("main"),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
}

func testCommit() repository.Commit {
	return repository.ReconstructCommit(
		1,
		"abc1234567890",
		1,
		"initial commit",
		repository.NewAuthor("Test", "test@example.com"),
		repository.NewAuthor("Test", "test@example.com"),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		"",
	)
}

func testServer() *Server {
	e := testEnrichment()
	return NewServer(
		&fakeSearch{
			enrichments:    []enrichment.Enrichment{e},
			scores:         map[string]float64{"42": 0.95},
			originalScores: map[string][]float64{"42": {0.85}},
		},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{enrichments: []enrichment.Enrichment{testArchEnrichment()}},
		"0.1.0-test",
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

	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/list", 2, nil)

	var result mcp.ListToolsResult
	resultJSON(t, resp, &result)

	if len(result.Tools) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(result.Tools))
	}

	tools := map[string]mcp.Tool{}
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
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
		if _, ok := tools[name]; !ok {
			t.Errorf("missing tool: %s", name)
		}
	}

	// Verify search tool parameters
	searchTool := tools["search"]
	props := searchTool.InputSchema.Properties
	if props == nil {
		t.Fatal("search tool has no properties")
	}
	for _, param := range []string{"user_intent", "keywords", "related_file_paths", "related_file_contents"} {
		if _, ok := props[param]; !ok {
			t.Errorf("search tool missing %s parameter", param)
		}
	}
	if !contains(searchTool.InputSchema.Required, "user_intent") {
		t.Error("user_intent should be required")
	}
	if !contains(searchTool.InputSchema.Required, "keywords") {
		t.Error("keywords should be required")
	}
}

func TestServer_Search(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "search",
		"arguments": map[string]any{
			"user_intent":           "hello",
			"keywords":              []string{"hello"},
			"related_file_paths":    []string{},
			"related_file_contents": []string{},
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
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
	if items[0].Score != 0.85 {
		t.Errorf("expected score 0.85, got %f", items[0].Score)
	}
}

func TestServer_SearchMissingUserIntent(t *testing.T) {
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
	if text == "" || !containsStr(text, "user_intent is required") {
		t.Errorf("expected error text containing 'user_intent is required', got: %s", text)
	}
}

func TestServer_GetVersion(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name":      "get_version",
		"arguments": map[string]any{},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error")
	}

	text := textFromContent(t, result)
	if text != "0.1.0-test" {
		t.Errorf("expected version 0.1.0-test, got %s", text)
	}
}

func TestServer_ListRepositories(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name":      "list_repositories",
		"arguments": map[string]any{},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error")
	}

	text := textFromContent(t, result)
	if !containsStr(text, "https://github.com/example/repo") {
		t.Errorf("expected repo URL in output, got: %s", text)
	}
	if !containsStr(text, "tracking branch: main") {
		t.Errorf("expected tracking info in output, got: %s", text)
	}
	if !containsStr(text, "abc1234") {
		t.Errorf("expected short SHA in output, got: %s", text)
	}
}

func TestServer_GetArchitectureDocs(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "get_architecture_docs",
		"arguments": map[string]any{
			"repo_url": "https://github.com/example/repo",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)
	if !containsStr(text, "Architecture") {
		t.Errorf("expected architecture content, got: %s", text)
	}
}

func TestServer_GetArchitectureDocsRepoNotFound(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "get_architecture_docs",
		"arguments": map[string]any{
			"repo_url": "https://github.com/nonexistent/repo",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Fatal("expected error for unknown repo")
	}

	text := textFromContent(t, result)
	if !containsStr(text, "repository not found") {
		t.Errorf("expected 'repository not found' error, got: %s", text)
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

// Ensure fakes satisfy interfaces at compile time.
var (
	_ Searcher         = (*fakeSearch)(nil)
	_ RepositoryLister = (*fakeRepositoryLister)(nil)
	_ CommitFinder     = (*fakeCommitFinder)(nil)
	_ EnrichmentQuery  = (*fakeEnrichmentQuery)(nil)
)
