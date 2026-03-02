package e2e_test

import (
	"fmt"
	"net/http"
	"strings"
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

func TestSearch_POST_EmptyBody_Returns400(t *testing.T) {
	ts := NewTestServer(t)

	// Send an empty body with Content-Type: application/json.
	resp := ts.POSTRaw("/api/v1/search", "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSearch_POST_LinksUseBlobAPI(t *testing.T) {
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

	// Decode into a raw structure to check for absence of derives_from
	// and presence of blob API links.
	var raw struct {
		Data []struct {
			Attributes map[string]any `json:"attributes"`
			Links      *struct {
				Repository string `json:"repository"`
				Commit     string `json:"commit"`
				File       string `json:"file"`
			} `json:"links"`
		} `json:"data"`
	}
	ts.DecodeJSON(resp, &raw)

	if len(raw.Data) == 0 {
		t.Fatal("expected at least one search result")
	}

	for _, d := range raw.Data {
		// derives_from must NOT be present
		if _, ok := d.Attributes["derives_from"]; ok {
			t.Error("response still contains derives_from field")
		}

		// links.file must use the blob API format
		if d.Links != nil && d.Links.File != "" {
			expectedPrefix := fmt.Sprintf("/api/v1/repositories/%d/blob/", repo.ID())
			if !strings.HasPrefix(d.Links.File, expectedPrefix) {
				t.Errorf("links.file = %s, want prefix %s", d.Links.File, expectedPrefix)
			}
			expectedFile := fmt.Sprintf("/api/v1/repositories/%d/blob/%s/%s", repo.ID(), commit.SHA(), file.Path())
			if d.Links.File != expectedFile {
				t.Errorf("links.file = %s, want %s", d.Links.File, expectedFile)
			}
		}
	}
}

func TestKeywordSearch_GET_WithSeededData(t *testing.T) {
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

	resp := ts.GET("/api/v1/search/keyword?keywords=SearchKeywords")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	// Check blob API links
	var raw struct {
		Data []struct {
			Links *struct {
				File string `json:"file"`
			} `json:"links"`
		} `json:"data"`
	}
	ts.DecodeJSON(resp, &raw)

	if len(raw.Data) == 0 {
		t.Fatal("expected at least one keyword search result")
	}

	found := false
	expectedFile := fmt.Sprintf("/api/v1/repositories/%d/blob/%s/%s", repo.ID(), commit.SHA(), file.Path())
	for _, d := range raw.Data {
		if d.Links != nil && d.Links.File == expectedFile {
			found = true
		}
	}

	if !found {
		t.Errorf("links.file did not contain expected blob link %s", expectedFile)
	}
}

func TestKeywordSearch_GET_WithLanguageFilter(t *testing.T) {
	ts := NewTestServer(t)

	repo := ts.CreateRepository("https://github.com/test/language-filter-repo")
	commit := ts.CreateCommit(repo, "langfilter123456", "add go handler")

	// Create a Go snippet
	snippet := ts.CreateSnippetEnrichmentForCommit(commit.SHA(), "func HandleRequest(w http.ResponseWriter)", "go")
	ts.SeedBM25(fmt.Sprintf("%d", snippet.ID()), "func HandleRequest http ResponseWriter")

	// Search with language=py — should find nothing since the snippet is Go
	resp := ts.GET("/api/v1/search/keyword?keywords=HandleRequest&language=py")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
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
