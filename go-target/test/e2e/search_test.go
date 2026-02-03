package e2e_test

import (
	"net/http"
	"testing"

	"github.com/helixml/kodit/internal/api/v1/dto"
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

func intPtr(i int) *int {
	return &i
}
