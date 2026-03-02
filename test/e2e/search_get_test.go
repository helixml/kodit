package e2e_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

func TestSemanticSearch_GET_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/semantic?query=" + url.QueryEscape("authentication handler"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestSemanticSearch_GET_MissingQuery(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/semantic")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestKeywordSearch_GET_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/keyword?keywords=nonexistent_xyzzy_keyword")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestKeywordSearch_GET_MissingKeywords(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/keyword")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSemanticSearch_GET_WithRepositoryID(t *testing.T) {
	ts := NewTestServer(t)
	repo := ts.CreateRepository("https://github.com/test/semantic-repo.git")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/semantic?query=%s&repository_id=%d",
		url.QueryEscape("authentication handler"), repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	// No vector data seeded, expect empty results
	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestSemanticSearch_GET_InvalidRepositoryID(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/semantic?query=test&repository_id=notanumber")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestKeywordSearch_GET_WithRepositoryID(t *testing.T) {
	ts := NewTestServer(t)
	repo := ts.CreateRepository("https://github.com/test/keyword-repo.git")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/keyword?keywords=nonexistent_xyzzy&repository_id=%d", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestKeywordSearch_GET_InvalidRepositoryID(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/keyword?keywords=test&repository_id=notanumber")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSemanticSearch_GET_RepositoryIDZero_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository with enrichments so there's data in the system.
	repo := ts.CreateRepository("https://github.com/test/semantic-repoid-zero.git")
	commit := ts.CreateCommit(repo, "abc000def000", "add module")
	snippet := ts.CreateSnippetEnrichmentForCommit(commit.SHA(), "func Main() {}", "go")
	_ = snippet

	// repository_id=0 is not a valid repository. It should not match
	// orphaned enrichments that happen to have no repository association.
	resp := ts.GET("/api/v1/search/semantic?query=Main&repository_id=0")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSemanticSearch_GET_LimitZero_Returns400(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/semantic?query=test&limit=0")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestKeywordSearch_GET_LimitZero_Returns400(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/keyword?keywords=test&limit=0")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSemanticSearch_GET_WhitespaceOnlyQuery_Returns400(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/semantic?query=" + url.QueryEscape("   "))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSemanticSearch_GET_NonExistentRepo_Returns404(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/semantic?query=test&repository_id=99999")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusNotFound, body)
	}
}

func TestKeywordSearch_GET_NonExistentRepo_Returns404(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/keyword?keywords=test&repository_id=99999")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusNotFound, body)
	}
}

func TestSemanticSearch_GET_LanguageFilter_ReturnsOnlyMatchingLanguage(t *testing.T) {
	ts := NewTestServer(t)

	// No vector data seeded, so the search returns empty — but the endpoint
	// must accept the language parameter without error.
	resp := ts.GET("/api/v1/search/semantic?query=handler&limit=5&language=py")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	// Verify no non-Python results are returned (with no vector data, 0 is correct).
	for _, d := range result.Data {
		if !strings.EqualFold(d.Attributes.Content.Language, "py") {
			t.Errorf("expected language=py, got %q", d.Attributes.Content.Language)
		}
	}
}
