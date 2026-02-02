package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/helixml/kodit/internal/api/v1/dto"
	"github.com/helixml/kodit/internal/enrichment"
)

func TestEnrichments_List_WithTypeFilter(t *testing.T) {
	ts := NewTestServer(t)

	// Create some enrichments
	ts.CreateEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, "test content 1")
	ts.CreateEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, "test content 2")
	ts.CreateEnrichment(enrichment.TypeArchitecture, enrichment.SubtypePhysical, "architecture content")

	resp := ts.GET("/api/v1/enrichments?type=development")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.EnrichmentListResponse
	ts.DecodeJSON(resp, &result)

	if result.TotalCount != 2 {
		t.Errorf("total_count = %d, want 2", result.TotalCount)
	}
	if len(result.Data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(result.Data))
	}
}

func TestEnrichments_List_WithSubtypeFilter(t *testing.T) {
	ts := NewTestServer(t)

	// Create some enrichments
	ts.CreateEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, "snippet content")
	ts.CreateEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeExample, "example content")

	resp := ts.GET("/api/v1/enrichments?type=development&subtype=snippet")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.EnrichmentListResponse
	ts.DecodeJSON(resp, &result)

	if result.TotalCount != 1 {
		t.Errorf("total_count = %d, want 1", result.TotalCount)
	}
}

func TestEnrichments_List_NoFilter(t *testing.T) {
	ts := NewTestServer(t)

	// Create an enrichment
	ts.CreateEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, "test content")

	// Without type filter, should return empty list
	resp := ts.GET("/api/v1/enrichments")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.EnrichmentListResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0 (no filter specified)", len(result.Data))
	}
}

func TestEnrichments_Get(t *testing.T) {
	ts := NewTestServer(t)

	// Create an enrichment
	e := ts.CreateEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, "test content for get")

	resp := ts.GET(fmt.Sprintf("/api/v1/enrichments/%d", e.ID()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.EnrichmentResponse
	ts.DecodeJSON(resp, &result)

	if result.ID != e.ID() {
		t.Errorf("ID = %d, want %d", result.ID, e.ID())
	}
	if result.Type != "development" {
		t.Errorf("type = %q, want %q", result.Type, "development")
	}
	if result.Subtype != "snippet" {
		t.Errorf("subtype = %q, want %q", result.Subtype, "snippet")
	}
	if result.Content != "test content for get" {
		t.Errorf("content = %q, want %q", result.Content, "test content for get")
	}
}

func TestEnrichments_Get_NotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/enrichments/99999")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
