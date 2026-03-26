package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

// PipelinesRouter handles pipeline API endpoints.
type PipelinesRouter struct {
	client *kodit.Client
	logger zerolog.Logger
}

// NewPipelinesRouter creates a new PipelinesRouter.
func NewPipelinesRouter(client *kodit.Client) *PipelinesRouter {
	return &PipelinesRouter{
		client: client,
		logger: client.Logger(),
	}
}

// Routes returns the chi router for pipeline endpoints.
func (r *PipelinesRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/", r.List)
	router.Post("/", r.Create)
	router.Get("/{id}", r.Get)
	router.Put("/{id}", r.Update)
	router.Delete("/{id}", r.Delete)

	return router
}

// List handles GET /api/v1/pipelines.
//
//	@Summary		List pipelines
//	@Description	List all pipelines with pagination
//	@Tags			pipelines
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int	false	"Page number (default: 1)"
//	@Param			page_size	query		int	false	"Results per page (default: 20, max: 100)"
//	@Success		200			{object}	dto.PipelineListResponse
//	@Failure		500			{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/pipelines [get]
func (r *PipelinesRouter) List(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	pagination, err := ParsePagination(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	pipelines, err := r.client.Pipelines.Find(ctx, pagination.Options()...)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	total, err := r.client.Pipelines.Count(ctx)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	data := make([]dto.PipelineData, len(pipelines))
	for i, p := range pipelines {
		data[i] = pipelineToDTO(p)
	}

	middleware.WriteJSON(w, http.StatusOK, dto.PipelineListResponse{
		Data:  data,
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
}

// Create handles POST /api/v1/pipelines.
//
//	@Summary		Create pipeline
//	@Description	Create a new pipeline with steps
//	@Tags			pipelines
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.PipelineCreateRequest	true	"Pipeline to create"
//	@Success		201		{object}	dto.PipelineDetailResponse
//	@Failure		400		{object}	middleware.JSONAPIErrorResponse
//	@Failure		500		{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/pipelines [post]
func (r *PipelinesRouter) Create(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var body dto.PipelineCreateRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	steps := make([]service.StepParams, len(body.Data.Attributes.Steps))
	for i, s := range body.Data.Attributes.Steps {
		steps[i] = service.StepParams{
			Name:      s.Name,
			Kind:      s.Kind,
			DependsOn: s.DependsOn,
			JoinType:  s.JoinType,
		}
	}

	detail, err := r.client.Pipelines.Create(ctx, &service.CreatePipelineParams{
		Name:  body.Data.Attributes.Name,
		Steps: steps,
	})
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, pipelineDetailToDTO(detail))
}

// Get handles GET /api/v1/pipelines/{id}.
//
//	@Summary		Get pipeline
//	@Description	Get a pipeline with its steps and dependencies
//	@Tags			pipelines
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Pipeline ID"
//	@Success		200	{object}	dto.PipelineDetailResponse
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/pipelines/{id} [get]
func (r *PipelinesRouter) Get(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	detail, err := r.client.Pipelines.Detail(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, pipelineDetailToDTO(detail))
}

// Update handles PUT /api/v1/pipelines/{id}.
//
//	@Summary		Update pipeline
//	@Description	Replace pipeline name and steps
//	@Tags			pipelines
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int							true	"Pipeline ID"
//	@Param			body	body		dto.PipelineUpdateRequest	true	"Pipeline update"
//	@Success		200		{object}	dto.PipelineDetailResponse
//	@Failure		400		{object}	middleware.JSONAPIErrorResponse
//	@Failure		404		{object}	middleware.JSONAPIErrorResponse
//	@Failure		500		{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/pipelines/{id} [put]
func (r *PipelinesRouter) Update(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	var body dto.PipelineUpdateRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	steps := make([]service.StepParams, len(body.Data.Attributes.Steps))
	for i, s := range body.Data.Attributes.Steps {
		steps[i] = service.StepParams{
			Name:      s.Name,
			Kind:      s.Kind,
			DependsOn: s.DependsOn,
			JoinType:  s.JoinType,
		}
	}

	detail, err := r.client.Pipelines.Update(ctx, id, &service.UpdatePipelineParams{
		Name:  body.Data.Attributes.Name,
		Steps: steps,
	})
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, pipelineDetailToDTO(detail))
}

// Delete handles DELETE /api/v1/pipelines/{id}.
//
//	@Summary		Delete pipeline
//	@Description	Delete a pipeline and all its steps
//	@Tags			pipelines
//	@Accept			json
//	@Produce		json
//	@Param			id	path	int	true	"Pipeline ID"
//	@Success		204
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/pipelines/{id} [delete]
func (r *PipelinesRouter) Delete(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if err := r.client.Pipelines.Delete(ctx, id); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func pipelineToDTO(p repository.Pipeline) dto.PipelineData {
	return dto.PipelineData{
		Type: "pipeline",
		ID:   p.ID(),
		Attributes: dto.PipelineAttributes{
			Name:      p.Name(),
			CreatedAt: p.CreatedAt(),
			UpdatedAt: p.UpdatedAt(),
		},
		Links: dto.PipelineLinks{
			Self: fmt.Sprintf("/api/v1/pipelines/%d", p.ID()),
		},
	}
}

func pipelineDetailToDTO(d service.PipelineDetail) dto.PipelineDetailResponse {
	steps := d.Steps()
	deps := d.Dependencies()
	assocs := d.Associations()

	included := make([]dto.StepData, len(steps))
	for i, s := range steps {
		included[i] = stepToDTO(s, deps, assocs)
	}

	return dto.PipelineDetailResponse{
		Data:     pipelineToDTO(d.Pipeline()),
		Included: included,
	}
}

func stepToDTO(s repository.Step, deps []repository.StepDependency, assocs []repository.PipelineStep) dto.StepData {
	var dependsOn []int64
	for _, dep := range deps {
		if dep.StepID() == s.ID() {
			dependsOn = append(dependsOn, dep.DependsOnID())
		}
	}
	if dependsOn == nil {
		dependsOn = []int64{}
	}

	joinType := "all"
	for _, a := range assocs {
		if a.StepID() == s.ID() {
			joinType = a.JoinType()
			break
		}
	}

	return dto.StepData{
		Type: "step",
		ID:   s.ID(),
		Attributes: dto.StepAttributes{
			Name:      s.Name(),
			Kind:      s.Kind(),
			DependsOn: dependsOn,
			JoinType:  joinType,
		},
		Links: dto.StepLinks{
			Self: fmt.Sprintf("/api/v1/steps/%d", s.ID()),
		},
	}
}
