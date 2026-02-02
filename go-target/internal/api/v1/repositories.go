// Package v1 provides the v1 API routes.
package v1

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/internal/api/middleware"
	"github.com/helixml/kodit/internal/api/v1/dto"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/repository"
)

// RepositoriesRouter handles repository API endpoints.
type RepositoriesRouter struct {
	queryService *repository.QueryService
	syncService  *repository.SyncService
	logger       *slog.Logger
}

// NewRepositoriesRouter creates a new RepositoriesRouter.
func NewRepositoriesRouter(
	queryService *repository.QueryService,
	syncService *repository.SyncService,
	logger *slog.Logger,
) *RepositoriesRouter {
	if logger == nil {
		logger = slog.Default()
	}
	return &RepositoriesRouter{
		queryService: queryService,
		syncService:  syncService,
		logger:       logger,
	}
}

// Routes returns the chi router for repository endpoints.
func (r *RepositoriesRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/", r.List)
	router.Post("/", r.Add)
	router.Get("/{id}", r.Get)
	router.Delete("/{id}", r.Delete)
	router.Post("/{id}/sync", r.Sync)

	return router
}

// List handles GET /api/v1/repositories.
func (r *RepositoriesRouter) List(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	sources, err := r.queryService.All(ctx)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	response := dto.RepositoryListResponse{
		Data:       sourcesToDTO(sources),
		TotalCount: len(sources),
	}

	middleware.WriteJSON(w, http.StatusOK, response)
}

// Get handles GET /api/v1/repositories/{id}.
func (r *RepositoriesRouter) Get(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	source, err := r.queryService.ByID(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, sourceToDTO(source))
}

// Add handles POST /api/v1/repositories.
func (r *RepositoriesRouter) Add(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var body dto.RepositoryRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if body.RemoteURL == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "remote_url is required",
		})
		return
	}

	var source repository.Source
	var err error

	// If tracking config provided, use AddRepositoryWithTracking
	if body.Branch != "" || body.Tag != "" || body.Commit != "" {
		tc := git.NewTrackingConfig(body.Branch, body.Tag, body.Commit)
		source, err = r.syncService.AddRepositoryWithTracking(ctx, body.RemoteURL, tc)
	} else {
		source, err = r.syncService.AddRepository(ctx, body.RemoteURL)
	}

	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, sourceToDTO(source))
}

// Delete handles DELETE /api/v1/repositories/{id}.
func (r *RepositoriesRouter) Delete(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if err := r.syncService.RequestDelete(ctx, id); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Sync handles POST /api/v1/repositories/{id}/sync.
func (r *RepositoriesRouter) Sync(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if err := r.syncService.RequestSync(ctx, id); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func sourcesToDTO(sources []repository.Source) []dto.RepositoryResponse {
	result := make([]dto.RepositoryResponse, len(sources))
	for i, source := range sources {
		result[i] = sourceToDTO(source)
	}
	return result
}

func sourceToDTO(source repository.Source) dto.RepositoryResponse {
	repo := source.Repo()
	trackingType := ""
	trackingValue := ""

	if tc := repo.TrackingConfig(); tc.Branch() != "" {
		trackingType = "branch"
		trackingValue = tc.Branch()
	} else if tc.Tag() != "" {
		trackingType = "tag"
		trackingValue = tc.Tag()
	} else if tc.Commit() != "" {
		trackingType = "commit"
		trackingValue = tc.Commit()
	}

	return dto.RepositoryResponse{
		ID:            repo.ID(),
		RemoteURL:     repo.RemoteURL(),
		WorkingCopy:   repo.WorkingCopy().Path(),
		TrackingType:  trackingType,
		TrackingValue: trackingValue,
		CreatedAt:     repo.CreatedAt(),
		UpdatedAt:     repo.UpdatedAt(),
	}
}
