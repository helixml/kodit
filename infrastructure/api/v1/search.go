package v1

import (
	"context"
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
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

// SearchRouter handles search API endpoints.
type SearchRouter struct {
	client *kodit.Client
	logger *slog.Logger
}

// NewSearchRouter creates a new SearchRouter.
func NewSearchRouter(client *kodit.Client) *SearchRouter {
	return &SearchRouter{
		client: client,
		logger: client.Logger(),
	}
}

// Routes returns the chi router for search endpoints.
func (r *SearchRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Post("/", r.Search)

	return router
}

// Search handles POST /api/v1/search.
//
//	@Summary		Search code
//	@Description	Hybrid search across code snippets and enrichments
//	@Tags			search
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.SearchRequest	true	"Search request"
//	@Success		200		{object}	dto.SearchResponse
//	@Failure		400		{object}	middleware.JSONAPIErrorResponse
//	@Failure		500		{object}	middleware.JSONAPIErrorResponse
//	@Security		APIKeyAuth
//	@Router			/search [post]
func (r *SearchRouter) Search(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var body dto.SearchRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	searchReq, err := buildSearchRequest(body)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}
	result, err := r.client.Search.Search(ctx, searchReq)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Fetch related enrichments (e.g., summaries) for the search results
	enrichments := result.Enrichments()
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

	commits, err := r.commitMap(ctx, fileMap)
	if err != nil {
		r.logger.Warn("failed to fetch commits", "error", err)
		commits = map[string]repository.Commit{}
	}

	repos, err := r.repositoryMap(ctx, commits)
	if err != nil {
		r.logger.Warn("failed to fetch repositories", "error", err)
		repos = map[int64]repository.Repository{}
	}

	response := buildSearchResponse(result, related, fileMap, commits, repos)
	middleware.WriteJSON(w, http.StatusOK, response)
}

func buildSearchRequest(body dto.SearchRequest) (search.MultiRequest, error) {
	attrs := body.Data.Attributes

	// Determine limit (default 10)
	topK := 10
	if attrs.Limit != nil && *attrs.Limit > 0 {
		topK = *attrs.Limit
	}

	// Determine text and code queries
	var textQuery, codeQuery string
	if attrs.Text != nil {
		textQuery = *attrs.Text
	}
	if attrs.Code != nil {
		codeQuery = *attrs.Code
	}

	// Build filters
	var opts []search.FiltersOption
	if attrs.Filters != nil {
		f := attrs.Filters
		if len(f.Languages) > 0 {
			opts = append(opts, search.WithLanguage(f.Languages[0]))
		}
		if len(f.Authors) > 0 {
			opts = append(opts, search.WithAuthor(f.Authors[0]))
		}
		if f.StartDate != nil {
			opts = append(opts, search.WithCreatedAfter(*f.StartDate))
		}
		if f.EndDate != nil {
			opts = append(opts, search.WithCreatedBefore(*f.EndDate))
		}
		if len(f.Sources) > 0 {
			repoID, err := strconv.ParseInt(f.Sources[0], 10, 64)
			if err != nil {
				return search.MultiRequest{}, fmt.Errorf("invalid source repository ID %q: %w", f.Sources[0], err)
			}
			opts = append(opts, search.WithSourceRepo(repoID))
		}
		if len(f.FilePatterns) > 0 {
			opts = append(opts, search.WithFilePath(f.FilePatterns[0]))
		}
		if len(f.EnrichmentTypes) > 0 {
			opts = append(opts, search.WithEnrichmentTypes(f.EnrichmentTypes))
		}
		if len(f.EnrichmentSubtypes) > 0 {
			opts = append(opts, search.WithEnrichmentSubtypes(f.EnrichmentSubtypes))
		}
		if len(f.CommitSHA) > 0 {
			opts = append(opts, search.WithCommitSHAs(f.CommitSHA))
		}
	}

	filters := search.NewFilters(opts...)

	return search.NewMultiRequest(topK, textQuery, codeQuery, attrs.Keywords, filters), nil
}

func buildSearchResponse(
	result service.MultiSearchResult,
	related map[string][]enrichment.Enrichment,
	fileMap map[string][]repository.File,
	commits map[string]repository.Commit,
	repos map[int64]repository.Repository,
) dto.SearchResponse {
	enrichments := result.Enrichments()
	originalScores := result.OriginalScores()

	data := make([]dto.SnippetData, len(enrichments))
	for i, e := range enrichments {
		idStr := strconv.FormatInt(e.ID(), 10)
		data[i] = enrichmentToSearchResult(e, originalScores[idStr], related[idStr], fileMap[idStr], commits, repos)
	}

	return dto.SearchResponse{
		Data: data,
	}
}

func enrichmentToSearchResult(
	e enrichment.Enrichment,
	scores []float64,
	related []enrichment.Enrichment,
	files []repository.File,
	commits map[string]repository.Commit,
	repos map[int64]repository.Repository,
) dto.SnippetData {
	createdAt := e.CreatedAt()
	updatedAt := e.UpdatedAt()

	enrichmentSchemas := make([]dto.EnrichmentSchema, len(related))
	for i, r := range related {
		enrichmentSchemas[i] = dto.EnrichmentSchema{
			Type:    string(r.Subtype()),
			Content: r.Content(),
		}
	}

	derivesFrom := make([]dto.GitFileSchema, len(files))
	for i, f := range files {
		derivesFrom[i] = dto.GitFileSchema{
			BlobSHA:  f.BlobSHA(),
			Path:     f.Path(),
			MimeType: f.MimeType(),
			Size:     f.Size(),
		}
	}

	links := snippetLinks(files, commits, repos)

	return dto.SnippetData{
		Type: string(e.Subtype()),
		ID:   strconv.FormatInt(e.ID(), 10),
		Attributes: dto.SnippetAttributes{
			CreatedAt:   &createdAt,
			UpdatedAt:   &updatedAt,
			DerivesFrom: derivesFrom,
			Content: dto.SnippetContentSchema{
				Value:    e.Content(),
				Language: e.Language(),
			},
			Enrichments:    enrichmentSchemas,
			OriginalScores: scores,
		},
		Links: links,
	}
}

// commitMap returns commits keyed by SHA for the given file map.
func (r *SearchRouter) commitMap(ctx context.Context, fileMap map[string][]repository.File) (map[string]repository.Commit, error) {
	shas := uniqueCommitSHAs(fileMap)
	if len(shas) == 0 {
		return map[string]repository.Commit{}, nil
	}

	commits, err := r.client.Commits.Find(ctx, repository.WithCommitSHAIn(shas))
	if err != nil {
		return nil, err
	}

	result := make(map[string]repository.Commit, len(commits))
	for _, c := range commits {
		result[c.SHA()] = c
	}
	return result, nil
}

// repositoryMap returns repositories keyed by ID for the given commit map.
func (r *SearchRouter) repositoryMap(ctx context.Context, commits map[string]repository.Commit) (map[int64]repository.Repository, error) {
	ids := uniqueRepoIDs(commits)
	if len(ids) == 0 {
		return map[int64]repository.Repository{}, nil
	}

	repos, err := r.client.Repositories.Find(ctx, repository.WithIDIn(ids))
	if err != nil {
		return nil, err
	}

	result := make(map[int64]repository.Repository, len(repos))
	for _, repo := range repos {
		result[repo.ID()] = repo
	}
	return result, nil
}

func uniqueCommitSHAs(fileMap map[string][]repository.File) []string {
	seen := map[string]struct{}{}
	for _, files := range fileMap {
		for _, f := range files {
			seen[f.CommitSHA()] = struct{}{}
		}
	}

	shas := make([]string, 0, len(seen))
	for sha := range seen {
		shas = append(shas, sha)
	}
	return shas
}

func uniqueRepoIDs(commits map[string]repository.Commit) []int64 {
	seen := map[int64]struct{}{}
	for _, c := range commits {
		seen[c.RepoID()] = struct{}{}
	}

	ids := make([]int64, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func snippetLinks(files []repository.File, commits map[string]repository.Commit, repos map[int64]repository.Repository) *dto.SnippetLinks {
	if len(files) == 0 {
		return nil
	}

	file := files[0]
	commit, ok := commits[file.CommitSHA()]
	if !ok {
		return nil
	}

	repo, ok := repos[commit.RepoID()]
	if !ok {
		return nil
	}

	repoID := strconv.FormatInt(repo.ID(), 10)
	return &dto.SnippetLinks{
		Repository: fmt.Sprintf("/api/v1/repositories/%s", repoID),
		Commit:     fmt.Sprintf("/api/v1/repositories/%s/commits/%s", repoID, commit.SHA()),
		File:       fmt.Sprintf("/api/v1/repositories/%s/commits/%s/files/%s", repoID, commit.SHA(), file.BlobSHA()),
	}
}

