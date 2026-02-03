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

	resp := ts.GET("/api/v1/enrichments?enrichment_type=development")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.EnrichmentJSONAPIListResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(result.Data))
	}
	for _, d := range result.Data {
		if d.Type != "enrichment" {
			t.Errorf("type = %q, want %q", d.Type, "enrichment")
		}
	}
}

func TestEnrichments_List_WithSubtypeFilter(t *testing.T) {
	ts := NewTestServer(t)

	// Create some enrichments
	ts.CreateEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, "snippet content")
	ts.CreateEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeExample, "example content")

	resp := ts.GET("/api/v1/enrichments?enrichment_type=development&enrichment_subtype=snippet")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.EnrichmentJSONAPIListResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 1 {
		t.Errorf("len(data) = %d, want 1", len(result.Data))
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

	var result dto.EnrichmentJSONAPIListResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0 (no filter specified)", len(result.Data))
	}
}

func TestEnrichments_List_WithPagination(t *testing.T) {
	ts := NewTestServer(t)

	// Create 5 enrichments
	for i := 0; i < 5; i++ {
		ts.CreateEnrichment(enrichment.TypeDevelopment, enrichment.SubtypeSnippet, fmt.Sprintf("content %d", i))
	}

	// Request page 2 with page_size 2
	resp := ts.GET("/api/v1/enrichments?enrichment_type=development&page=2&page_size=2")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.EnrichmentJSONAPIListResponse
	ts.DecodeJSON(resp, &result)

	// Page 2 with page_size 2 should return items 2-3 (0-indexed: 2, 3)
	if len(result.Data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(result.Data))
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

	var result dto.EnrichmentJSONAPIResponse
	ts.DecodeJSON(resp, &result)

	if result.Data.Type != "enrichment" {
		t.Errorf("type = %q, want %q", result.Data.Type, "enrichment")
	}
	if result.Data.ID != fmt.Sprintf("%d", e.ID()) {
		t.Errorf("ID = %q, want %q", result.Data.ID, fmt.Sprintf("%d", e.ID()))
	}
	if result.Data.Attributes.Type != "development" {
		t.Errorf("attributes.type = %q, want %q", result.Data.Attributes.Type, "development")
	}
	if result.Data.Attributes.Subtype != "snippet" {
		t.Errorf("attributes.subtype = %q, want %q", result.Data.Attributes.Subtype, "snippet")
	}
	if result.Data.Attributes.Content != "test content for get" {
		t.Errorf("attributes.content = %q, want %q", result.Data.Attributes.Content, "test content for get")
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
