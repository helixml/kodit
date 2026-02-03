package v1

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/internal/api"
	"github.com/helixml/kodit/internal/api/middleware"
	"github.com/helixml/kodit/internal/api/v1/dto"
	"github.com/helixml/kodit/internal/enrichment"
)

// EnrichmentsRouter handles enrichment API endpoints.
type EnrichmentsRouter struct {
	enrichmentRepo enrichment.EnrichmentRepository
	logger         *slog.Logger
}

// NewEnrichmentsRouter creates a new EnrichmentsRouter.
func NewEnrichmentsRouter(enrichmentRepo enrichment.EnrichmentRepository, logger *slog.Logger) *EnrichmentsRouter {
	if logger == nil {
		logger = slog.Default()
	}
	return &EnrichmentsRouter{
		enrichmentRepo: enrichmentRepo,
		logger:         logger,
	}
}

// Routes returns the chi router for enrichment endpoints.
func (r *EnrichmentsRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/", r.List)
	router.Get("/{id}", r.Get)

	return router
}

// List handles GET /api/v1/enrichments.
// Supports query parameters: enrichment_type, enrichment_subtype, page, page_size
func (r *EnrichmentsRouter) List(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Parse filter parameters (matching Python API param names)
	typeParam := req.URL.Query().Get("enrichment_type")
	subtypeParam := req.URL.Query().Get("enrichment_subtype")

	// Parse pagination parameters
	pagination := api.ParsePagination(req)

	// If no filters provided, require at least one filter
	if typeParam == "" && subtypeParam == "" {
		middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIListResponse{
			Data: []dto.EnrichmentData{},
		})
		return
	}

	var enrichments []enrichment.Enrichment
	var err error

	if typeParam != "" && subtypeParam != "" {
		enrichments, err = r.enrichmentRepo.FindByTypeAndSubtype(
			ctx,
			enrichment.Type(typeParam),
			enrichment.Subtype(subtypeParam),
		)
	} else if typeParam != "" {
		enrichments, err = r.enrichmentRepo.FindByType(ctx, enrichment.Type(typeParam))
	}

	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Apply pagination manually
	offset := pagination.Offset()
	limit := pagination.Limit()

	if offset >= len(enrichments) {
		enrichments = []enrichment.Enrichment{}
	} else {
		end := offset + limit
		if end > len(enrichments) {
			end = len(enrichments)
		}
		enrichments = enrichments[offset:end]
	}

	response := dto.EnrichmentJSONAPIListResponse{
		Data: enrichmentsToJSONAPIDTO(enrichments),
	}

	middleware.WriteJSON(w, http.StatusOK, response)
}

// Get handles GET /api/v1/enrichments/{id}.
func (r *EnrichmentsRouter) Get(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	e, err := r.enrichmentRepo.Get(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIResponse{
		Data: enrichmentToJSONAPIDTO(e),
	})
}

func enrichmentsToJSONAPIDTO(enrichments []enrichment.Enrichment) []dto.EnrichmentData {
	result := make([]dto.EnrichmentData, len(enrichments))
	for i, e := range enrichments {
		result[i] = enrichmentToJSONAPIDTO(e)
	}
	return result
}

func enrichmentToJSONAPIDTO(e enrichment.Enrichment) dto.EnrichmentData {
	return dto.EnrichmentData{
		Type: "enrichment",
		ID:   fmt.Sprintf("%d", e.ID()),
		Attributes: dto.EnrichmentAttributes{
			Type:      string(e.Type()),
			Subtype:   string(e.Subtype()),
			Content:   e.Content(),
			CreatedAt: e.CreatedAt(),
			UpdatedAt: e.UpdatedAt(),
		},
	}
}
