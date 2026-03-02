package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

func TestSearch_POST_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)

	text := "authentication"
	body := dto.SearchRequest{
		Data: dto.SearchData{
			Type: "search",
			Attributes: dto.SearchAttributes{
				Text:  &text,
				Limit: intPtr(10),
			},
		},
	}

	resp := ts.POST("/api/v1/search", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	// With fake repositories, we get empty results
	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestSearch_POST_WithFilters(t *testing.T) {
	ts := NewTestServer(t)

	text := "login"
	body := dto.SearchRequest{
		Data: dto.SearchData{
			Type: "search",
			Attributes: dto.SearchAttributes{
				Text:  &text,
				Limit: intPtr(5),
				Filters: &dto.SearchFilters{
					Languages: []string{"python"},
					Authors:   []string{"john"},
				},
			},
		},
	}

	resp := ts.POST("/api/v1/search", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	// With fake repositories, filters don't matter - we get empty results
	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestSearch_POST_WithTextAndCodeQuery(t *testing.T) {
	ts := NewTestServer(t)

	text := "user authentication"
	code := "def authenticate"
	body := dto.SearchRequest{
		Data: dto.SearchData{
			Type: "search",
			Attributes: dto.SearchAttributes{
				Text:  &text,
				Code:  &code,
				Limit: intPtr(10),
			},
		},
	}

	resp := ts.POST("/api/v1/search", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	// With fake repositories, we get empty results
	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestSearch_POST_WithKeywords(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.SearchRequest{
		Data: dto.SearchData{
			Type: "search",
			Attributes: dto.SearchAttributes{
				Keywords: []string{"auth", "login", "security"},
				Limit:    intPtr(10),
			},
		},
	}

	resp := ts.POST("/api/v1/search", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	// With fake repositories, we get empty results
	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestSearch_POST_DerivesFromPopulated(t *testing.T) {
	ts := NewTestServer(t)

	// Seed a repository, commit, and file
	repo := ts.CreateRepository("https://github.com/test/derives-from-repo")
	commit := ts.CreateCommit(repo, "abc123def456", "add authentication module")
	file := ts.CreateFile(commit.SHA(), "pkg/auth/handler.go", "deadbeef1234", "text/x-go", ".go", 512)

	// Create a snippet enrichment associated with the commit
	snippet := ts.CreateSnippetEnrichmentForCommit(commit.SHA(), "func Authenticate(token string) error", "go")

	// Associate the snippet enrichment with the source file
	ts.CreateEnrichmentAssociation(snippet, enrichment.EntityTypeFile, fmt.Sprintf("%d", file.ID()))

	// Seed BM25 so keyword search finds this snippet
	ts.SeedBM25(fmt.Sprintf("%d", snippet.ID()), "func Authenticate token string error")

	// Search using a keyword that matches the BM25 content
	body := dto.SearchRequest{
		Data: dto.SearchData{
			Type: "search",
			Attributes: dto.SearchAttributes{
				Keywords: []string{"Authenticate"},
				Limit:    intPtr(10),
			},
		},
	}

	resp := ts.POST("/api/v1/search", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) == 0 {
		t.Fatal("expected at least one search result")
	}

	found := false
	for _, d := range result.Data {
		if len(d.Attributes.DerivesFrom) == 0 {
			continue
		}
		for _, df := range d.Attributes.DerivesFrom {
			if df.BlobSHA == file.BlobSHA() && df.Path == file.Path() {
				found = true
			}
		}
	}

	if !found {
		t.Errorf("derives_from did not contain expected file (blob_sha=%s, path=%s)", file.BlobSHA(), file.Path())
	}
}

func TestSemanticSearch_POST_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.SemanticSearchRequest{
		Data: dto.SemanticSearchData{
			Type: "semantic_search",
			Attributes: dto.SemanticSearchAttributes{
				Query: "authentication handler",
			},
		},
	}

	resp := ts.POST("/api/v1/search/semantic", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	// No indexed data, so semantic search returns empty
	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestSemanticSearch_POST_MissingQuery(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.SemanticSearchRequest{
		Data: dto.SemanticSearchData{
			Type: "semantic_search",
			Attributes: dto.SemanticSearchAttributes{
				Query: "",
			},
		},
	}

	resp := ts.POST("/api/v1/search/semantic", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestKeywordSearch_POST_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.KeywordSearchRequest{
		Data: dto.KeywordSearchData{
			Type: "keyword_search",
			Attributes: dto.KeywordSearchAttributes{
				Keywords: "nonexistent_xyzzy_keyword",
			},
		},
	}

	resp := ts.POST("/api/v1/search/keyword", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestKeywordSearch_POST_MissingKeywords(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.KeywordSearchRequest{
		Data: dto.KeywordSearchData{
			Type: "keyword_search",
			Attributes: dto.KeywordSearchAttributes{
				Keywords: "",
			},
		},
	}

	resp := ts.POST("/api/v1/search/keyword", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestKeywordSearch_POST_WithSeededData(t *testing.T) {
	ts := NewTestServer(t)

	// Seed a repository, commit, and file
	repo := ts.CreateRepository("https://github.com/test/keyword-search-repo")
	commit := ts.CreateCommit(repo, "abc123keyword456", "add keyword search module")
	file := ts.CreateFile(commit.SHA(), "pkg/search/handler.go", "deadbeef5678", "text/x-go", ".go", 256)

	// Create a snippet enrichment associated with the commit
	snippet := ts.CreateSnippetEnrichmentForCommit(commit.SHA(), "func SearchKeywords(query string) []Result", "go")

	// Associate the snippet enrichment with the source file
	ts.CreateEnrichmentAssociation(snippet, enrichment.EntityTypeFile, fmt.Sprintf("%d", file.ID()))

	// Seed BM25 so keyword search finds this snippet
	ts.SeedBM25(fmt.Sprintf("%d", snippet.ID()), "func SearchKeywords query string Result")

	body := dto.KeywordSearchRequest{
		Data: dto.KeywordSearchData{
			Type: "keyword_search",
			Attributes: dto.KeywordSearchAttributes{
				Keywords: "SearchKeywords",
			},
		},
	}

	resp := ts.POST("/api/v1/search/keyword", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) == 0 {
		t.Fatal("expected at least one keyword search result")
	}

	found := false
	for _, d := range result.Data {
		for _, df := range d.Attributes.DerivesFrom {
			if df.BlobSHA == file.BlobSHA() && df.Path == file.Path() {
				found = true
			}
		}
	}

	if !found {
		t.Errorf("derives_from did not contain expected file (blob_sha=%s, path=%s)", file.BlobSHA(), file.Path())
	}
}

func TestKeywordSearch_POST_WithLanguageFilter(t *testing.T) {
	ts := NewTestServer(t)

	repo := ts.CreateRepository("https://github.com/test/language-filter-repo")
	commit := ts.CreateCommit(repo, "langfilter123456", "add go handler")

	// Create a Go snippet
	snippet := ts.CreateSnippetEnrichmentForCommit(commit.SHA(), "func HandleRequest(w http.ResponseWriter)", "go")
	ts.SeedBM25(fmt.Sprintf("%d", snippet.ID()), "func HandleRequest http ResponseWriter")

	// Search with language=py — should find nothing since the snippet is Go
	lang := "py"
	body := dto.KeywordSearchRequest{
		Data: dto.KeywordSearchData{
			Type: "keyword_search",
			Attributes: dto.KeywordSearchAttributes{
				Keywords: "HandleRequest",
				Language: &lang,
			},
		},
	}

	resp := ts.POST("/api/v1/search/keyword", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("expected 0 results with language=py, got %d", len(result.Data))
	}
}

func intPtr(i int) *int {
	return &i
}
