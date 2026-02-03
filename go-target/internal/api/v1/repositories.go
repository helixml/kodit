// Package v1 provides the v1 API routes.
package v1

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/internal/api/middleware"
	"github.com/helixml/kodit/internal/api/v1/dto"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/repository"
	"github.com/helixml/kodit/internal/tracking"
)

// RepositoriesRouter handles repository API endpoints.
type RepositoriesRouter struct {
	queryService        *repository.QueryService
	syncService         *repository.SyncService
	trackingQueryService *tracking.QueryService
	logger              *slog.Logger
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

// WithTrackingQueryService sets the tracking query service for status endpoints.
func (r *RepositoriesRouter) WithTrackingQueryService(svc *tracking.QueryService) *RepositoriesRouter {
	r.trackingQueryService = svc
	return r
}

// Routes returns the chi router for repository endpoints.
func (r *RepositoriesRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/", r.List)
	router.Post("/", r.Add)
	router.Get("/{id}", r.Get)
	router.Delete("/{id}", r.Delete)
	router.Post("/{id}/sync", r.Sync)
	router.Get("/{id}/status", r.GetStatus)
	router.Get("/{id}/status/summary", r.GetStatusSummary)
	router.Get("/{id}/commits", r.ListCommits)
	router.Get("/{id}/commits/{commit_sha}", r.GetCommit)
	router.Get("/{id}/commits/{commit_sha}/files", r.ListCommitFiles)

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

// GetStatus handles GET /api/v1/repositories/{id}/status.
func (r *RepositoriesRouter) GetStatus(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Check repository exists
	_, err = r.queryService.ByID(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// If tracking service not configured, return empty list
	if r.trackingQueryService == nil {
		middleware.WriteJSON(w, http.StatusOK, dto.TaskStatusListResponse{Data: []dto.TaskStatusData{}})
		return
	}

	statuses, err := r.trackingQueryService.StatusesForRepository(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	taskStatuses := make([]dto.TaskStatusData, 0, len(statuses))
	for _, status := range statuses {
		createdAt := status.CreatedAt()
		updatedAt := status.UpdatedAt()
		taskStatuses = append(taskStatuses, dto.TaskStatusData{
			Type: "task_status",
			ID:   status.ID(),
			Attributes: dto.TaskStatusAttributes{
				Step:      string(status.Operation()),
				State:     string(status.State()),
				Progress:  status.CompletionPercent(),
				Total:     status.Total(),
				Current:   status.Current(),
				CreatedAt: &createdAt,
				UpdatedAt: &updatedAt,
				Error:     status.Error(),
				Message:   status.Message(),
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.TaskStatusListResponse{Data: taskStatuses})
}

// GetStatusSummary handles GET /api/v1/repositories/{id}/status/summary.
func (r *RepositoriesRouter) GetStatusSummary(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Check repository exists
	_, err = r.queryService.ByID(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// If tracking service not configured, return default pending status
	if r.trackingQueryService == nil {
		middleware.WriteJSON(w, http.StatusOK, dto.RepositoryStatusSummaryResponse{
			Data: dto.RepositoryStatusSummaryData{
				Type: "repository_status_summary",
				ID:   fmt.Sprintf("%d", id),
				Attributes: dto.RepositoryStatusSummaryAttributes{
					Status:    "pending",
					Message:   "",
					UpdatedAt: time.Now().UTC(),
				},
			},
		})
		return
	}

	summary, err := r.trackingQueryService.SummaryForRepository(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.RepositoryStatusSummaryResponse{
		Data: dto.RepositoryStatusSummaryData{
			Type: "repository_status_summary",
			ID:   fmt.Sprintf("%d", id),
			Attributes: dto.RepositoryStatusSummaryAttributes{
				Status:    string(summary.Status()),
				Message:   summary.Message(),
				UpdatedAt: summary.UpdatedAt(),
			},
		},
	})
}

// ListCommits handles GET /api/v1/repositories/{id}/commits.
func (r *RepositoriesRouter) ListCommits(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Check repository exists
	_, err = r.queryService.ByID(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commits, err := r.queryService.CommitsForRepository(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	data := make([]dto.CommitData, 0, len(commits))
	for _, commit := range commits {
		data = append(data, dto.CommitData{
			Type: "commit",
			ID:   commit.SHA(),
			Attributes: dto.CommitAttributes{
				CommitSHA:       commit.SHA(),
				Date:            commit.CommittedAt(),
				Message:         commit.Message(),
				ParentCommitSHA: commit.ParentCommitSHA(),
				Author:          commit.Author().Name(),
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.CommitJSONAPIListResponse{Data: data})
}

// GetCommit handles GET /api/v1/repositories/{id}/commits/{commit_sha}.
func (r *RepositoriesRouter) GetCommit(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check repository exists
	_, err = r.queryService.ByID(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commit, err := r.queryService.CommitBySHA(ctx, id, commitSHA)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.CommitJSONAPIResponse{
		Data: dto.CommitData{
			Type: "commit",
			ID:   commit.SHA(),
			Attributes: dto.CommitAttributes{
				CommitSHA:       commit.SHA(),
				Date:            commit.CommittedAt(),
				Message:         commit.Message(),
				ParentCommitSHA: commit.ParentCommitSHA(),
				Author:          commit.Author().Name(),
			},
		},
	})
}

// ListCommitFiles handles GET /api/v1/repositories/{id}/commits/{commit_sha}/files.
func (r *RepositoriesRouter) ListCommitFiles(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check repository exists
	_, err = r.queryService.ByID(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Check commit exists and belongs to this repo
	_, err = r.queryService.CommitBySHA(ctx, id, commitSHA)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	files, err := r.queryService.FilesForCommit(ctx, commitSHA)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	data := make([]dto.FileData, 0, len(files))
	for _, file := range files {
		data = append(data, dto.FileData{
			Type: "file",
			ID:   file.BlobSHA(),
			Attributes: dto.FileAttributes{
				BlobSHA:   file.BlobSHA(),
				Path:      file.Path(),
				MimeType:  file.MimeType(),
				Size:      file.Size(),
				Extension: file.Extension(),
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.FileJSONAPIListResponse{Data: data})
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
