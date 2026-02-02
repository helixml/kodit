package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helixml/kodit/internal/api/v1/dto"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/enrichment"
)

// FakeEnrichmentRepository implements enrichment.EnrichmentRepository for testing.
type FakeEnrichmentRepository struct {
	enrichments []enrichment.Enrichment
	getErr      error
}

func (f *FakeEnrichmentRepository) Get(_ context.Context, id int64) (enrichment.Enrichment, error) {
	if f.getErr != nil {
		return enrichment.Enrichment{}, f.getErr
	}
	for _, e := range f.enrichments {
		if e.ID() == id {
			return e, nil
		}
	}
	return enrichment.Enrichment{}, domain.ErrNotFound
}

func (f *FakeEnrichmentRepository) Save(_ context.Context, e enrichment.Enrichment) (enrichment.Enrichment, error) {
	return e, nil
}

func (f *FakeEnrichmentRepository) Delete(_ context.Context, _ enrichment.Enrichment) error {
	return nil
}

func (f *FakeEnrichmentRepository) FindByType(_ context.Context, _ enrichment.Type) ([]enrichment.Enrichment, error) {
	return f.enrichments, nil
}

func (f *FakeEnrichmentRepository) FindByTypeAndSubtype(_ context.Context, typ enrichment.Type, subtype enrichment.Subtype) ([]enrichment.Enrichment, error) {
	var result []enrichment.Enrichment
	for _, e := range f.enrichments {
		if e.Type() == typ && e.Subtype() == subtype {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *FakeEnrichmentRepository) FindByEntityKey(_ context.Context, key enrichment.EntityTypeKey) ([]enrichment.Enrichment, error) {
	var result []enrichment.Enrichment
	for _, e := range f.enrichments {
		if e.EntityTypeKey() == key {
			result = append(result, e)
		}
	}
	return result, nil
}

func TestEnrichmentsRouter_List(t *testing.T) {
	fake := &FakeEnrichmentRepository{
		enrichments: []enrichment.Enrichment{
			enrichment.NewEnrichment(
				enrichment.TypeDevelopment,
				enrichment.SubtypeSnippet,
				enrichment.EntityTypeSnippet,
				"test content",
			).WithID(1),
		},
	}

	router := NewEnrichmentsRouter(fake, slog.Default())
	routes := router.Routes()

	// List endpoint requires type query parameter
	req := httptest.NewRequest(http.MethodGet, "/?type=development", nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusOK)
	}

	var response dto.EnrichmentListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 1 {
		t.Errorf("len(Data) = %v, want 1", len(response.Data))
	}
}

func TestEnrichmentsRouter_List_NoFilter(t *testing.T) {
	fake := &FakeEnrichmentRepository{
		enrichments: []enrichment.Enrichment{
			enrichment.NewEnrichment(
				enrichment.TypeDevelopment,
				enrichment.SubtypeSnippet,
				enrichment.EntityTypeSnippet,
				"test content",
			).WithID(1),
		},
	}

	router := NewEnrichmentsRouter(fake, slog.Default())
	routes := router.Routes()

	// Without type filter, should return empty list
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusOK)
	}

	var response dto.EnrichmentListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Data) != 0 {
		t.Errorf("len(Data) = %v, want 0 (no filter specified)", len(response.Data))
	}
}

func TestEnrichmentsRouter_Get(t *testing.T) {
	fake := &FakeEnrichmentRepository{
		enrichments: []enrichment.Enrichment{
			enrichment.NewEnrichment(
				enrichment.TypeDevelopment,
				enrichment.SubtypeSnippet,
				enrichment.EntityTypeSnippet,
				"test content",
			).WithID(1),
		},
	}

	router := NewEnrichmentsRouter(fake, slog.Default())
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, "/1", nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusOK)
	}

	var response dto.EnrichmentResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != 1 {
		t.Errorf("ID = %v, want 1", response.ID)
	}
}

func TestEnrichmentsRouter_Get_NotFound(t *testing.T) {
	fake := &FakeEnrichmentRepository{
		enrichments: []enrichment.Enrichment{},
	}

	router := NewEnrichmentsRouter(fake, slog.Default())
	routes := router.Routes()

	req := httptest.NewRequest(http.MethodGet, "/999", nil)
	w := httptest.NewRecorder()

	routes.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code = %v, want %v", w.Code, http.StatusNotFound)
	}
}
