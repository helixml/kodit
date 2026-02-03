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
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/repository"
	"github.com/helixml/kodit/internal/tracking"
)

// RepositoriesRouter handles repository API endpoints.
type RepositoriesRouter struct {
	queryService           *repository.QueryService
	syncService            *repository.SyncService
	trackingQueryService   *tracking.QueryService
	enrichmentQueryService *enrichment.QueryService
	enrichmentRepo         enrichment.EnrichmentRepository
	logger                 *slog.Logger
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

// WithEnrichmentServices sets the enrichment services for enrichment endpoints.
func (r *RepositoriesRouter) WithEnrichmentServices(
	querySvc *enrichment.QueryService,
	repo enrichment.EnrichmentRepository,
) *RepositoriesRouter {
	r.enrichmentQueryService = querySvc
	r.enrichmentRepo = repo
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
	router.Get("/{id}/commits/{commit_sha}/files/{blob_sha}", r.GetCommitFile)
	router.Get("/{id}/commits/{commit_sha}/enrichments", r.ListCommitEnrichments)
	router.Get("/{id}/commits/{commit_sha}/enrichments/{enrichment_id}", r.GetCommitEnrichment)
	router.Get("/{id}/commits/{commit_sha}/snippets", r.ListCommitSnippets)
	router.Post("/{id}/commits/{commit_sha}/rescan", r.RescanCommit)
	router.Get("/{id}/tags", r.ListTags)
	router.Get("/{id}/tags/{tag_id}", r.GetTag)
	router.Get("/{id}/tracking-config", r.GetTrackingConfig)
	router.Put("/{id}/tracking-config", r.UpdateTrackingConfig)

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

// GetCommitFile handles GET /api/v1/repositories/{id}/commits/{commit_sha}/files/{blob_sha}.
func (r *RepositoriesRouter) GetCommitFile(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")
	blobSHA := chi.URLParam(req, "blob_sha")

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

	file, err := r.queryService.FileByBlobSHA(ctx, commitSHA, blobSHA)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.FileJSONAPIResponse{
		Data: dto.FileData{
			Type: "file",
			ID:   file.BlobSHA(),
			Attributes: dto.FileAttributes{
				BlobSHA:   file.BlobSHA(),
				Path:      file.Path(),
				MimeType:  file.MimeType(),
				Size:      file.Size(),
				Extension: file.Extension(),
			},
		},
	})
}

// ListCommitEnrichments handles GET /api/v1/repositories/{id}/commits/{commit_sha}/enrichments.
func (r *RepositoriesRouter) ListCommitEnrichments(w http.ResponseWriter, req *http.Request) {
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

	// If enrichment service not configured, return empty list
	if r.enrichmentQueryService == nil {
		middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIListResponse{Data: []dto.EnrichmentData{}})
		return
	}

	// Parse optional type/subtype filters from query params
	var typFilter *enrichment.Type
	var subtypeFilter *enrichment.Subtype

	if typeStr := req.URL.Query().Get("enrichment_type"); typeStr != "" {
		typ := enrichment.Type(typeStr)
		typFilter = &typ
	}
	if subtypeStr := req.URL.Query().Get("enrichment_subtype"); subtypeStr != "" {
		sub := enrichment.Subtype(subtypeStr)
		subtypeFilter = &sub
	}

	enrichments, err := r.enrichmentQueryService.EnrichmentsForCommit(ctx, commitSHA, typFilter, subtypeFilter)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	data := make([]dto.EnrichmentData, 0, len(enrichments))
	for _, e := range enrichments {
		data = append(data, dto.EnrichmentData{
			Type: "enrichment",
			ID:   fmt.Sprintf("%d", e.ID()),
			Attributes: dto.EnrichmentAttributes{
				Type:      string(e.Type()),
				Subtype:   string(e.Subtype()),
				Content:   e.Content(),
				CreatedAt: e.CreatedAt(),
				UpdatedAt: e.UpdatedAt(),
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIListResponse{Data: data})
}

// GetCommitEnrichment handles GET /api/v1/repositories/{id}/commits/{commit_sha}/enrichments/{enrichment_id}.
func (r *RepositoriesRouter) GetCommitEnrichment(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")
	enrichmentIDStr := chi.URLParam(req, "enrichment_id")
	enrichmentID, err := strconv.ParseInt(enrichmentIDStr, 10, 64)
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

	// Check commit exists and belongs to this repo
	_, err = r.queryService.CommitBySHA(ctx, id, commitSHA)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// If enrichment repo not configured, return error
	if r.enrichmentRepo == nil {
		middleware.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "enrichments not configured"})
		return
	}

	e, err := r.enrichmentRepo.Get(ctx, enrichmentID)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIResponse{
		Data: dto.EnrichmentData{
			Type: "enrichment",
			ID:   fmt.Sprintf("%d", e.ID()),
			Attributes: dto.EnrichmentAttributes{
				Type:      string(e.Type()),
				Subtype:   string(e.Subtype()),
				Content:   e.Content(),
				CreatedAt: e.CreatedAt(),
				UpdatedAt: e.UpdatedAt(),
			},
		},
	})
}

// ListCommitSnippets handles GET /api/v1/repositories/{id}/commits/{commit_sha}/snippets.
// This redirects to the enrichments endpoint with type=development and subtype=snippet filters.
func (r *RepositoriesRouter) ListCommitSnippets(w http.ResponseWriter, req *http.Request) {
	idStr := chi.URLParam(req, "id")
	commitSHA := chi.URLParam(req, "commit_sha")

	// Redirect to enrichments endpoint with snippet filters
	redirectURL := fmt.Sprintf("/api/v1/repositories/%s/commits/%s/enrichments?enrichment_type=development&enrichment_subtype=snippet",
		idStr, commitSHA)

	http.Redirect(w, req, redirectURL, http.StatusPermanentRedirect)
}

// RescanCommit handles POST /api/v1/repositories/{id}/commits/{commit_sha}/rescan.
func (r *RepositoriesRouter) RescanCommit(w http.ResponseWriter, req *http.Request) {
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

	if err := r.syncService.RequestRescan(ctx, id, commitSHA); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// ListTags handles GET /api/v1/repositories/{id}/tags.
func (r *RepositoriesRouter) ListTags(w http.ResponseWriter, req *http.Request) {
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

	tags, err := r.queryService.TagsForRepository(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	data := make([]dto.TagData, 0, len(tags))
	for _, tag := range tags {
		data = append(data, dto.TagData{
			Type: "tag",
			ID:   fmt.Sprintf("%d", tag.ID()),
			Attributes: dto.TagAttributes{
				Name:      tag.Name(),
				CommitSHA: tag.CommitSHA(),
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.TagJSONAPIListResponse{Data: data})
}

// GetTag handles GET /api/v1/repositories/{id}/tags/{tag_id}.
func (r *RepositoriesRouter) GetTag(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	tagIDStr := chi.URLParam(req, "tag_id")
	tagID, err := strconv.ParseInt(tagIDStr, 10, 64)
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

	tag, err := r.queryService.TagByID(ctx, id, tagID)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.TagJSONAPIResponse{
		Data: dto.TagData{
			Type: "tag",
			ID:   fmt.Sprintf("%d", tag.ID()),
			Attributes: dto.TagAttributes{
				Name:      tag.Name(),
				CommitSHA: tag.CommitSHA(),
			},
		},
	})
}

// GetTrackingConfig handles GET /api/v1/repositories/{id}/tracking-config.
func (r *RepositoriesRouter) GetTrackingConfig(w http.ResponseWriter, req *http.Request) {
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

	tc := source.Repo().TrackingConfig()
	trackingType := "none"
	if tc.Branch() != "" {
		trackingType = "branch"
	} else if tc.Tag() != "" {
		trackingType = "tag"
	} else if tc.Commit() != "" {
		trackingType = "commit"
	}

	middleware.WriteJSON(w, http.StatusOK, dto.TrackingConfigResponse{
		Data: dto.TrackingConfigData{
			Type: "tracking_config",
			ID:   fmt.Sprintf("%d", id),
			Attributes: dto.TrackingConfigAttributes{
				Type:   trackingType,
				Branch: tc.Branch(),
				Tag:    tc.Tag(),
				Commit: tc.Commit(),
			},
		},
	})
}

// UpdateTrackingConfig handles PUT /api/v1/repositories/{id}/tracking-config.
func (r *RepositoriesRouter) UpdateTrackingConfig(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	var body dto.TrackingConfigRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	tc := git.NewTrackingConfig(body.Branch, body.Tag, body.Commit)

	source, err := r.syncService.UpdateTrackingConfig(ctx, id, tc)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	updatedTC := source.Repo().TrackingConfig()
	trackingType := "none"
	if updatedTC.Branch() != "" {
		trackingType = "branch"
	} else if updatedTC.Tag() != "" {
		trackingType = "tag"
	} else if updatedTC.Commit() != "" {
		trackingType = "commit"
	}

	middleware.WriteJSON(w, http.StatusOK, dto.TrackingConfigResponse{
		Data: dto.TrackingConfigData{
			Type: "tracking_config",
			ID:   fmt.Sprintf("%d", id),
			Attributes: dto.TrackingConfigAttributes{
				Type:   trackingType,
				Branch: updatedTC.Branch(),
				Tag:    updatedTC.Tag(),
				Commit: updatedTC.Commit(),
			},
		},
	})
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
