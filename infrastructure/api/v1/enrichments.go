package v1

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
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
//	@Failure		500					{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/enrichments [get]
func (r *EnrichmentsRouter) List(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Parse filter parameters (matching Python API param names)
	typeParam := req.URL.Query().Get("enrichment_type")
	subtypeParam := req.URL.Query().Get("enrichment_subtype")

	// Build list params
	params := &service.EnrichmentListParams{}
	if typeParam != "" {
		t := enrichment.Type(typeParam)
		params.Type = &t
	}
	if subtypeParam != "" {
		s := enrichment.Subtype(subtypeParam)
		params.Subtype = &s
	}

	pagination := ParsePagination(req)
	params.Limit = pagination.Limit()
	params.Offset = pagination.Offset()

	enrichments, err := r.client.Enrichments.List(ctx, params)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	total, err := r.client.Enrichments.Count(ctx, params)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	ids := make([]int64, len(enrichments))
	for i, e := range enrichments {
		ids[i] = e.ID()
	}
	lineRanges, err := r.client.Enrichments.LineRanges(ctx, ids)
	if err != nil {
		r.logger.Warn("failed to fetch line ranges", "error", err)
		lineRanges = map[string]chunk.LineRange{}
	}

	response := dto.EnrichmentJSONAPIListResponse{
		Data:  enrichmentsToJSONAPIDTO(enrichments, lineRanges),
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
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
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
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

	e, err := r.client.Enrichments.Get(ctx, repository.WithID(id))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	lineRanges, err := r.client.Enrichments.LineRanges(ctx, []int64{id})
	if err != nil {
		r.logger.Warn("failed to fetch line ranges", "error", err)
		lineRanges = map[string]chunk.LineRange{}
	}

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIResponse{
		Data: enrichmentToJSONAPIDTO(e, lineRanges),
	})
}

func enrichmentsToJSONAPIDTO(enrichments []enrichment.Enrichment, lineRanges map[string]chunk.LineRange) []dto.EnrichmentData {
	result := make([]dto.EnrichmentData, len(enrichments))
	for i, e := range enrichments {
		result[i] = enrichmentToJSONAPIDTO(e, lineRanges)
	}
	return result
}

func enrichmentToJSONAPIDTO(e enrichment.Enrichment, lineRanges map[string]chunk.LineRange) dto.EnrichmentData {
	attrs := dto.EnrichmentAttributes{
		Type:      string(e.Type()),
		Subtype:   string(e.Subtype()),
		Content:   e.Content(),
		CreatedAt: e.CreatedAt(),
		UpdatedAt: e.UpdatedAt(),
	}

	idStr := fmt.Sprintf("%d", e.ID())
	if lr, ok := lineRanges[idStr]; ok {
		startLine := lr.StartLine()
		endLine := lr.EndLine()
		attrs.StartLine = &startLine
		attrs.EndLine = &endLine
	}

	return dto.EnrichmentData{
		Type:       "enrichment",
		ID:         idStr,
		Attributes: attrs,
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
//	@Failure		404		{object}	middleware.JSONAPIErrorResponse
//	@Failure		500		{object}	middleware.JSONAPIErrorResponse
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

	existing, err := r.client.Enrichments.Get(ctx, repository.WithID(id))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	saved, err := r.client.Enrichments.Save(ctx, existing.WithContent(body.Data.Attributes.Content))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	lineRanges, err := r.client.Enrichments.LineRanges(ctx, []int64{id})
	if err != nil {
		r.logger.Warn("failed to fetch line ranges", "error", err)
		lineRanges = map[string]chunk.LineRange{}
	}

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIResponse{
		Data: enrichmentToJSONAPIDTO(saved, lineRanges),
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
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
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

	if err := r.client.Enrichments.DeleteBy(ctx, repository.WithID(id)); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
