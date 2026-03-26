package kodit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	mcpinternal "github.com/helixml/kodit/internal/mcp"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestScopedMCPServer_RepositoryListFiltered verifies that an MCP server built
// with scoped decorators only returns repositories in the allowed set.
func TestScopedMCPServer_RepositoryListFiltered(t *testing.T) {
	repo1 := repository.ReconstructRepository(
		1, "https://github.com/org/allowed", "https://github.com/org/allowed", "",
		repository.WorkingCopy{}, repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
	)
	repo2 := repository.ReconstructRepository(
		2, "https://github.com/org/forbidden", "https://github.com/org/forbidden", "",
		repository.WorkingCopy{}, repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
	)
	commit := repository.ReconstructCommit(
		1, "abc123", 1, "init", repository.NewAuthor("A", "a@b.c"),
		repository.NewAuthor("A", "a@b.c"), time.Now(), time.Now(), time.Now(), "",
	)

	repos := &scopedFakeRepositoryLister{repos: []repository.Repository{repo1, repo2}}
	fileContent := &scopedFakeFileContentReader{}
	semantic := &scopedFakeSemanticSearcher{}
	keyword := &scopedFakeKeywordSearcher{}
	grepper := &scopedFakeGrepper{}
	fileLister := &scopedFakeFileLister{}

	// Scope to only repo 1.
	scopedRepos, scopedFC, scopedSS, scopedKS, scopedG, scopedFL :=
		mcpinternal.Scope(repos, fileContent, semantic, keyword, grepper, fileLister, []int64{1})

	srv := mcpinternal.NewServer(
		scopedRepos,
		&scopedFakeCommitFinder{commits: []repository.Commit{commit}},
		&scopedFakeEnrichmentQuery{},
		scopedFC,
		scopedSS,
		scopedKS,
		&scopedFakeEnrichmentResolver{
			sourceFiles:   map[string][]int64{},
			lineRanges:    map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{},
		},
		scopedFL,
		&scopedFakeFileFinder{},
		scopedG,
		nil,
		"test",
		zerolog.Nop(),
	)

	// Initialize.
	send(t, srv, "initialize", 0, map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})

	// List repositories — only the allowed one should appear.
	resp := send(t, srv, "tools/call", 1, map[string]any{
		"name":      "kodit_repositories",
		"arguments": map[string]any{},
	})

	var result mcp.CallToolResult
	marshalResult(t, resp, &result)
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}

	text := extractText(t, result)
	if !contains(text, "org/allowed") {
		t.Errorf("expected allowed repo in output, got: %s", text)
	}
	if contains(text, "org/forbidden") {
		t.Errorf("forbidden repo should not appear in output, got: %s", text)
	}
}

// TestScopedMCPServer_ReadResourceBlocked verifies that reading file content
// for an out-of-scope repository returns an error.
func TestScopedMCPServer_ReadResourceBlocked(t *testing.T) {
	repo1 := repository.ReconstructRepository(
		1, "https://github.com/org/allowed", "https://github.com/org/allowed", "",
		repository.WorkingCopy{}, repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
	)

	repos := &scopedFakeRepositoryLister{repos: []repository.Repository{repo1}}
	fileContent := &scopedFakeFileContentReader{content: []byte("secret")}

	_, scopedFC, scopedSS, scopedKS, scopedG, scopedFL :=
		mcpinternal.Scope(repos, fileContent,
			&scopedFakeSemanticSearcher{}, &scopedFakeKeywordSearcher{},
			&scopedFakeGrepper{}, &scopedFakeFileLister{}, []int64{1})

	srv := mcpinternal.NewServer(
		repos, // unscoped repos is fine here — we test file content gating
		&scopedFakeCommitFinder{},
		&scopedFakeEnrichmentQuery{},
		scopedFC,
		scopedSS,
		scopedKS,
		&scopedFakeEnrichmentResolver{
			sourceFiles: map[string][]int64{}, lineRanges: map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{},
		},
		scopedFL,
		&scopedFakeFileFinder{},
		scopedG,
		nil,
		"test",
		zerolog.Nop(),
	)

	send(t, srv, "initialize", 0, map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})

	// Try to read from repo 99 (out of scope).
	resp := send(t, srv, "tools/call", 1, map[string]any{
		"name": "kodit_read_resource",
		"arguments": map[string]any{
			"uri": "file://99/main/secret.txt",
		},
	})

	var result mcp.CallToolResult
	marshalResult(t, resp, &result)
	if !result.IsError {
		t.Error("expected error for out-of-scope repo, got success")
	}
}

// TestScopedMCPServer_NilRepoIDsNoScoping verifies that an empty/nil repoIDs
// set results in unscoped behavior.
func TestScopedMCPServer_NilRepoIDsNoScoping(t *testing.T) {
	repo1 := repository.ReconstructRepository(
		1, "https://github.com/org/repo1", "https://github.com/org/repo1", "",
		repository.WorkingCopy{}, repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
	)
	repo2 := repository.ReconstructRepository(
		2, "https://github.com/org/repo2", "https://github.com/org/repo2", "",
		repository.WorkingCopy{}, repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
	)
	commit := repository.ReconstructCommit(
		1, "abc123", 1, "init", repository.NewAuthor("A", "a@b.c"),
		repository.NewAuthor("A", "a@b.c"), time.Now(), time.Now(), time.Now(), "",
	)

	// No scoping — build server directly with the fakes.
	srv := mcpinternal.NewServer(
		&scopedFakeRepositoryLister{repos: []repository.Repository{repo1, repo2}},
		&scopedFakeCommitFinder{commits: []repository.Commit{commit}},
		&scopedFakeEnrichmentQuery{},
		&scopedFakeFileContentReader{},
		&scopedFakeSemanticSearcher{},
		&scopedFakeKeywordSearcher{},
		&scopedFakeEnrichmentResolver{
			sourceFiles: map[string][]int64{}, lineRanges: map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{},
		},
		&scopedFakeFileLister{},
		&scopedFakeFileFinder{},
		&scopedFakeGrepper{},
		nil,
		"test",
		zerolog.Nop(),
	)

	send(t, srv, "initialize", 0, map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})

	resp := send(t, srv, "tools/call", 1, map[string]any{
		"name":      "kodit_repositories",
		"arguments": map[string]any{},
	})

	var result mcp.CallToolResult
	marshalResult(t, resp, &result)
	text := extractText(t, result)

	if !contains(text, "org/repo1") || !contains(text, "org/repo2") {
		t.Errorf("unscoped server should see all repos, got: %s", text)
	}
}

// --- helpers ---

func send(t *testing.T, srv *mcpinternal.Server, method string, id int, params map[string]any) mcp.JSONRPCResponse {
	t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	result := srv.MCPServer().HandleMessage(context.Background(), raw)
	resp, ok := result.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", result)
	}
	return resp
}

func marshalResult(t *testing.T, resp mcp.JSONRPCResponse, dst any) {
	t.Helper()
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func extractText(t *testing.T, result mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("no content")
	}
	b, _ := json.Marshal(result.Content[0])
	var tc struct{ Text string }
	_ = json.Unmarshal(b, &tc)
	return tc.Text
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestScopedMCPServer_ListRepositories_SanitizesCredentials verifies that the
// MCP kodit_repositories tool returns the sanitized URL (without credentials),
// not the original URL containing embedded secrets.
func TestScopedMCPServer_ListRepositories_SanitizesCredentials(t *testing.T) {
	repo := repository.ReconstructRepository(
		1,
		"http://user:secret-token@api:8080/git/my-repo",
		"http://api:8080/git/my-repo",
		"",
		repository.WorkingCopy{},
		repository.NewTrackingConfigForBranch("main"),
		repository.DefaultChunkingConfig(),
		time.Now(), time.Now(), time.Time{},
	)
	commit := repository.ReconstructCommit(
		1, "abc123", 1, "init", repository.NewAuthor("A", "a@b.c"),
		repository.NewAuthor("A", "a@b.c"), time.Now(), time.Now(), time.Now(), "",
	)

	repos := &scopedFakeRepositoryLister{repos: []repository.Repository{repo}}
	scopedRepos, scopedFC, scopedSS, scopedKS, scopedG, scopedFL :=
		mcpinternal.Scope(repos, &scopedFakeFileContentReader{},
			&scopedFakeSemanticSearcher{}, &scopedFakeKeywordSearcher{},
			&scopedFakeGrepper{}, &scopedFakeFileLister{}, []int64{1})

	srv := mcpinternal.NewServer(
		scopedRepos,
		&scopedFakeCommitFinder{commits: []repository.Commit{commit}},
		&scopedFakeEnrichmentQuery{},
		scopedFC,
		scopedSS,
		scopedKS,
		&scopedFakeEnrichmentResolver{
			sourceFiles: map[string][]int64{}, lineRanges: map[string]chunk.LineRange{},
			repositoryIDs: map[string]int64{},
		},
		scopedFL,
		&scopedFakeFileFinder{},
		scopedG,
		nil,
		"test",
		zerolog.Nop(),
	)

	send(t, srv, "initialize", 0, map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.0.1"},
	})

	resp := send(t, srv, "tools/call", 1, map[string]any{
		"name":      "kodit_repositories",
		"arguments": map[string]any{},
	})

	var result mcp.CallToolResult
	marshalResult(t, resp, &result)

	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}

	text := extractText(t, result)
	if contains(text, "secret-token") {
		t.Errorf("scoped kodit_repositories leaks credentials: %s", text)
	}
	if !contains(text, "http://api:8080/git/my-repo") {
		t.Errorf("expected sanitized URL in output, got: %s", text)
	}
}

// --- fakes (prefixed to avoid collisions with internal/mcp fakes) ---

type scopedFakeRepositoryLister struct {
	repos []repository.Repository
}

func (f *scopedFakeRepositoryLister) Find(_ context.Context, options ...repository.Option) ([]repository.Repository, error) {
	if len(options) == 0 {
		return f.repos, nil
	}
	q := repository.Build(options...)
	result := f.repos
	for _, c := range q.Conditions() {
		if c.Field() == "id" && c.In() {
			ids, ok := c.Value().([]int64)
			if !ok {
				continue
			}
			set := make(map[int64]struct{}, len(ids))
			for _, id := range ids {
				set[id] = struct{}{}
			}
			var filtered []repository.Repository
			for _, r := range result {
				if _, exists := set[r.ID()]; exists {
					filtered = append(filtered, r)
				}
			}
			result = filtered
		}
		if c.Field() == "sanitized_remote_uri" {
			url, ok := c.Value().(string)
			if !ok {
				continue
			}
			var filtered []repository.Repository
			for _, r := range result {
				if r.SanitizedURL() == url {
					filtered = append(filtered, r)
				}
			}
			result = filtered
		}
	}
	return result, nil
}

type scopedFakeCommitFinder struct {
	commits []repository.Commit
}

func (f *scopedFakeCommitFinder) Find(_ context.Context, _ ...repository.Option) ([]repository.Commit, error) {
	return f.commits, nil
}

type scopedFakeEnrichmentQuery struct {
	enrichments []enrichment.Enrichment
}

func (f *scopedFakeEnrichmentQuery) List(_ context.Context, _ *service.EnrichmentListParams) ([]enrichment.Enrichment, error) {
	return f.enrichments, nil
}

type scopedFakeFileContentReader struct {
	content []byte
}

func (f *scopedFakeFileContentReader) Content(_ context.Context, _ int64, blobName, _ string) (service.BlobContent, error) {
	return service.NewBlobContent(f.content, blobName), nil
}

type scopedFakeSemanticSearcher struct{}

func (f *scopedFakeSemanticSearcher) SearchCodeWithScores(_ context.Context, _ string, _ int, _ search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	return nil, nil, nil
}

type scopedFakeKeywordSearcher struct{}

func (f *scopedFakeKeywordSearcher) SearchKeywordsWithScores(_ context.Context, _ string, _ int, _ search.Filters) ([]enrichment.Enrichment, map[string]float64, error) {
	return nil, nil, nil
}

type scopedFakeEnrichmentResolver struct {
	sourceFiles   map[string][]int64
	lineRanges    map[string]chunk.LineRange
	repositoryIDs map[string]int64
}

func (f *scopedFakeEnrichmentResolver) SourceFiles(_ context.Context, _ []int64) (map[string][]int64, error) {
	return f.sourceFiles, nil
}

func (f *scopedFakeEnrichmentResolver) LineRanges(_ context.Context, _ []int64) (map[string]chunk.LineRange, error) {
	return f.lineRanges, nil
}

func (f *scopedFakeEnrichmentResolver) RepositoryIDs(_ context.Context, _ []int64) (map[string]int64, error) {
	return f.repositoryIDs, nil
}

type scopedFakeFileFinder struct{}

func (f *scopedFakeFileFinder) Find(_ context.Context, _ ...repository.Option) ([]repository.File, error) {
	return nil, nil
}

type scopedFakeGrepper struct{}

func (f *scopedFakeGrepper) Search(_ context.Context, _ int64, _ string, _ string, _ int) ([]service.GrepResult, error) {
	return nil, nil
}

type scopedFakeFileLister struct{}

func (f *scopedFakeFileLister) ListFiles(_ context.Context, _ int64, _ string) ([]service.FileEntry, error) {
	return nil, nil
}
