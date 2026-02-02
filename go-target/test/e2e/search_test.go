package e2e_test

import (
	"net/http"
	"testing"

	"github.com/helixml/kodit/internal/api/v1/dto"
)

func TestSearch_GET_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search?q=test")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.SearchResponse
	ts.DecodeJSON(resp, &result)

	// With fake repositories, we get empty results
	if result.TotalCount != 0 {
		t.Errorf("total_count = %d, want 0", result.TotalCount)
	}
	if result.Query != "test" {
		t.Errorf("query = %q, want %q", result.Query, "test")
	}
}

func TestSearch_GET_MissingQuery(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSearch_POST_ReturnsEmpty(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.SearchRequest{
		Query: "authentication",
		TopK:  10,
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

	if result.TotalCount != 0 {
		t.Errorf("total_count = %d, want 0", result.TotalCount)
	}
	if result.Query != "authentication" {
		t.Errorf("query = %q, want %q", result.Query, "authentication")
	}
}

func TestSearch_POST_WithFilters(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.SearchRequest{
		Query:    "login",
		TopK:     5,
		Language: "python",
		Author:   "john",
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
	if result.TotalCount != 0 {
		t.Errorf("total_count = %d, want 0", result.TotalCount)
	}
}

func TestSearch_POST_WithTextAndCodeQuery(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.SearchRequest{
		Query:     "general query",
		TextQuery: "user authentication",
		CodeQuery: "def authenticate",
		TopK:      10,
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

	if result.Query != "general query" {
		t.Errorf("query = %q, want %q", result.Query, "general query")
	}
}
