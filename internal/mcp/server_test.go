package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/chunk"
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

// fakeFileContentReader implements FileContentReader with canned content.
type fakeFileContentReader struct {
	content   []byte
	commitSHA string
}

func (f *fakeFileContentReader) Content(_ context.Context, _ int64, _, _ string) (service.BlobContent, error) {
	return service.NewBlobContent(f.content, f.commitSHA), nil
}

// fakeSemanticSearcher implements SemanticSearcher with canned results.
type fakeSemanticSearcher struct {
	enrichments []enrichment.Enrichment
	scores      map[string]float64
}

func (f *fakeSemanticSearcher) SearchCodeWithScores(_ context.Context, _ string, _ int) ([]enrichment.Enrichment, map[string]float64, error) {
	return f.enrichments, f.scores, nil
}

// fakeKeywordSearcher implements KeywordSearcher with canned results.
type fakeKeywordSearcher struct {
	enrichments []enrichment.Enrichment
	scores      map[string]float64
}

func (f *fakeKeywordSearcher) SearchKeywordsWithScores(_ context.Context, _ string, _ int, _ search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	return f.enrichments, f.scores, nil
}

// fakeEnrichmentResolver implements EnrichmentResolver with canned data.
type fakeEnrichmentResolver struct {
	sourceFiles   map[string][]int64
	lineRanges    map[string]chunk.LineRange
	repositoryIDs map[string]int64
}

func (f *fakeEnrichmentResolver) SourceFiles(_ context.Context, _ []int64) (map[string][]int64, error) {
	return f.sourceFiles, nil
}

func (f *fakeEnrichmentResolver) LineRanges(_ context.Context, _ []int64) (map[string]chunk.LineRange, error) {
	return f.lineRanges, nil
}

func (f *fakeEnrichmentResolver) RepositoryIDs(_ context.Context, _ []int64) (map[string]int64, error) {
	return f.repositoryIDs, nil
}

// fakeFileFinder implements FileFinder with canned files.
type fakeFileFinder struct {
	files []repository.File
}

func (f *fakeFileFinder) Find(_ context.Context, _ ...repository.Option) ([]repository.File, error) {
	return f.files, nil
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
		time.Time{},
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
		&fakeFileContentReader{content: []byte("alpha\nbeta\ngamma\ndelta\nepsilon\nzeta\neta"), commitSHA: "abc1234567890"},
		&fakeSemanticSearcher{},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{},
		},
		&fakeFileFinder{},
		"1.0.0-test",
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
	if result.ServerInfo.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", result.ServerInfo.Version)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be present")
	}
	if !containsStr(result.Instructions, "semantic_search") {
		t.Error("expected instructions to mention semantic_search")
	}
}

func TestServer_ListTools(t *testing.T) {
	srv := testServer()

	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/list", 2, nil)

	var result mcp.ListToolsResult
	resultJSON(t, resp, &result)

	if len(result.Tools) != 10 {
		t.Fatalf("expected 10 tools, got %d", len(result.Tools))
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
		"semantic_search",
		"keyword_search",
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
	if text != "1.0.0-test" {
		t.Errorf("expected version 1.0.0-test, got %s", text)
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

func TestServer_ReadFileResource(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "resources/read", 2, map[string]any{
		"uri": "file://1/main/README.md",
	})

	b, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Contents))
	}
	if result.Contents[0].Text != "alpha\nbeta\ngamma\ndelta\nepsilon\nzeta\neta" {
		t.Errorf("expected full content, got %q", result.Contents[0].Text)
	}
	if result.Contents[0].URI != "file://1/main/README.md" {
		t.Errorf("expected URI file://1/main/README.md, got %s", result.Contents[0].URI)
	}
}

func TestServer_ReadFileResource_WithLines(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	text := readResourceText(t, srv, "file://1/main/README.md?lines=L2-L3")
	expected := "beta\ngamma"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestServer_ReadFileResource_WithLineNumbers(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	text := readResourceText(t, srv, "file://1/main/README.md?line_numbers=true")
	expected := "1\talpha\n2\tbeta\n3\tgamma\n4\tdelta\n5\tepsilon\n6\tzeta\n7\teta"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestServer_ReadFileResource_WithLinesAndLineNumbers(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	text := readResourceText(t, srv, "file://1/main/README.md?lines=L2-L3&line_numbers=true")
	expected := "2\tbeta\n3\tgamma"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestServer_ReadFileResource_WithNonContiguousRanges(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	text := readResourceText(t, srv, "file://1/main/README.md?lines=L1-L2,L5&line_numbers=true")
	expected := "1\talpha\n2\tbeta\n...\n5\tepsilon"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestServer_ReadFileResource_WithContiguousRanges(t *testing.T) {
	srv := testServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	text := readResourceText(t, srv, "file://1/main/README.md?lines=L1-L3,L4-L5")
	expected := "alpha\nbeta\ngamma\ndelta\nepsilon"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func semanticSearchServer() *Server {
	e := enrichment.ReconstructEnrichment(
		99,
		enrichment.TypeDevelopment,
		enrichment.SubtypeChunk,
		enrichment.EntityTypeCommit,
		"func handleRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(200)\n}",
		".go",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	testFile := repository.ReconstructFile(
		10, "abc123def456", "src/handler.go", "", "", ".go", ".go", 512,
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	return NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{},
		&fakeFileContentReader{content: []byte("placeholder"), commitSHA: "abc123def456"},
		&fakeSemanticSearcher{
			enrichments: []enrichment.Enrichment{e},
			scores:      map[string]float64{"99": 0.87},
		},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{"99": {10}},
			lineRanges:    map[string]chunk.LineRange{"99": chunk.ReconstructLineRange(1, 99, 10, 25)},
			repositoryIDs: map[string]int64{"99": 1},
		},
		&fakeFileFinder{files: []repository.File{testFile}},
		"1.0.0-test",
		nil,
	)
}

func TestServer_SemanticSearch(t *testing.T) {
	srv := semanticSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "handle HTTP requests",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)

	var items []struct {
		URI      string  `json:"uri"`
		Path     string  `json:"path"`
		Language string  `json:"language"`
		Lines    string  `json:"lines"`
		Score    float64 `json:"score"`
		Preview  string  `json:"preview"`
	}
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("unmarshal semantic search results: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 result, got %d", len(items))
	}
	item := items[0]
	if item.URI != "file://1/abc123def456/src/handler.go?lines=L10-L25&line_numbers=true" {
		t.Errorf("expected URI with line range, got %s", item.URI)
	}
	if item.Path != "src/handler.go" {
		t.Errorf("expected path src/handler.go, got %s", item.Path)
	}
	if item.Language != ".go" {
		t.Errorf("expected language .go, got %s", item.Language)
	}
	if item.Lines != "L10-L25" {
		t.Errorf("expected lines L10-L25, got %s", item.Lines)
	}
	if item.Score != 0.87 {
		t.Errorf("expected score 0.87, got %f", item.Score)
	}
	if item.Preview == "" {
		t.Error("expected non-empty preview")
	}
}

func TestServer_SemanticSearchMissingQuery(t *testing.T) {
	srv := semanticSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name":      "semantic_search",
		"arguments": map[string]any{},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Fatal("expected error response")
	}
	text := textFromContent(t, result)
	if !containsStr(text, "query is required") {
		t.Errorf("expected 'query is required' error, got: %s", text)
	}
}

func TestServer_SemanticSearch_AbsolutePathNormalized(t *testing.T) {
	// File paths stored in the database may contain absolute clone paths
	// (e.g., /root/.kodit/clones/repo-name/bigquery/main.py) from legacy
	// migrations. The semantic_search URI and path fields must use
	// repo-relative paths so that ReadResource works without stripping prefixes.
	e := enrichment.ReconstructEnrichment(
		77,
		enrichment.TypeDevelopment,
		enrichment.SubtypeChunk,
		enrichment.EntityTypeCommit,
		"from google.cloud import bigquery\nclient = bigquery.Client()",
		".py",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	// File record has an absolute clone path — this is the bug trigger.
	absolutePath := "/root/.kodit/clones/my-repo/bigquery/main.py"
	testFile := repository.ReconstructFile(
		20, "def456abc789", absolutePath, "", "", ".py", ".py", 256,
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	srv := NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{},
		&fakeFileContentReader{content: []byte("placeholder"), commitSHA: "def456abc789"},
		&fakeSemanticSearcher{
			enrichments: []enrichment.Enrichment{e},
			scores:      map[string]float64{"77": 0.91},
		},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{"77": {20}},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{"77": 1},
		},
		&fakeFileFinder{files: []repository.File{testFile}},
		"1.0.0-test",
		nil,
	)
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "bigquery client",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)

	var items []struct {
		URI  string `json:"uri"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 result, got %d", len(items))
	}

	// URI and path must use the repo-relative path, not the absolute clone path.
	if items[0].Path != "bigquery/main.py" {
		t.Errorf("path = %s, want bigquery/main.py (repo-relative)", items[0].Path)
	}
	expectedURI := "file://1/def456abc789/bigquery/main.py"
	if items[0].URI != expectedURI {
		t.Errorf("uri = %s, want %s", items[0].URI, expectedURI)
	}
}

func TestServer_SemanticSearch_LanguageFilterDotPrefix(t *testing.T) {
	// The language parameter description says "Filter by file extension (e.g. .go, .py)"
	// so both ".py" and "py" must match enrichments stored with either format.
	// Enrichments may store language with or without the dot prefix depending on
	// the indexing pipeline version, and users may provide either form.
	e := enrichment.ReconstructEnrichment(
		55,
		enrichment.TypeDevelopment,
		enrichment.SubtypeChunk,
		enrichment.EntityTypeCommit,
		"from google.cloud import bigquery\nclient = bigquery.Client()",
		"py", // stored WITHOUT dot
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	testFile := repository.ReconstructFile(
		30, "fff000aaa111", "bigquery/main.py", "", "", ".py", ".py", 128,
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	srv := NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{},
		&fakeFileContentReader{content: []byte("placeholder"), commitSHA: "fff000aaa111"},
		&fakeSemanticSearcher{
			enrichments: []enrichment.Enrichment{e},
			scores:      map[string]float64{"55": 0.90},
		},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{"55": {30}},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{"55": 1},
		},
		&fakeFileFinder{files: []repository.File{testFile}},
		"1.0.0-test",
		nil,
	)
	sendMessage(t, srv, "initialize", 1, initializeParams())

	// User passes ".py" (with dot) but enrichment stores "py" (without dot).
	// The filter should normalize and match.
	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query":    "bigquery client",
			"language": ".py",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)

	var items []struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("language filter '.py' returned %d results, want 1 (enrichment stores 'py')", len(items))
	}
}

func TestServer_SemanticSearch_SourceRepoFilterApplied(t *testing.T) {
	// source_repo with a non-existent repo URL should return empty results (or an
	// error), not silently ignore the filter and return results from other repos.
	srv := semanticSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query":       "handle HTTP requests",
			"source_repo": "https://github.com/nonexistent/fake-repo-12345",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		return // an error response is also acceptable
	}

	text := textFromContent(t, result)
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("source_repo filter for non-existent repo returned %d results, want 0", len(items))
	}
}

func TestServer_SemanticSearch_LimitZeroReturnsEmpty(t *testing.T) {
	// limit: 0 logically means "give me zero results" and should return [].
	srv := semanticSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "handle HTTP requests",
			"limit": 0,
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)
	if text != "[]" {
		t.Errorf("limit 0 returned results, want empty array: %s", text)
	}
}

func TestServer_SemanticSearch_NegativeLimitReturnsError(t *testing.T) {
	// A negative limit is invalid and should return an error, not silently
	// fall back to the default.
	srv := semanticSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "handle HTTP requests",
			"limit": -1,
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Error("expected error for negative limit, got success")
	}
}

func TestServer_SemanticSearch_LimitCapsResults(t *testing.T) {
	// When the underlying search returns more results than the requested limit,
	// the handler must cap the response to exactly limit items.
	e1 := enrichment.ReconstructEnrichment(
		61, enrichment.TypeDevelopment, enrichment.SubtypeChunk, enrichment.EntityTypeCommit,
		"func one() {}", ".go",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	e2 := enrichment.ReconstructEnrichment(
		62, enrichment.TypeDevelopment, enrichment.SubtypeChunk, enrichment.EntityTypeCommit,
		"func two() {}", ".go",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	e3 := enrichment.ReconstructEnrichment(
		63, enrichment.TypeDevelopment, enrichment.SubtypeChunk, enrichment.EntityTypeCommit,
		"func three() {}", ".go",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	f1 := repository.ReconstructFile(101, "aaa", "a.go", "", "", ".go", ".go", 64, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	f2 := repository.ReconstructFile(102, "bbb", "b.go", "", "", ".go", ".go", 64, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	f3 := repository.ReconstructFile(103, "ccc", "c.go", "", "", ".go", ".go", 64, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	srv := NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{},
		&fakeFileContentReader{content: []byte("placeholder"), commitSHA: "aaa"},
		&fakeSemanticSearcher{
			enrichments: []enrichment.Enrichment{e1, e2, e3},
			scores:      map[string]float64{"61": 0.9, "62": 0.8, "63": 0.7},
		},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{"61": {101}, "62": {102}, "63": {103}},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{"61": 1, "62": 1, "63": 1},
		},
		&fakeFileFinder{files: []repository.File{f1, f2, f3}},
		"1.0.0-test",
		nil,
	)
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "functions",
			"limit": 2,
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("limit 2 returned %d results, want 2", len(items))
	}
}

func TestServer_SemanticSearch_EmptyQueryReturnsError(t *testing.T) {
	// An empty query string should return an error, not silently search for everything.
	srv := semanticSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Error("expected error for empty query, got success")
	}
}

// recordingFileContentReader records the arguments passed to Content so tests
// can verify the resource reader receives the correct (normalized) paths.
type recordingFileContentReader struct {
	calls []fileContentCall
	body  map[string][]byte // keyed by filePath
}

type fileContentCall struct {
	repoID   int64
	blobName string
	filePath string
}

func (r *recordingFileContentReader) Content(_ context.Context, repoID int64, blobName, filePath string) (service.BlobContent, error) {
	r.calls = append(r.calls, fileContentCall{repoID, blobName, filePath})
	if b, ok := r.body[filePath]; ok {
		return service.NewBlobContent(b, blobName), nil
	}
	return service.NewBlobContent([]byte("default"), blobName), nil
}

func TestServer_SemanticSearchThenReadFile(t *testing.T) {
	// The typical agent workflow: semantic_search returns URIs, agent reads them.
	// Verify the full round-trip works — the URI from search must resolve
	// through the file resource reader without manual path manipulation.
	fileContent := []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n")
	reader := &recordingFileContentReader{
		body: map[string][]byte{"src/handler.go": fileContent},
	}
	srv := NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{},
		reader,
		&fakeSemanticSearcher{
			enrichments: []enrichment.Enrichment{testEnrichment()},
			scores:      map[string]float64{"42": 0.95},
		},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{"42": {10}},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{"42": 1},
		},
		&fakeFileFinder{files: []repository.File{
			repository.ReconstructFile(10, "abc123def456", "src/handler.go", "", "", ".go", ".go", 512,
				time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		}},
		"1.0.0-test",
		nil,
	)
	sendMessage(t, srv, "initialize", 1, initializeParams())

	// Step 1: semantic_search
	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "handler",
		},
	})
	var searchResult mcp.CallToolResult
	resultJSON(t, resp, &searchResult)
	if searchResult.IsError {
		t.Fatalf("search failed: %s", textFromContent(t, searchResult))
	}

	var items []struct {
		URI  string `json:"uri"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(textFromContent(t, searchResult)), &items); err != nil {
		t.Fatalf("unmarshal search results: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("search returned no results")
	}

	// Step 2: read the URI returned by search
	uri := items[0].URI
	text := readResourceText(t, srv, uri)

	if text != string(fileContent) {
		t.Errorf("resource content = %q, want %q", text, string(fileContent))
	}

	// Verify the resource reader received the repo-relative path, not an absolute one.
	if len(reader.calls) != 1 {
		t.Fatalf("expected 1 Content call, got %d", len(reader.calls))
	}
	call := reader.calls[0]
	if call.repoID != 1 {
		t.Errorf("repoID = %d, want 1", call.repoID)
	}
	if call.filePath != "src/handler.go" {
		t.Errorf("filePath = %s, want src/handler.go", call.filePath)
	}
}

func TestServer_SemanticSearchThenReadFile_AbsolutePath(t *testing.T) {
	// Same round-trip but with a legacy absolute clone path in the database.
	// The URI from search must normalize the path so the resource reader gets
	// the repo-relative path.
	fileContent := []byte("from google.cloud import bigquery\nclient = bigquery.Client()\n")
	reader := &recordingFileContentReader{
		body: map[string][]byte{"bigquery/main.py": fileContent},
	}
	e := enrichment.ReconstructEnrichment(
		77, enrichment.TypeDevelopment, enrichment.SubtypeChunk, enrichment.EntityTypeCommit,
		"from google.cloud import bigquery", ".py",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	srv := NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{},
		reader,
		&fakeSemanticSearcher{
			enrichments: []enrichment.Enrichment{e},
			scores:      map[string]float64{"77": 0.91},
		},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{"77": {20}},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{"77": 1},
		},
		&fakeFileFinder{files: []repository.File{
			// Legacy absolute clone path in the database.
			repository.ReconstructFile(20, "def456abc789", "/root/.kodit/clones/my-repo/bigquery/main.py",
				"", "", ".py", ".py", 256, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		}},
		"1.0.0-test",
		nil,
	)
	sendMessage(t, srv, "initialize", 1, initializeParams())

	// Step 1: semantic_search
	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "bigquery client",
		},
	})
	var searchResult mcp.CallToolResult
	resultJSON(t, resp, &searchResult)
	if searchResult.IsError {
		t.Fatalf("search failed: %s", textFromContent(t, searchResult))
	}

	var items []struct {
		URI  string `json:"uri"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(textFromContent(t, searchResult)), &items); err != nil {
		t.Fatalf("unmarshal search results: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("search returned no results")
	}

	// Step 2: read the URI — this must work without stripping any prefix.
	uri := items[0].URI
	text := readResourceText(t, srv, uri)

	if text != string(fileContent) {
		t.Errorf("resource content = %q, want %q", text, string(fileContent))
	}

	// Verify the reader got the normalized repo-relative path.
	if len(reader.calls) != 1 {
		t.Fatalf("expected 1 Content call, got %d", len(reader.calls))
	}
	if reader.calls[0].filePath != "bigquery/main.py" {
		t.Errorf("filePath = %s, want bigquery/main.py", reader.calls[0].filePath)
	}
}

func TestServer_SemanticSearchThenReadFile_WithLineRange(t *testing.T) {
	// When search results include line ranges, the URI contains ?lines=... parameters.
	// Verify the resource reader applies the line filter correctly.
	fileContent := []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n")
	reader := &recordingFileContentReader{
		body: map[string][]byte{"pkg/core.go": fileContent},
	}
	e := enrichment.ReconstructEnrichment(
		88, enrichment.TypeDevelopment, enrichment.SubtypeChunk, enrichment.EntityTypeCommit,
		"line3\nline4\nline5", ".go",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	srv := NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{},
		reader,
		&fakeSemanticSearcher{
			enrichments: []enrichment.Enrichment{e},
			scores:      map[string]float64{"88": 0.80},
		},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{"88": {15}},
			lineRanges:    map[string]chunk.LineRange{"88": chunk.ReconstructLineRange(1, 88, 3, 5)},
			repositoryIDs: map[string]int64{"88": 1},
		},
		&fakeFileFinder{files: []repository.File{
			repository.ReconstructFile(15, "aaa111bbb222", "pkg/core.go", "", "", ".go", ".go", 100,
				time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		}},
		"1.0.0-test",
		nil,
	)
	sendMessage(t, srv, "initialize", 1, initializeParams())

	// Step 1: semantic_search
	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "core logic",
		},
	})
	var searchResult mcp.CallToolResult
	resultJSON(t, resp, &searchResult)
	if searchResult.IsError {
		t.Fatalf("search failed: %s", textFromContent(t, searchResult))
	}

	var items []struct {
		URI   string `json:"uri"`
		Lines string `json:"lines"`
	}
	if err := json.Unmarshal([]byte(textFromContent(t, searchResult)), &items); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("search returned no results")
	}
	if items[0].Lines != "L3-L5" {
		t.Errorf("lines = %s, want L3-L5", items[0].Lines)
	}

	// Step 2: read the URI with line range parameters
	uri := items[0].URI
	text := readResourceText(t, srv, uri)

	// The URI includes ?lines=L3-L5&line_numbers=true, so expect numbered output.
	expected := "3\tline3\n4\tline4\n5\tline5"
	if text != expected {
		t.Errorf("resource content = %q, want %q", text, expected)
	}
}

func TestServer_SemanticSearchNoResults(t *testing.T) {
	srv := NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{},
		&fakeCommitFinder{},
		&fakeEnrichmentQuery{},
		&fakeFileContentReader{},
		&fakeSemanticSearcher{},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{},
		},
		&fakeFileFinder{},
		"1.0.0-test",
		nil,
	)
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "semantic_search",
		"arguments": map[string]any{
			"query": "nonexistent code",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)
	if text != "[]" {
		t.Errorf("expected empty array, got: %s", text)
	}
}

func keywordSearchServer() *Server {
	e := enrichment.ReconstructEnrichment(
		99,
		enrichment.TypeDevelopment,
		enrichment.SubtypeChunk,
		enrichment.EntityTypeCommit,
		"func handleRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(200)\n}",
		".go",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	testFile := repository.ReconstructFile(
		10, "abc123def456", "src/handler.go", "", "", ".go", ".go", 512,
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	return NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{},
		&fakeFileContentReader{content: []byte("placeholder"), commitSHA: "abc123def456"},
		&fakeSemanticSearcher{},
		&fakeKeywordSearcher{
			enrichments: []enrichment.Enrichment{e},
			scores:      map[string]float64{"99": 0.87},
		},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{"99": {10}},
			lineRanges:    map[string]chunk.LineRange{"99": chunk.ReconstructLineRange(1, 99, 10, 25)},
			repositoryIDs: map[string]int64{"99": 1},
		},
		&fakeFileFinder{files: []repository.File{testFile}},
		"1.0.0-test",
		nil,
	)
}

func TestServer_KeywordSearch(t *testing.T) {
	srv := keywordSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "keyword_search",
		"arguments": map[string]any{
			"keywords": "handleRequest http",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)

	var items []struct {
		URI      string  `json:"uri"`
		Path     string  `json:"path"`
		Language string  `json:"language"`
		Lines    string  `json:"lines"`
		Score    float64 `json:"score"`
		Preview  string  `json:"preview"`
	}
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("unmarshal keyword search results: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 result, got %d", len(items))
	}
	item := items[0]
	if item.URI != "file://1/abc123def456/src/handler.go?lines=L10-L25&line_numbers=true" {
		t.Errorf("expected URI with line range, got %s", item.URI)
	}
	if item.Path != "src/handler.go" {
		t.Errorf("expected path src/handler.go, got %s", item.Path)
	}
	if item.Language != ".go" {
		t.Errorf("expected language .go, got %s", item.Language)
	}
	if item.Lines != "L10-L25" {
		t.Errorf("expected lines L10-L25, got %s", item.Lines)
	}
	if item.Score != 0.87 {
		t.Errorf("expected score 0.87, got %f", item.Score)
	}
	if item.Preview == "" {
		t.Error("expected non-empty preview")
	}
}

func TestServer_KeywordSearch_MissingKeywords(t *testing.T) {
	srv := keywordSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name":      "keyword_search",
		"arguments": map[string]any{},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Fatal("expected error response")
	}
	text := textFromContent(t, result)
	if !containsStr(text, "keywords is required") {
		t.Errorf("expected 'keywords is required' error, got: %s", text)
	}
}

func TestServer_KeywordSearch_WhitespaceOnlyKeywords(t *testing.T) {
	srv := keywordSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "keyword_search",
		"arguments": map[string]any{
			"keywords": "   ",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Fatal("expected error for whitespace-only keywords")
	}
	text := textFromContent(t, result)
	if !containsStr(text, "keywords must not be empty") {
		t.Errorf("expected 'keywords must not be empty' error, got: %s", text)
	}
}

func TestServer_KeywordSearch_EmptyKeywords(t *testing.T) {
	srv := keywordSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "keyword_search",
		"arguments": map[string]any{
			"keywords": "",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Fatal("expected error response")
	}
	text := textFromContent(t, result)
	if !containsStr(text, "keywords must not be empty") {
		t.Errorf("expected 'keywords must not be empty' error, got: %s", text)
	}
}

func TestServer_KeywordSearch_NegativeLimit(t *testing.T) {
	srv := keywordSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "keyword_search",
		"arguments": map[string]any{
			"keywords": "test",
			"limit":    -1,
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if !result.IsError {
		t.Error("expected error for negative limit, got success")
	}
}

func TestServer_KeywordSearch_ZeroLimit(t *testing.T) {
	srv := keywordSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "keyword_search",
		"arguments": map[string]any{
			"keywords": "test",
			"limit":    0,
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)
	if text != "[]" {
		t.Errorf("limit 0 returned results, want empty array: %s", text)
	}
}

func TestServer_KeywordSearch_SourceRepoFilter(t *testing.T) {
	srv := keywordSearchServer()
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "keyword_search",
		"arguments": map[string]any{
			"keywords":    "handleRequest",
			"source_repo": "https://github.com/nonexistent/fake-repo-12345",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		return // an error response is also acceptable
	}

	text := textFromContent(t, result)
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("source_repo filter for non-existent repo returned %d results, want 0", len(items))
	}
}

func TestServer_KeywordSearch_NoResults(t *testing.T) {
	srv := NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{},
		&fakeCommitFinder{},
		&fakeEnrichmentQuery{},
		&fakeFileContentReader{},
		&fakeSemanticSearcher{},
		&fakeKeywordSearcher{},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{},
		},
		&fakeFileFinder{},
		"1.0.0-test",
		nil,
	)
	sendMessage(t, srv, "initialize", 1, initializeParams())

	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "keyword_search",
		"arguments": map[string]any{
			"keywords": "nonexistent",
		},
	})

	var result mcp.CallToolResult
	resultJSON(t, resp, &result)

	if result.IsError {
		t.Fatalf("expected success, got error: %s", textFromContent(t, result))
	}

	text := textFromContent(t, result)
	if text != "[]" {
		t.Errorf("expected empty array, got: %s", text)
	}
}

func TestServer_KeywordSearchThenReadFile(t *testing.T) {
	fileContent := []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n")
	reader := &recordingFileContentReader{
		body: map[string][]byte{"src/handler.go": fileContent},
	}
	e := enrichment.ReconstructEnrichment(
		99,
		enrichment.TypeDevelopment,
		enrichment.SubtypeChunk,
		enrichment.EntityTypeCommit,
		"func handleRequest(ctx context.Context) {}",
		".go",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	testFile := repository.ReconstructFile(
		10, "abc123def456", "src/handler.go", "", "", ".go", ".go", 512,
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	srv := NewServer(
		&fakeSearch{},
		&fakeRepositoryLister{repos: []repository.Repository{testRepo()}},
		&fakeCommitFinder{commits: []repository.Commit{testCommit()}},
		&fakeEnrichmentQuery{},
		reader,
		&fakeSemanticSearcher{},
		&fakeKeywordSearcher{
			enrichments: []enrichment.Enrichment{e},
			scores:      map[string]float64{"99": 0.95},
		},
		&fakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{"99": {10}},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{"99": 1},
		},
		&fakeFileFinder{files: []repository.File{testFile}},
		"1.0.0-test",
		nil,
	)
	sendMessage(t, srv, "initialize", 1, initializeParams())

	// Step 1: keyword_search
	resp := sendMessage(t, srv, "tools/call", 2, map[string]any{
		"name": "keyword_search",
		"arguments": map[string]any{
			"keywords": "handleRequest",
		},
	})
	var searchResult mcp.CallToolResult
	resultJSON(t, resp, &searchResult)
	if searchResult.IsError {
		t.Fatalf("search failed: %s", textFromContent(t, searchResult))
	}

	var items []struct {
		URI  string `json:"uri"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(textFromContent(t, searchResult)), &items); err != nil {
		t.Fatalf("unmarshal search results: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("search returned no results")
	}

	// Step 2: read the URI returned by search
	uri := items[0].URI
	text := readResourceText(t, srv, uri)

	if text != string(fileContent) {
		t.Errorf("resource content = %q, want %q", text, string(fileContent))
	}

	// Verify the resource reader received the repo-relative path.
	if len(reader.calls) != 1 {
		t.Fatalf("expected 1 Content call, got %d", len(reader.calls))
	}
	call := reader.calls[0]
	if call.repoID != 1 {
		t.Errorf("repoID = %d, want 1", call.repoID)
	}
	if call.filePath != "src/handler.go" {
		t.Errorf("filePath = %s, want src/handler.go", call.filePath)
	}
}

// readResourceText is a helper that reads an MCP resource and returns the text content.
func readResourceText(t *testing.T, srv *Server, uri string) string {
	t.Helper()

	resp := sendMessage(t, srv, "resources/read", 2, map[string]any{
		"uri": uri,
	})

	b, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var result struct {
		Contents []struct {
			Text string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Contents))
	}
	return result.Contents[0].Text
}

// Ensure fakes satisfy interfaces at compile time.
var (
	_ Searcher           = (*fakeSearch)(nil)
	_ RepositoryLister   = (*fakeRepositoryLister)(nil)
	_ CommitFinder       = (*fakeCommitFinder)(nil)
	_ EnrichmentQuery    = (*fakeEnrichmentQuery)(nil)
	_ FileContentReader  = (*fakeFileContentReader)(nil)
	_ SemanticSearcher   = (*fakeSemanticSearcher)(nil)
	_ KeywordSearcher    = (*fakeKeywordSearcher)(nil)
	_ EnrichmentResolver = (*fakeEnrichmentResolver)(nil)
	_ FileFinder         = (*fakeFileFinder)(nil)
)
