// Package v1 provides the v1 API routes.
package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

// RepositoriesRouter handles repository API endpoints.
type RepositoriesRouter struct {
	queryService           *service.RepositoryQuery
	syncService            *service.RepositorySync
	trackingQueryService   *service.TrackingQuery
	enrichmentQueryService *service.EnrichmentQuery
	enrichmentStore        enrichment.EnrichmentStore
	associationStore       enrichment.AssociationStore
	snippetStore           snippet.SnippetStore
	vectorStore            VectorStoreForAPI
	logger                 *slog.Logger
}

// VectorStoreForAPI provides embedding access for API endpoints.
type VectorStoreForAPI interface {
	EmbeddingsForSnippets(ctx context.Context, snippetIDs []string) ([]snippet.EmbeddingInfo, error)
}

// NewRepositoriesRouter creates a new RepositoriesRouter.
func NewRepositoriesRouter(
	queryService *service.RepositoryQuery,
	syncService *service.RepositorySync,
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
func (r *RepositoriesRouter) WithTrackingQueryService(svc *service.TrackingQuery) *RepositoriesRouter {
	r.trackingQueryService = svc
	return r
}

// WithEnrichmentServices sets the enrichment services for enrichment endpoints.
func (r *RepositoriesRouter) WithEnrichmentServices(
	querySvc *service.EnrichmentQuery,
	store enrichment.EnrichmentStore,
	assocStore enrichment.AssociationStore,
) *RepositoriesRouter {
	r.enrichmentQueryService = querySvc
	r.enrichmentStore = store
	r.associationStore = assocStore
	return r
}

// WithIndexingServices sets the indexing services for embedding endpoints.
func (r *RepositoriesRouter) WithIndexingServices(
	snippetStore snippet.SnippetStore,
	vectorStore VectorStoreForAPI,
) *RepositoriesRouter {
	r.snippetStore = snippetStore
	r.vectorStore = vectorStore
	return r
}

// Routes returns the chi router for repository endpoints.
func (r *RepositoriesRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/", r.List)
	router.Post("/", r.Add)
	router.Get("/{id}", r.Get)
	router.Delete("/{id}", r.Delete)
	router.Get("/{id}/status", r.GetStatus)
	router.Get("/{id}/status/summary", r.GetStatusSummary)
	router.Get("/{id}/commits", r.ListCommits)
	router.Get("/{id}/commits/{commit_sha}", r.GetCommit)
	router.Get("/{id}/commits/{commit_sha}/files", r.ListCommitFiles)
	router.Get("/{id}/commits/{commit_sha}/files/{blob_sha}", r.GetCommitFile)
	router.Get("/{id}/commits/{commit_sha}/enrichments", r.ListCommitEnrichments)
	router.Delete("/{id}/commits/{commit_sha}/enrichments", r.DeleteCommitEnrichments)
	router.Get("/{id}/commits/{commit_sha}/enrichments/{enrichment_id}", r.GetCommitEnrichment)
	router.Delete("/{id}/commits/{commit_sha}/enrichments/{enrichment_id}", r.DeleteCommitEnrichment)
	router.Get("/{id}/commits/{commit_sha}/snippets", r.ListCommitSnippets)
	router.Get("/{id}/commits/{commit_sha}/embeddings", r.ListCommitEmbeddings)
	router.Post("/{id}/commits/{commit_sha}/rescan", r.RescanCommit)
	router.Get("/{id}/tags", r.ListTags)
	router.Get("/{id}/tags/{tag_id}", r.GetTag)
	router.Get("/{id}/enrichments", r.ListRepositoryEnrichments)
	router.Get("/{id}/tracking-config", r.GetTrackingConfig)
	router.Put("/{id}/tracking-config", r.UpdateTrackingConfig)

	return router
}

// List handles GET /api/v1/repositories.
//
//	@Summary		List repositories
//	@Description	Get all tracked Git repositories
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	dto.RepositoryListResponse
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories [get]
func (r *RepositoriesRouter) List(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	sources, err := r.queryService.All(ctx)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	response := dto.RepositoryListResponse{
		Data: sourcesToDTO(sources),
	}

	middleware.WriteJSON(w, http.StatusOK, response)
}

// Get handles GET /api/v1/repositories/{id}.
//
//	@Summary		Get repository
//	@Description	Get a repository by ID
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Repository ID"
//	@Success		200	{object}	dto.RepositoryResponse
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id} [get]
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

	middleware.WriteJSON(w, http.StatusOK, dto.RepositoryResponse{Data: sourceToDTO(source)})
}

// Add handles POST /api/v1/repositories.
//
//	@Summary		Add repository
//	@Description	Add a new Git repository to track
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.RepositoryRequest	true	"Repository request"
//	@Success		201		{object}	dto.RepositoryResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories [post]
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

	var source service.Source
	var err error

	// If tracking config provided, use AddRepositoryWithTracking
	if body.Branch != "" || body.Tag != "" || body.Commit != "" {
		tc := repository.NewTrackingConfig(body.Branch, body.Tag, body.Commit)
		source, err = r.syncService.AddRepositoryWithTracking(ctx, body.RemoteURL, tc)
	} else {
		source, err = r.syncService.AddRepository(ctx, body.RemoteURL)
	}

	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, dto.RepositoryResponse{Data: sourceToDTO(source)})
}

// Delete handles DELETE /api/v1/repositories/{id}.
//
//	@Summary		Delete repository
//	@Description	Delete a repository by ID
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path	int	true	"Repository ID"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id} [delete]
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

// GetStatus handles GET /api/v1/repositories/{id}/status.
//
//	@Summary		Get repository status
//	@Description	Get indexing task status for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Repository ID"
//	@Success		200	{object}	dto.TaskStatusListResponse
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/status [get]
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
//
//	@Summary		Get repository status summary
//	@Description	Get aggregated indexing status summary for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Repository ID"
//	@Success		200	{object}	dto.RepositoryStatusSummaryResponse
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/status/summary [get]
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
//
//	@Summary		List commits
//	@Description	List commits for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Repository ID"
//	@Success		200	{object}	dto.CommitJSONAPIListResponse
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits [get]
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
//
//	@Summary		Get commit
//	@Description	Get a commit by SHA
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int		true	"Repository ID"
//	@Param			commit_sha	path		string	true	"Commit SHA"
//	@Success		200			{object}	dto.CommitJSONAPIResponse
//	@Failure		404			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha} [get]
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
//
//	@Summary		List commit files
//	@Description	List files for a commit
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int		true	"Repository ID"
//	@Param			commit_sha	path		string	true	"Commit SHA"
//	@Success		200			{object}	dto.FileJSONAPIListResponse
//	@Failure		404			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/files [get]
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
//
//	@Summary		Get commit file
//	@Description	Get a file by blob SHA
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int		true	"Repository ID"
//	@Param			commit_sha	path		string	true	"Commit SHA"
//	@Param			blob_sha	path		string	true	"Blob SHA"
//	@Success		200			{object}	dto.FileJSONAPIResponse
//	@Failure		404			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/files/{blob_sha} [get]
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
//
//	@Summary		List commit enrichments
//	@Description	List enrichments for a commit
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id					path		int		true	"Repository ID"
//	@Param			commit_sha			path		string	true	"Commit SHA"
//	@Param			enrichment_type		query		string	false	"Filter by enrichment type"
//	@Param			enrichment_subtype	query		string	false	"Filter by enrichment subtype"
//	@Success		200					{object}	dto.EnrichmentJSONAPIListResponse
//	@Failure		404					{object}	map[string]string
//	@Failure		500					{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/enrichments [get]
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
//
//	@Summary		Get commit enrichment
//	@Description	Get an enrichment by ID within commit context
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id				path		int		true	"Repository ID"
//	@Param			commit_sha		path		string	true	"Commit SHA"
//	@Param			enrichment_id	path		int		true	"Enrichment ID"
//	@Success		200				{object}	dto.EnrichmentJSONAPIResponse
//	@Failure		404				{object}	map[string]string
//	@Failure		500				{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/enrichments/{enrichment_id} [get]
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

	// If enrichment store not configured, return error
	if r.enrichmentStore == nil {
		middleware.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "enrichments not configured"})
		return
	}

	e, err := r.enrichmentStore.Get(ctx, enrichmentID)
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

// DeleteCommitEnrichments handles DELETE /api/v1/repositories/{id}/commits/{commit_sha}/enrichments.
//
//	@Summary		Delete commit enrichments
//	@Description	Delete all enrichments for a commit
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path	int		true	"Repository ID"
//	@Param			commit_sha	path	string	true	"Commit SHA"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/enrichments [delete]
func (r *RepositoriesRouter) DeleteCommitEnrichments(w http.ResponseWriter, req *http.Request) {
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

	// If association store not configured, return error
	if r.associationStore == nil {
		middleware.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "enrichments not configured"})
		return
	}

	// Get all enrichments for the commit first
	if r.enrichmentQueryService != nil {
		enrichments, err := r.enrichmentQueryService.EnrichmentsForCommit(ctx, commitSHA, nil, nil)
		if err == nil {
			// Delete each enrichment
			for _, e := range enrichments {
				_ = r.enrichmentStore.Delete(ctx, e)
			}
		}
	}

	// Delete all associations for the commit entity
	if err := r.associationStore.DeleteByEntityID(ctx, commitSHA); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteCommitEnrichment handles DELETE /api/v1/repositories/{id}/commits/{commit_sha}/enrichments/{enrichment_id}.
//
//	@Summary		Delete commit enrichment
//	@Description	Delete a specific enrichment from a commit
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id				path	int		true	"Repository ID"
//	@Param			commit_sha		path	string	true	"Commit SHA"
//	@Param			enrichment_id	path	int		true	"Enrichment ID"
//	@Success		204
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/enrichments/{enrichment_id} [delete]
func (r *RepositoriesRouter) DeleteCommitEnrichment(w http.ResponseWriter, req *http.Request) {
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

	// If enrichment store not configured, return error
	if r.enrichmentStore == nil {
		middleware.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "enrichments not configured"})
		return
	}

	// Get the enrichment
	e, err := r.enrichmentStore.Get(ctx, enrichmentID)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Delete associations for this enrichment
	if r.associationStore != nil {
		_ = r.associationStore.DeleteByEnrichmentID(ctx, enrichmentID)
	}

	// Delete the enrichment
	if err := r.enrichmentStore.Delete(ctx, e); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListCommitSnippets handles GET /api/v1/repositories/{id}/commits/{commit_sha}/snippets.
//
//	@Summary		List commit snippets
//	@Description	List code snippets for a commit
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int		true	"Repository ID"
//	@Param			commit_sha	path		string	true	"Commit SHA"
//	@Success		200			{object}	dto.SnippetListResponse
//	@Failure		401			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/snippets [get]
func (r *RepositoriesRouter) ListCommitSnippets(w http.ResponseWriter, req *http.Request) {
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

	// If snippet store not configured, return empty list
	if r.snippetStore == nil {
		middleware.WriteJSON(w, http.StatusOK, dto.SnippetListResponse{Data: []dto.SnippetData{}})
		return
	}

	snippets, err := r.snippetStore.SnippetsForCommit(ctx, commitSHA)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	data := make([]dto.SnippetData, 0, len(snippets))
	for _, s := range snippets {
		createdAt := s.CreatedAt()
		updatedAt := s.UpdatedAt()

		// Convert files to GitFileSchema
		derivesFrom := make([]dto.GitFileSchema, 0, len(s.DerivesFrom()))
		for _, f := range s.DerivesFrom() {
			derivesFrom = append(derivesFrom, dto.GitFileSchema{
				BlobSHA:  f.BlobSHA(),
				Path:     f.Path(),
				MimeType: f.MimeType(),
				Size:     f.Size(),
			})
		}

		// Convert enrichments
		enrichments := make([]dto.EnrichmentSchema, 0, len(s.Enrichments()))
		for _, e := range s.Enrichments() {
			enrichments = append(enrichments, dto.EnrichmentSchema{
				Type:    e.Type(),
				Content: e.Content(),
			})
		}

		data = append(data, dto.SnippetData{
			Type: "snippet",
			ID:   s.SHA(),
			Attributes: dto.SnippetAttributes{
				CreatedAt:   &createdAt,
				UpdatedAt:   &updatedAt,
				DerivesFrom: derivesFrom,
				Content: dto.SnippetContentSchema{
					Value:    s.Content(),
					Language: s.Extension(),
				},
				Enrichments:    enrichments,
				OriginalScores: []float64{},
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.SnippetListResponse{Data: data})
}

// RescanCommit handles POST /api/v1/repositories/{id}/commits/{commit_sha}/rescan.
//
//	@Summary		Rescan commit
//	@Description	Trigger a rescan of a specific commit
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path	int		true	"Repository ID"
//	@Param			commit_sha	path	string	true	"Commit SHA"
//	@Success		202
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/rescan [post]
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

// ListCommitEmbeddings handles GET /api/v1/repositories/{id}/commits/{commit_sha}/embeddings.
//
//	@Summary		List commit embeddings
//	@Description	List embeddings for snippets in a commit
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int		true	"Repository ID"
//	@Param			commit_sha	path		string	true	"Commit SHA"
//	@Param			full		query		bool	false	"Return full embedding vectors"
//	@Success		200			{object}	dto.EmbeddingJSONAPIListResponse
//	@Failure		404			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/embeddings [get]
func (r *RepositoriesRouter) ListCommitEmbeddings(w http.ResponseWriter, req *http.Request) {
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

	// If snippet/vector stores not configured, return empty list
	if r.snippetStore == nil || r.vectorStore == nil {
		middleware.WriteJSON(w, http.StatusOK, dto.EmbeddingJSONAPIListResponse{Data: []dto.EmbeddingData{}})
		return
	}

	// Parse optional full parameter (default: false, only return first 5 values)
	fullStr := req.URL.Query().Get("full")
	full := fullStr == "true"

	// Get snippets for the commit
	snippets, err := r.snippetStore.SnippetsForCommit(ctx, commitSHA)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if len(snippets) == 0 {
		middleware.WriteJSON(w, http.StatusOK, dto.EmbeddingJSONAPIListResponse{Data: []dto.EmbeddingData{}})
		return
	}

	// Extract snippet IDs
	snippetIDs := make([]string, 0, len(snippets))
	for _, s := range snippets {
		snippetIDs = append(snippetIDs, s.SHA())
	}

	// Get embeddings for snippets
	embeddings, err := r.vectorStore.EmbeddingsForSnippets(ctx, snippetIDs)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Build response
	data := make([]dto.EmbeddingData, 0, len(embeddings))
	for i, emb := range embeddings {
		var embVector []float64
		if full {
			embVector = emb.Embedding()
		} else {
			embVector = emb.EmbeddingTruncated(5)
		}

		data = append(data, dto.EmbeddingData{
			Type: "embedding",
			ID:   fmt.Sprintf("%d", i),
			Attributes: dto.EmbeddingAttributes{
				SnippetSHA:    emb.SnippetID(),
				EmbeddingType: emb.Type(),
				Embedding:     embVector,
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.EmbeddingJSONAPIListResponse{Data: data})
}

// ListRepositoryEnrichments handles GET /api/v1/repositories/{id}/enrichments.
// Lists the most recent enrichments for a repository across commits.
//
//	@Summary		List repository enrichments
//	@Description	List recent enrichments across commits for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id					path		int		true	"Repository ID"
//	@Param			enrichment_type		query		string	false	"Filter by enrichment type"
//	@Param			max_commits_to_check	query		int		false	"Max commits to check (default: 100)"
//	@Param			page_size			query		int		false	"Results per page (default: 20, max: 100)"
//	@Success		200					{object}	dto.EnrichmentJSONAPIListResponse
//	@Failure		404					{object}	map[string]string
//	@Failure		500					{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/enrichments [get]
func (r *RepositoriesRouter) ListRepositoryEnrichments(w http.ResponseWriter, req *http.Request) {
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

	// If enrichment service not configured, return empty list
	if r.enrichmentQueryService == nil {
		middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIListResponse{Data: []dto.EnrichmentData{}})
		return
	}

	// Parse optional type filter
	var typFilter *enrichment.Type
	if typeStr := req.URL.Query().Get("enrichment_type"); typeStr != "" {
		typ := enrichment.Type(typeStr)
		typFilter = &typ
	}

	// Parse max_commits_to_check (default: 100)
	maxCommits := 100
	if maxStr := req.URL.Query().Get("max_commits_to_check"); maxStr != "" {
		if parsed, err := strconv.Atoi(maxStr); err == nil && parsed > 0 {
			maxCommits = parsed
		}
	}

	// Parse page_size (default: 20, max: 100)
	pageSize := 20
	if sizeStr := req.URL.Query().Get("page_size"); sizeStr != "" {
		if parsed, err := strconv.Atoi(sizeStr); err == nil && parsed > 0 {
			pageSize = parsed
			if pageSize > 100 {
				pageSize = 100
			}
		}
	}

	// Get recent commits for the repository
	commits, err := r.queryService.CommitsForRepository(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Limit to most recent commits
	if len(commits) > maxCommits {
		commits = commits[:maxCommits]
	}

	// Extract commit SHAs
	commitSHAs := make([]string, 0, len(commits))
	for _, c := range commits {
		commitSHAs = append(commitSHAs, c.SHA())
	}

	// Get enrichments across commits
	enrichments, err := r.enrichmentQueryService.EnrichmentsForCommits(ctx, commitSHAs, typFilter, pageSize)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Build response
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

// ListTags handles GET /api/v1/repositories/{id}/tags.
//
//	@Summary		List tags
//	@Description	List tags for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Repository ID"
//	@Success		200	{object}	dto.TagJSONAPIListResponse
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/tags [get]
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
				Name:            tag.Name(),
				TargetCommitSHA: tag.CommitSHA(),
				IsVersionTag:    isVersionTag(tag.Name()),
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.TagJSONAPIListResponse{Data: data})
}

// GetTag handles GET /api/v1/repositories/{id}/tags/{tag_id}.
//
//	@Summary		Get tag
//	@Description	Get a tag by ID
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int	true	"Repository ID"
//	@Param			tag_id	path		int	true	"Tag ID"
//	@Success		200		{object}	dto.TagJSONAPIResponse
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/tags/{tag_id} [get]
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
				Name:            tag.Name(),
				TargetCommitSHA: tag.CommitSHA(),
				IsVersionTag:    isVersionTag(tag.Name()),
			},
		},
	})
}

// isVersionTag returns true if the tag name looks like a version tag.
// Version tags typically start with 'v' followed by a digit, or match semver patterns.
func isVersionTag(name string) bool {
	if len(name) == 0 {
		return false
	}
	// Check for v-prefix version tags (v1.0.0, v2, etc.)
	if name[0] == 'v' && len(name) > 1 && name[1] >= '0' && name[1] <= '9' {
		return true
	}
	// Check for plain numeric version tags (1.0.0, 2.0, etc.)
	if name[0] >= '0' && name[0] <= '9' {
		return true
	}
	return false
}

// GetTrackingConfig handles GET /api/v1/repositories/{id}/tracking-config.
//
//	@Summary		Get tracking config
//	@Description	Get current tracking configuration for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Repository ID"
//	@Success		200	{object}	dto.TrackingConfigResponse
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/tracking-config [get]
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
	middleware.WriteJSON(w, http.StatusOK, trackingConfigToResponse(tc))
}

// UpdateTrackingConfig handles PUT /api/v1/repositories/{id}/tracking-config.
//
//	@Summary		Update tracking config
//	@Description	Update tracking configuration for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int									true	"Repository ID"
//	@Param			body	body		dto.TrackingConfigUpdateRequest		true	"Tracking config"
//	@Success		200		{object}	dto.TrackingConfigResponse
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/tracking-config [put]
func (r *RepositoriesRouter) UpdateTrackingConfig(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	var body dto.TrackingConfigUpdateRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Convert JSON:API request to domain tracking config
	var branch, tag, commit string
	switch body.Data.Attributes.Mode {
	case dto.TrackingModeBranch:
		if body.Data.Attributes.Value != nil {
			branch = *body.Data.Attributes.Value
		}
	case dto.TrackingModeTag:
		if body.Data.Attributes.Value != nil {
			tag = *body.Data.Attributes.Value
		}
	}

	tc := repository.NewTrackingConfig(branch, tag, commit)

	source, err := r.syncService.UpdateTrackingConfig(ctx, id, tc)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	updatedTC := source.Repo().TrackingConfig()
	middleware.WriteJSON(w, http.StatusOK, trackingConfigToResponse(updatedTC))
}

func trackingConfigToResponse(tc repository.TrackingConfig) dto.TrackingConfigResponse {
	mode := dto.TrackingModeBranch
	var value *string

	if tc.Tag() != "" {
		mode = dto.TrackingModeTag
		v := tc.Tag()
		value = &v
	} else if tc.Branch() != "" {
		v := tc.Branch()
		value = &v
	}

	return dto.TrackingConfigResponse{
		Data: dto.TrackingConfigData{
			Type: "tracking-config",
			Attributes: dto.TrackingConfigAttributes{
				Mode:  mode,
				Value: value,
			},
		},
	}
}

func sourcesToDTO(sources []service.Source) []dto.RepositoryData {
	result := make([]dto.RepositoryData, len(sources))
	for i, source := range sources {
		result[i] = sourceToDTO(source)
	}
	return result
}

func sourceToDTO(source service.Source) dto.RepositoryData {
	repo := source.Repo()
	createdAt := repo.CreatedAt()
	updatedAt := repo.UpdatedAt()
	clonedPath := repo.WorkingCopy().Path()

	attrs := dto.RepositoryAttributes{
		RemoteURI:   repo.RemoteURL(),
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
		ClonedPath:  &clonedPath,
		NumCommits:  0,
		NumBranches: 0,
		NumTags:     0,
	}

	if tc := repo.TrackingConfig(); tc.Branch() != "" {
		branch := tc.Branch()
		attrs.TrackingBranch = &branch
	}

	return dto.RepositoryData{
		Type:       "repository",
		ID:         fmt.Sprintf("%d", repo.ID()),
		Attributes: attrs,
	}
}
