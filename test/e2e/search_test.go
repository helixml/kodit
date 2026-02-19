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

func intPtr(i int) *int {
	return &i
}
