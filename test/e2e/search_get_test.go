package e2e_test

import (
	"fmt"
	"net/http"
	"net/url"
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
