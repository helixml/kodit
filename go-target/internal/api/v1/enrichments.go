package v1

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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
func (r *EnrichmentsRouter) List(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	typeParam := req.URL.Query().Get("type")
	subtypeParam := req.URL.Query().Get("subtype")

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
	} else {
		// No filters, return empty list (too large to return all)
		middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentListResponse{
			Data:       []dto.EnrichmentResponse{},
			TotalCount: 0,
		})
		return
	}

	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	response := dto.EnrichmentListResponse{
		Data:       enrichmentsToDTO(enrichments),
		TotalCount: len(enrichments),
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

	middleware.WriteJSON(w, http.StatusOK, enrichmentToDTO(e))
}

func enrichmentsToDTO(enrichments []enrichment.Enrichment) []dto.EnrichmentResponse {
	result := make([]dto.EnrichmentResponse, len(enrichments))
	for i, e := range enrichments {
		result[i] = enrichmentToDTO(e)
	}
	return result
}

func enrichmentToDTO(e enrichment.Enrichment) dto.EnrichmentResponse {
	return dto.EnrichmentResponse{
		ID:        e.ID(),
		Type:      string(e.Type()),
		Subtype:   string(e.Subtype()),
		Content:   e.Content(),
		Language:  e.Language(),
		CreatedAt: e.CreatedAt(),
		UpdatedAt: e.UpdatedAt(),
	}
}
