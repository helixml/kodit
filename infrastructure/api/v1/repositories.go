// Package v1 provides the v1 API routes.
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
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
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
//	@Param			page		query	int	false	"Page number (default: 1)"
//	@Param			page_size	query	int	false	"Results per page (default: 20, max: 100)"
//	@Success		200	{object}	dto.RepositoryListResponse
//	@Failure		500	{object}	map[string]string
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

	response := dto.RepositoryListResponse{
		Data:  reposToDTO(repos),
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

	middleware.WriteJSON(w, http.StatusOK, dto.RepositoryDetailsResponse{
		Data:          repoToDTO(repo),
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
//	@Success		201		{object}	dto.RepositoryResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
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
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "remote_uri is required",
		})
		return
	}

	source, err := r.client.Repositories.Add(ctx, &service.RepositoryAddParams{
		URL: body.Data.Attributes.RemoteURI,
	})
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, dto.RepositoryResponse{Data: repoToDTO(source.Repo())})
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
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits [get]
func (r *RepositoriesRouter) ListCommits(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Check repository exists
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

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
//	@Failure		404			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/files [get]
func (r *RepositoriesRouter) ListCommitFiles(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check repository exists
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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
//	@Failure		404					{object}	map[string]string
//	@Failure		500					{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/enrichments [get]
func (r *RepositoriesRouter) ListCommitEnrichments(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check repository exists
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIListResponse{
		Data:  data,
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
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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

	if err := r.client.Enrichments.Delete(ctx, &service.EnrichmentDeleteParams{CommitSHA: commitSHA}); err != nil {
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
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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

	if err := r.client.Enrichments.Delete(ctx, &service.EnrichmentDeleteParams{ID: &enrichmentID}); err != nil {
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
//	@Failure		401			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/commits/{commit_sha}/snippets [get]
func (r *RepositoriesRouter) ListCommitSnippets(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commitSHA := chi.URLParam(req, "commit_sha")

	// Check repository exists
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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

	data := make([]dto.SnippetData, 0, len(enrichments))
	for _, e := range enrichments {
		createdAt := e.CreatedAt()
		updatedAt := e.UpdatedAt()

		data = append(data, dto.SnippetData{
			Type: "snippet",
			ID:   fmt.Sprintf("%d", e.ID()),
			Attributes: dto.SnippetAttributes{
				CreatedAt:   &createdAt,
				UpdatedAt:   &updatedAt,
				DerivesFrom: []dto.GitFileSchema{},
				Content: dto.SnippetContentSchema{
					Value:    e.Content(),
					Language: e.Language(),
				},
				Enrichments:    []dto.EnrichmentSchema{},
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
//	@Success		410			{object}	map[string]string
//	@Security		APIKeyAuth
//	@Deprecated
//	@Router			/repositories/{id}/commits/{commit_sha}/embeddings [get]
func (r *RepositoriesRouter) ListCommitEmbeddingsDeprecated(w http.ResponseWriter, _ *http.Request) {
	middleware.WriteJSON(w, http.StatusGone, map[string]string{
		"error":   "this endpoint has been removed",
		"message": "embeddings are an internal detail of snippets and enrichments, not a commit-level resource",
	})
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
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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

	if err := r.client.Repositories.Rescan(ctx, &service.RescanParams{RepositoryID: id, CommitSHA: commitSHA}); err != nil {
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
//	@Failure		404					{object}	map[string]string
//	@Failure		500					{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/enrichments [get]
func (r *RepositoriesRouter) ListRepositoryEnrichments(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Check repository exists
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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

	middleware.WriteJSON(w, http.StatusOK, dto.EnrichmentJSONAPIListResponse{
		Data:  data,
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
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
//	@Failure		404	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/repositories/{id}/tags [get]
func (r *RepositoriesRouter) ListTags(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination := ParsePagination(req)

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Check repository exists
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
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
			ID:   fmt.Sprintf("%d", tag.ID()),
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
	_, err = r.client.Repositories.Get(ctx, repository.WithID(id))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	tag, err := r.client.Tags.Get(ctx, repository.WithID(tagID), repository.WithRepoID(id))
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

func reposToDTO(repos []repository.Repository) []dto.RepositoryData {
	result := make([]dto.RepositoryData, len(repos))
	for i, repo := range repos {
		result[i] = repoToDTO(repo)
	}
	return result
}

func repoToDTO(repo repository.Repository) dto.RepositoryData {
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
