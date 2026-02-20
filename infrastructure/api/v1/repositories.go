// Package v1 provides the v1 API routes.
package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/domain/wiki"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
	"github.com/helixml/kodit/internal/database"
)

// RepositoriesRouter handles repository API endpoints.
type RepositoriesRouter struct {
	client *kodit.Client
	logger *slog.Logger
}

// NewRepositoriesRouter creates a new RepositoriesRouter.
func NewRepositoriesRouter(client *kodit.Client) *RepositoriesRouter {
	return &RepositoriesRouter{
		client: client,
		logger: client.Logger(),
	}
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
	router.Get("/{id}/commits/{commit_sha}/embeddings", r.ListCommitEmbeddingsDeprecated)
	router.Post("/{id}/sync", r.Sync)
	router.Post("/{id}/commits/{commit_sha}/rescan", r.RescanCommit)
	router.Get("/{id}/tags", r.ListTags)
	router.Get("/{id}/tags/{tag_name}", r.GetTag)
	router.Get("/{id}/enrichments", r.ListRepositoryEnrichments)
	router.Post("/{id}/wiki/generate", r.GenerateWiki)
	router.Get("/{id}/wiki", r.GetWikiTree)
	router.Get("/{id}/wiki/*", r.GetWikiPage)
	router.Get("/{id}/tracking-config", r.GetTrackingConfig)
	router.Put("/{id}/tracking-config", r.UpdateTrackingConfig)
	router.Get("/{id}/blob/{blob_name}/*", r.GetBlob)

	return router
}

// repositoryID parses the "id" URL parameter and verifies the repository exists.
func (r *RepositoriesRouter) repositoryID(req *http.Request) (int64, error) {
	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, err
	}

	_, err = r.client.Repositories.Get(req.Context(), repository.WithID(id))
	if err != nil {
		return 0, err
	}

	return id, nil
}

// List handles GET /api/v1/repositories.
//
//	@Summary		List repositories
//	@Description	Get all tracked Git repositories
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			page		query	int	false	"Page number (default: 1)"
//	@Param			page_size	query	int	false	"Results per page (default: 20, max: 100)"
//	@Success		200	{object}	dto.RepositoryListResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories [get]
func (r *RepositoriesRouter) List(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	repos, err := r.client.Repositories.Find(ctx, pagination.Options()...)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	total, err := r.client.Repositories.Count(ctx)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	data := make([]dto.RepositoryData, 0, len(repos))
	for _, repo := range repos {
		repoID := repo.ID()
		numCommits, _ := r.client.Commits.Count(ctx, repository.WithRepoID(repoID))
		branches, _ := r.client.Repositories.BranchesForRepository(ctx, repoID)
		numTags, _ := r.client.Tags.Count(ctx, repository.WithRepoID(repoID))
		data = append(data, repoToDTO(repo, numCommits, int64(len(branches)), numTags))
	}

	response := dto.RepositoryListResponse{
		Data:  data,
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
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
//	@Success		200	{object}	dto.RepositoryDetailsResponse
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
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

	repo, err := r.client.Repositories.Get(ctx, repository.WithID(id))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	branches, err := r.client.Repositories.BranchesForRepository(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commits, err := r.client.Commits.Find(ctx, repository.WithRepoID(id), repository.WithLimit(10))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	branchData := make([]dto.RepositoryBranchData, 0, len(branches))
	for _, b := range branches {
		branchData = append(branchData, dto.RepositoryBranchData{
			Name:      b.Name(),
			IsDefault: b.IsDefault(),
		})
	}

	commitData := make([]dto.RepositoryCommitData, 0, len(commits))
	for _, c := range commits {
		commitData = append(commitData, dto.RepositoryCommitData{
			SHA:       c.SHA(),
			Message:   c.Message(),
			Author:    c.Author().Name(),
			Timestamp: c.CommittedAt(),
		})
	}

	numCommits, err := r.client.Commits.Count(ctx, repository.WithRepoID(id))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}
	numTags, err := r.client.Tags.Count(ctx, repository.WithRepoID(id))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.RepositoryDetailsResponse{
		Data:          repoToDTO(repo, numCommits, int64(len(branches)), numTags),
		Branches:      branchData,
		RecentCommits: commitData,
	})
}

// Add handles POST /api/v1/repositories.
//
//	@Summary		Add repository
//	@Description	Add a new Git repository to track
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.RepositoryCreateRequest	true	"Repository request"
//	@Success		200		{object}	dto.RepositoryResponse	"Repository already exists"
//	@Success		201		{object}	dto.RepositoryResponse	"Repository created"
//	@Failure		400		{object}	middleware.JSONAPIErrorResponse
//	@Failure		500		{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories [post]
func (r *RepositoriesRouter) Add(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var body dto.RepositoryCreateRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if body.Data.Attributes.RemoteURI == "" {
		middleware.WriteError(w, req, fmt.Errorf("remote_uri is required: %w", middleware.ErrValidation), r.logger)
		return
	}

	source, created, err := r.client.Repositories.Add(ctx, &service.RepositoryAddParams{
		URL: body.Data.Attributes.RemoteURI,
	})
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}

	middleware.WriteJSON(w, status, dto.RepositoryResponse{Data: repoToDTO(source.Repo(), 0, 0, 0)})
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
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
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

	if err := r.client.Repositories.Delete(ctx, id); err != nil {
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
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/status [get]
func (r *RepositoriesRouter) GetStatus(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	statuses, err := r.client.Tracking.Statuses(ctx, id)
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
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/status/summary [get]
func (r *RepositoriesRouter) GetStatusSummary(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	summary, err := r.client.Tracking.Summary(ctx, id)
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
//	@Param			id			path		int	true	"Repository ID"
//	@Param			page		query		int	false	"Page number (default: 1)"
//	@Param			page_size	query		int	false	"Results per page (default: 20, max: 100)"
//	@Success		200	{object}	dto.CommitJSONAPIListResponse
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits [get]
func (r *RepositoriesRouter) ListCommits(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	filterOpts := []repository.Option{repository.WithRepoID(id)}
	commits, err := r.client.Commits.Find(ctx, append(filterOpts, pagination.Options()...)...)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	total, err := r.client.Commits.Count(ctx, filterOpts...)
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

	middleware.WriteJSON(w, http.StatusOK, dto.CommitJSONAPIListResponse{
		Data:  data,
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
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
//	@Failure		404			{object}	middleware.JSONAPIErrorResponse
//	@Failure		500			{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha} [get]
func (r *RepositoriesRouter) GetCommit(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	commit, err := r.client.Commits.Get(ctx, repository.WithRepoID(id), repository.WithSHA(commitSHA))
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
//	@Param			page		query		int		false	"Page number (default: 1)"
//	@Param			page_size	query		int		false	"Results per page (default: 20, max: 100)"
//	@Success		200			{object}	dto.FileJSONAPIListResponse
//	@Failure		404			{object}	middleware.JSONAPIErrorResponse
//	@Failure		500			{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/files [get]
func (r *RepositoriesRouter) ListCommitFiles(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check commit exists and belongs to this repo
	_, err = r.client.Commits.Get(ctx, repository.WithRepoID(id), repository.WithSHA(commitSHA))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	filterOpts := []repository.Option{repository.WithCommitSHA(commitSHA)}
	files, err := r.client.Files.Find(ctx, append(filterOpts, pagination.Options()...)...)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	total, err := r.client.Files.Count(ctx, filterOpts...)
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

	middleware.WriteJSON(w, http.StatusOK, dto.FileJSONAPIListResponse{
		Data:  data,
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
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
//	@Failure		404			{object}	middleware.JSONAPIErrorResponse
//	@Failure		500			{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/files/{blob_sha} [get]
func (r *RepositoriesRouter) GetCommitFile(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")
	blobSHA := chi.URLParam(req, "blob_sha")

	// Check commit exists and belongs to this repo
	_, err = r.client.Commits.Get(ctx, repository.WithRepoID(id), repository.WithSHA(commitSHA))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	file, err := r.client.Files.Get(ctx, repository.WithCommitSHA(commitSHA), repository.WithBlobSHA(blobSHA))
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
//	@Param			page				query		int		false	"Page number (default: 1)"
//	@Param			page_size			query		int		false	"Results per page (default: 20, max: 100)"
//	@Success		200					{object}	dto.EnrichmentJSONAPIListResponse
//	@Failure		404					{object}	middleware.JSONAPIErrorResponse
//	@Failure		500					{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/enrichments [get]
func (r *RepositoriesRouter) ListCommitEnrichments(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check commit exists and belongs to this repo
	_, err = r.client.Commits.Get(ctx, repository.WithRepoID(id), repository.WithSHA(commitSHA))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Build enrichment list params from query params
	params := &service.EnrichmentListParams{
		CommitSHA: commitSHA,
		Limit:     pagination.Limit(),
		Offset:    pagination.Offset(),
	}
	if typeStr := req.URL.Query().Get("enrichment_type"); typeStr != "" {
		t := enrichment.Type(typeStr)
		params.Type = &t
	}
	if subtypeStr := req.URL.Query().Get("enrichment_subtype"); subtypeStr != "" {
		s := enrichment.Subtype(subtypeStr)
		params.Subtype = &s
	}

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

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIListResponse{
		Data:  enrichmentsToJSONAPIDTO(enrichments, lineRanges),
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
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
//	@Failure		404				{object}	middleware.JSONAPIErrorResponse
//	@Failure		500				{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/enrichments/{enrichment_id} [get]
func (r *RepositoriesRouter) GetCommitEnrichment(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
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

	// Check commit exists and belongs to this repo
	_, err = r.client.Commits.Get(ctx, repository.WithRepoID(id), repository.WithSHA(commitSHA))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	e, err := r.client.Enrichments.Get(ctx, repository.WithID(enrichmentID))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	lineRanges, err := r.client.Enrichments.LineRanges(ctx, []int64{enrichmentID})
	if err != nil {
		r.logger.Warn("failed to fetch line ranges", "error", err)
		lineRanges = map[string]chunk.LineRange{}
	}

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIResponse{
		Data: enrichmentToJSONAPIDTO(e, lineRanges),
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
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/enrichments [delete]
func (r *RepositoriesRouter) DeleteCommitEnrichments(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check commit exists and belongs to this repo
	_, err = r.client.Commits.Get(ctx, repository.WithRepoID(id), repository.WithSHA(commitSHA))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	enrichments, err := r.client.Enrichments.List(ctx, &service.EnrichmentListParams{CommitSHA: commitSHA})
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if len(enrichments) > 0 {
		ids := make([]int64, len(enrichments))
		for i, e := range enrichments {
			ids[i] = e.ID()
		}
		if err := r.client.Enrichments.DeleteBy(ctx, repository.WithIDIn(ids)); err != nil {
			middleware.WriteError(w, req, err, r.logger)
			return
		}
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
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/enrichments/{enrichment_id} [delete]
func (r *RepositoriesRouter) DeleteCommitEnrichment(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
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

	// Check commit exists and belongs to this repo
	_, err = r.client.Commits.Get(ctx, repository.WithRepoID(id), repository.WithSHA(commitSHA))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if err := r.client.Enrichments.DeleteBy(ctx, repository.WithID(enrichmentID)); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListCommitSnippets handles GET /api/v1/repositories/{id}/commits/{commit_sha}/snippets.
//
//	@Summary		List commit snippets
//	@Description	List code snippets for a commit (backed by enrichments)
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int		true	"Repository ID"
//	@Param			commit_sha	path		string	true	"Commit SHA"
//	@Param			page		query		int		false	"Page number (default: 1)"
//	@Param			page_size	query		int		false	"Results per page (default: 20, max: 100)"
//	@Success		200			{object}	dto.SnippetListResponse
//	@Failure		401			{object}	middleware.JSONAPIErrorResponse
//	@Failure		404			{object}	middleware.JSONAPIErrorResponse
//	@Failure		500			{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/snippets [get]
func (r *RepositoriesRouter) ListCommitSnippets(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check commit exists and belongs to this repo
	_, err = r.client.Commits.Get(ctx, repository.WithRepoID(id), repository.WithSHA(commitSHA))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	typDev := enrichment.TypeDevelopment
	subSnippet := enrichment.SubtypeSnippet
	params := &service.EnrichmentListParams{
		CommitSHA: commitSHA,
		Type:      &typDev,
		Subtype:   &subSnippet,
		Limit:     pagination.Limit(),
		Offset:    pagination.Offset(),
	}

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

	// Fetch related enrichments (e.g., summaries) for the snippets
	ids := make([]int64, len(enrichments))
	for i, e := range enrichments {
		ids[i] = e.ID()
	}
	related, err := r.client.Enrichments.RelatedEnrichments(ctx, ids)
	if err != nil {
		r.logger.Warn("failed to fetch related enrichments", "error", err)
		related = map[string][]enrichment.Enrichment{}
	}

	fileMap, err := sourceFileMap(ctx, r.client, ids)
	if err != nil {
		r.logger.Warn("failed to fetch source files", "error", err)
		fileMap = map[string][]repository.File{}
	}

	data := make([]dto.SnippetData, 0, len(enrichments))
	for _, e := range enrichments {
		createdAt := e.CreatedAt()
		updatedAt := e.UpdatedAt()
		idStr := fmt.Sprintf("%d", e.ID())

		enrichmentSchemas := make([]dto.EnrichmentSchema, 0)
		for _, rel := range related[idStr] {
			enrichmentSchemas = append(enrichmentSchemas, dto.EnrichmentSchema{
				Type:    string(rel.Subtype()),
				Content: rel.Content(),
			})
		}

		derivesFrom := make([]dto.GitFileSchema, 0)
		for _, f := range fileMap[idStr] {
			derivesFrom = append(derivesFrom, dto.GitFileSchema{
				BlobSHA:  f.BlobSHA(),
				Path:     f.Path(),
				MimeType: f.MimeType(),
				Size:     f.Size(),
			})
		}

		data = append(data, dto.SnippetData{
			Type: string(e.Subtype()),
			ID:   idStr,
			Attributes: dto.SnippetAttributes{
				CreatedAt:   &createdAt,
				UpdatedAt:   &updatedAt,
				DerivesFrom: derivesFrom,
				Content: dto.SnippetContentSchema{
					Value:    e.Content(),
					Language: e.Language(),
				},
				Enrichments:    enrichmentSchemas,
				OriginalScores: []float64{},
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.SnippetListResponse{
		Data:  data,
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
}

// ListCommitEmbeddingsDeprecated handles GET /api/v1/repositories/{id}/commits/{commit_sha}/embeddings.
// This endpoint is deprecated. Embeddings are an internal implementation detail of snippets and enrichments.
//
//	@Summary		List commit embeddings (deprecated)
//	@Description	This endpoint has been removed. Embeddings are an internal detail of snippets and enrichments.
//	@Tags			repositories
//	@Produce		json
//	@Param			id			path		int		true	"Repository ID"
//	@Param			commit_sha	path		string	true	"Commit SHA"
//	@Success		410			{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Deprecated
//	@Router			/repositories/{id}/commits/{commit_sha}/embeddings [get]
func (r *RepositoriesRouter) ListCommitEmbeddingsDeprecated(w http.ResponseWriter, req *http.Request) {
	middleware.WriteError(w, req, middleware.NewAPIError(
		http.StatusGone,
		"this endpoint has been removed — embeddings are an internal detail of snippets and enrichments",
		nil,
	), r.logger)
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
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/rescan [post]
func (r *RepositoriesRouter) RescanCommit(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check commit exists and belongs to this repo
	_, err = r.client.Commits.Get(ctx, repository.WithRepoID(id), repository.WithSHA(commitSHA))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if err := r.client.Repositories.Rescan(ctx, &service.RescanParams{RepositoryID: id, CommitSHA: commitSHA}); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Sync handles POST /api/v1/repositories/{id}/sync.
//
//	@Summary		Sync repository
//	@Description	Trigger a sync (git fetch + branch scan + commit indexing) for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path	int	true	"Repository ID"
//	@Success		202
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/sync [post]
func (r *RepositoriesRouter) Sync(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if err := r.client.Repositories.Sync(ctx, id); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusAccepted)
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
//	@Param			page				query		int		false	"Page number (default: 1)"
//	@Param			page_size			query		int		false	"Results per page (default: 20, max: 100)"
//	@Success		200					{object}	dto.EnrichmentJSONAPIListResponse
//	@Failure		404					{object}	middleware.JSONAPIErrorResponse
//	@Failure		500					{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/enrichments [get]
func (r *RepositoriesRouter) ListRepositoryEnrichments(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Build enrichment list params from query params
	var typ *enrichment.Type
	if typeStr := req.URL.Query().Get("enrichment_type"); typeStr != "" {
		t := enrichment.Type(typeStr)
		typ = &t
	}

	// Parse max_commits_to_check (default: 100)
	maxCommits := 100
	if maxStr := req.URL.Query().Get("max_commits_to_check"); maxStr != "" {
		if parsed, err := strconv.Atoi(maxStr); err == nil && parsed > 0 {
			maxCommits = parsed
		}
	}

	// Get recent commits for the repository
	commits, err := r.client.Commits.Find(ctx, repository.WithRepoID(id), repository.WithLimit(maxCommits))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Extract commit SHAs
	commitSHAs := make([]string, 0, len(commits))
	for _, c := range commits {
		commitSHAs = append(commitSHAs, c.SHA())
	}

	params := &service.EnrichmentListParams{
		CommitSHAs: commitSHAs,
		Type:       typ,
		Limit:      pagination.Limit(),
		Offset:     pagination.Offset(),
	}

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

	// Batch-fetch line ranges for the enrichments on this page.
	ids := make([]int64, len(enrichments))
	for i, e := range enrichments {
		ids[i] = e.ID()
	}
	lineRanges, err := r.client.Enrichments.LineRanges(ctx, ids)
	if err != nil {
		r.logger.Warn("failed to fetch line ranges", "error", err)
		lineRanges = map[string]chunk.LineRange{}
	}

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIListResponse{
		Data:  enrichmentsToJSONAPIDTO(enrichments, lineRanges),
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
}

// GetWikiTree handles GET /api/v1/repositories/{id}/wiki.
//
//	@Summary		Get wiki tree
//	@Description	Get the wiki navigation tree (titles and paths, no content)
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path		int	true	"Repository ID"
//	@Success		200	{object}	dto.WikiTreeResponse
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/wiki [get]
func (r *RepositoriesRouter) GetWikiTree(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	parsed, err := r.latestWiki(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	pathIndex := parsed.PathIndex()
	data := make([]dto.WikiTreeNode, 0, len(parsed.Pages()))
	for _, p := range parsed.Pages() {
		data = append(data, wikiTreeNode(p, pathIndex))
	}

	middleware.WriteJSON(w, http.StatusOK, dto.WikiTreeResponse{Data: data})
}

// GetWikiPage handles GET /api/v1/repositories/{id}/wiki/*.
// Serves a single wiki page as raw markdown with rewritten links.
//
//	@Summary		Get wiki page
//	@Description	Get a wiki page by hierarchical path as raw markdown
//	@Tags			repositories
//	@Produce		text/markdown
//	@Param			id		path		int		true	"Repository ID"
//	@Param			path	path		string	true	"Wiki page path (e.g. architecture/database-layer.md)"
//	@Success		200		{string}	string
//	@Failure		404		{object}	middleware.JSONAPIErrorResponse
//	@Failure		500		{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/wiki/{path} [get]
func (r *RepositoriesRouter) GetWikiPage(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	pagePath := strings.TrimPrefix(chi.URLParam(req, "*"), "/")
	pagePath = strings.TrimSuffix(pagePath, ".md")

	parsed, err := r.latestWiki(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	page, ok := parsed.PageByPath(pagePath)
	if !ok {
		middleware.WriteError(w, req, fmt.Errorf("wiki page %q not found: %w", pagePath, database.ErrNotFound), r.logger)
		return
	}

	pathIndex := parsed.PathIndex()
	urlPrefix := fmt.Sprintf("/api/v1/repositories/%d/wiki", id)
	rewritten := wiki.NewRewrittenContent(page.Content(), pathIndex, urlPrefix, ".md")

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, rewritten.String()); err != nil {
		r.logger.Error("failed to write wiki page response", "error", err)
	}
}

// GenerateWiki handles POST /api/v1/repositories/{id}/wiki/generate.
//
//	@Summary		Generate wiki
//	@Description	Trigger wiki generation for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id	path	int	true	"Repository ID"
//	@Success		202
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/wiki/generate [post]
func (r *RepositoriesRouter) GenerateWiki(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Find the latest commit for this repository.
	commits, err := r.client.Commits.Find(ctx, repository.WithRepoID(id), repository.WithLimit(1))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}
	if len(commits) == 0 {
		middleware.WriteError(w, req, fmt.Errorf("no commits found: %w", database.ErrNotFound), r.logger)
		return
	}

	payload := map[string]any{
		"repository_id": id,
		"commit_sha":    commits[0].SHA(),
	}
	operations := []task.Operation{task.OperationGenerateWikiForCommit}
	if err := r.client.Tasks.EnqueueOperations(ctx, operations, task.PriorityUserInitiated, payload); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// latestWiki finds the most recent wiki enrichment for a repository.
func (r *RepositoriesRouter) latestWiki(ctx context.Context, repoID int64) (wiki.Wiki, error) {
	commits, err := r.client.Commits.Find(ctx, repository.WithRepoID(repoID), repository.WithLimit(10))
	if err != nil {
		return wiki.Wiki{}, fmt.Errorf("find commits: %w", err)
	}

	shas := make([]string, 0, len(commits))
	for _, c := range commits {
		shas = append(shas, c.SHA())
	}

	if len(shas) == 0 {
		return wiki.Wiki{}, fmt.Errorf("no commits found for repository: %w", database.ErrNotFound)
	}

	wikiType := enrichment.TypeUsage
	wikiSubtype := enrichment.SubtypeWiki
	enrichments, err := r.client.Enrichments.List(ctx, &service.EnrichmentListParams{
		CommitSHAs: shas,
		Type:       &wikiType,
		Subtype:    &wikiSubtype,
		Limit:      1,
	})
	if err != nil {
		return wiki.Wiki{}, fmt.Errorf("find wiki enrichment: %w", err)
	}

	if len(enrichments) == 0 {
		return wiki.Wiki{}, fmt.Errorf("no wiki found for repository: %w", database.ErrNotFound)
	}

	parsed, err := wiki.ParseWiki(enrichments[0].Content())
	if err != nil {
		return wiki.Wiki{}, fmt.Errorf("parse wiki content: %w", err)
	}

	return parsed, nil
}

func wikiTreeNode(p wiki.Page, pathIndex map[string]string) dto.WikiTreeNode {
	children := make([]dto.WikiTreeNode, 0, len(p.Children()))
	for _, child := range p.Children() {
		children = append(children, wikiTreeNode(child, pathIndex))
	}

	return dto.WikiTreeNode{
		Slug:     p.Slug(),
		Title:    p.Title(),
		Path:     pathIndex[p.Slug()] + ".md",
		Children: children,
	}
}

// ListTags handles GET /api/v1/repositories/{id}/tags.
//
//	@Summary		List tags
//	@Description	List tags for a repository
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int	true	"Repository ID"
//	@Param			page		query		int	false	"Page number (default: 1)"
//	@Param			page_size	query		int	false	"Results per page (default: 20, max: 100)"
//	@Success		200			{object}	dto.TagJSONAPIListResponse
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/tags [get]
func (r *RepositoriesRouter) ListTags(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	filterOpts := []repository.Option{repository.WithRepoID(id)}
	tags, err := r.client.Tags.Find(ctx, append(filterOpts, pagination.Options()...)...)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	total, err := r.client.Tags.Count(ctx, filterOpts...)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	data := make([]dto.TagData, 0, len(tags))
	for _, tag := range tags {
		data = append(data, dto.TagData{
			Type: "tag",
			ID:   tag.Name(),
			Attributes: dto.TagAttributes{
				Name:            tag.Name(),
				TargetCommitSHA: tag.CommitSHA(),
				IsVersionTag:    isVersionTag(tag.Name()),
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.TagJSONAPIListResponse{
		Data:  data,
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
}

// GetTag handles GET /api/v1/repositories/{id}/tags/{tag_name}.
//
//	@Summary		Get tag
//	@Description	Get a tag by name
//	@Tags			repositories
//	@Accept			json
//	@Produce		json
//	@Param			id			path		int		true	"Repository ID"
//	@Param			tag_name	path		string	true	"Tag name"
//	@Success		200			{object}	dto.TagJSONAPIResponse
//	@Failure		404			{object}	middleware.JSONAPIErrorResponse
//	@Failure		500			{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/tags/{tag_name} [get]
func (r *RepositoriesRouter) GetTag(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	id, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	tagName := chi.URLParam(req, "tag_name")

	tag, err := r.client.Tags.Get(ctx, repository.WithName(tagName), repository.WithRepoID(id))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.TagJSONAPIResponse{
		Data: dto.TagData{
			Type: "tag",
			ID:   tag.Name(),
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
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
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

	repo, err := r.client.Repositories.Get(ctx, repository.WithID(id))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	tc := repo.TrackingConfig()
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
//	@Failure		404		{object}	middleware.JSONAPIErrorResponse
//	@Failure		500		{object}	middleware.JSONAPIErrorResponse
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

	// Convert JSON:API request to tracking config params
	var branch, tag string
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

	source, err := r.client.Repositories.UpdateTrackingConfig(ctx, id, &service.TrackingConfigParams{
		Branch: branch,
		Tag:    tag,
	})
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

func repoToDTO(repo repository.Repository, numCommits, numBranches, numTags int64) dto.RepositoryData {
	createdAt := repo.CreatedAt()
	updatedAt := repo.UpdatedAt()
	clonedPath := repo.WorkingCopy().Path()

	attrs := dto.RepositoryAttributes{
		RemoteURI:   repo.RemoteURL(),
		CreatedAt:   &createdAt,
		UpdatedAt:   &updatedAt,
		ClonedPath:  &clonedPath,
		NumCommits:  int(numCommits),
		NumBranches: int(numBranches),
		NumTags:     int(numTags),
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

// GetBlob handles GET /api/v1/repositories/{id}/blob/{blob_name}/*.
//
//	@Summary		Get raw file content
//	@Description	Returns raw file content from a Git repository at a given blob reference (commit SHA, tag, or branch)
//	@Tags			repositories
//	@Produce		octet-stream
//	@Produce		plain
//	@Param			id			path	int		true	"Repository ID"
//	@Param			blob_name	path	string	true	"Commit SHA, tag name, or branch name"
//	@Param			path		path	string	true	"File path within the repository"
//	@Param			lines			query	string	false	"Line ranges to extract (e.g. L17-L26,L45,L55-L90)"
//	@Param			line_numbers	query	bool	false	"Prefix each line with its 1-based line number"
//	@Success		200
//	@Failure		400	{object}	middleware.JSONAPIErrorResponse
//	@Failure		404	{object}	middleware.JSONAPIErrorResponse
//	@Failure		500	{object}	middleware.JSONAPIErrorResponse
//	@Router			/repositories/{id}/blob/{blob_name}/{path} [get]
func (r *RepositoriesRouter) GetBlob(w http.ResponseWriter, req *http.Request) {
	repoID, err := r.repositoryID(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	blobName := chi.URLParam(req, "blob_name")
	rawPath := strings.TrimPrefix(chi.URLParam(req, "*"), "/")
	filePath, err := url.PathUnescape(rawPath)
	if err != nil {
		filePath = rawPath
	}

	if blobName == "" || filePath == "" {
		middleware.WriteError(w, req, fmt.Errorf("blob_name and file path are required: %w", middleware.ErrValidation), r.logger)
		return
	}

	result, err := r.client.Blobs.Content(req.Context(), repoID, blobName, filePath)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	w.Header().Set("X-Commit-SHA", result.CommitSHA())

	linesParam := req.URL.Query().Get("lines")
	lineNumbers := req.URL.Query().Get("line_numbers") == "true"

	if linesParam != "" || lineNumbers {
		filter, filterErr := service.NewLineFilter(linesParam)
		if filterErr != nil {
			middleware.WriteError(w, req, fmt.Errorf("%s: %w", filterErr.Error(), middleware.ErrValidation), r.logger)
			return
		}

		var output []byte
		if lineNumbers {
			output = filter.ApplyWithLineNumbers(result.Content())
		} else {
			output = filter.Apply(result.Content())
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(output)
		return
	}

	contentType := http.DetectContentType(result.Content())
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Content())
}
