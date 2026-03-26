package v1

import (
	"net/http"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

// StepsRouter handles step API endpoints.
type StepsRouter struct {
	client *kodit.Client
	logger zerolog.Logger
}

// NewStepsRouter creates a new StepsRouter.
func NewStepsRouter(client *kodit.Client) *StepsRouter {
	return &StepsRouter{
		client: client,
		logger: client.Logger(),
	}
}

// Routes returns the chi router for step endpoints.
func (r *StepsRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/", r.List)
	router.Get("/{id}", r.Get)

	return router
}

// List handles GET /api/v1/steps.
//
//	@Summary		List steps
//	@Description	List all steps with pagination
//	@Tags			steps
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int	false	"Page number (default: 1)"
//	@Param			page_size	query		int	false	"Results per page (default: 20, max: 100)"
//	@Success		200			{object}	dto.StepListResponse
//	@Failure		500			{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/steps [get]
func (r *StepsRouter) List(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	pagination, err := ParsePagination(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	steps, err := r.client.Pipelines.FindSteps(ctx, pagination.Options()...)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	total, err := r.client.Pipelines.CountSteps(ctx)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	data := make([]dto.StepData, len(steps))
	for i, s := range steps {
		detail, err := r.client.Pipelines.DetailStep(ctx, s.ID())
		if err != nil {
			middleware.WriteError(w, req, err, r.logger)
			return
		}
		data[i] = stepToDTO(s, detail.Dependencies(), nil)
	}

	middleware.WriteJSON(w, http.StatusOK, dto.StepListResponse{
		Data:  data,
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
}

// Get handles GET /api/v1/steps/{id}.
//
//	@Summary		Get step
//	@Description	Get a step by ID
//	@Tags			steps
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Step ID"
//	@Success		200	{object}	dto.StepResponse
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/steps/{id} [get]
func (r *StepsRouter) Get(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	detail, err := r.client.Pipelines.DetailStep(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.StepResponse{
		Data: stepToDTO(detail.Step(), detail.Dependencies(), nil),
	})
}
