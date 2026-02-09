package v1

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

// EnrichmentsRouter handles enrichment API endpoints.
type EnrichmentsRouter struct {
	client *kodit.Client
	logger *slog.Logger
}

// NewEnrichmentsRouter creates a new EnrichmentsRouter.
func NewEnrichmentsRouter(client *kodit.Client) *EnrichmentsRouter {
	return &EnrichmentsRouter{
		client: client,
		logger: client.Logger(),
	}
}

// Routes returns the chi router for enrichment endpoints.
func (r *EnrichmentsRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/", r.List)
	router.Get("/{id}", r.Get)
	router.Patch("/{id}", r.Update)
	router.Delete("/{id}", r.Delete)

	return router
}

// List handles GET /api/v1/enrichments.
// Supports query parameters: enrichment_type, enrichment_subtype, page, page_size
//
//	@Summary		List enrichments
//	@Description	List enrichments with optional filters
//	@Tags			enrichments
//	@Accept			json
//	@Produce		json
//	@Param			enrichment_type		query		string	false	"Filter by enrichment type"
//	@Param			enrichment_subtype	query		string	false	"Filter by enrichment subtype"
//	@Param			page				query		int		false	"Page number (default: 1)"
//	@Param			page_size			query		int		false	"Results per page (default: 20, max: 100)"
//	@Success		200					{object}	dto.EnrichmentJSONAPIListResponse
//	@Failure		500					{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/enrichments [get]
func (r *EnrichmentsRouter) List(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Parse filter parameters (matching Python API param names)
	typeParam := req.URL.Query().Get("enrichment_type")
	subtypeParam := req.URL.Query().Get("enrichment_subtype")

	// If no filters provided, require at least one filter
	if typeParam == "" && subtypeParam == "" {
		middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIListResponse{
			Data: []dto.EnrichmentData{},
		})
		return
	}

	// Build domain filter
	filter := enrichment.NewFilter()
	if typeParam != "" {
		filter = filter.WithType(enrichment.Type(typeParam))
	}
	if subtypeParam != "" {
		filter = filter.WithSubtype(enrichment.Subtype(subtypeParam))
	}

	enrichments, err := r.client.Enrichments().List(ctx, filter)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Apply pagination manually
	pagination := ParsePagination(req)
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
//
//	@Summary		Get enrichment
//	@Description	Get an enrichment by ID
//	@Tags			enrichments
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Enrichment ID"
//	@Success		200	{object}	dto.EnrichmentJSONAPIResponse
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/enrichments/{id} [get]
func (r *EnrichmentsRouter) Get(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	e, err := r.client.Enrichments().Get(ctx, id)
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

// Update handles PATCH /api/v1/enrichments/{id}.
//
//	@Summary		Update enrichment
//	@Description	Update an enrichment's content
//	@Tags			enrichments
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int							true	"Enrichment ID"
//	@Param			body	body		dto.EnrichmentUpdateRequest	true	"Update request"
//	@Success		200		{object}	dto.EnrichmentJSONAPIResponse
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/enrichments/{id} [patch]
func (r *EnrichmentsRouter) Update(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	var body dto.EnrichmentUpdateRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	saved, err := r.client.Enrichments().Update(ctx, id, body.Data.Attributes.Content)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIResponse{
		Data: enrichmentToJSONAPIDTO(saved),
	})
}

// Delete handles DELETE /api/v1/enrichments/{id}.
//
//	@Summary		Delete enrichment
//	@Description	Delete an enrichment by ID
//	@Tags			enrichments
//	@Accept			json
//	@Produce		json
//	@Param			id	path	int	true	"Enrichment ID"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/enrichments/{id} [delete]
func (r *EnrichmentsRouter) Delete(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if err := r.client.Enrichments().Delete(ctx, id); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
