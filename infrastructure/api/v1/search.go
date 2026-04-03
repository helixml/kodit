package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp/syntax"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/sourcelocation"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

// SearchRouter handles search API endpoints.
type SearchRouter struct {
	client *kodit.Client
	logger zerolog.Logger
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
	router.Get("/semantic", r.SemanticSearch)
	router.Get("/keyword", r.KeywordSearch)
	router.Get("/ls", r.Ls)
	router.Get("/grep", r.Grep)

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
//	@Router			/search [post]
func (r *SearchRouter) Search(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var body dto.SearchRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		if err == io.EOF {
			middleware.WriteError(w, req, fmt.Errorf("request body is required: %w", middleware.ErrValidation), r.logger)
			return
		}
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

	response, err := r.resolveAndBuildResponse(ctx, result.Enrichments(), result.OriginalScores())
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, response)
}

// SemanticSearch handles GET /api/v1/search/semantic.
//
//	@Summary		Semantic code search
//	@Description	Search code snippets using semantic similarity
//	@Tags			search
//	@Produce		json
//	@Param			query			query		string	true	"Natural language search query"
//	@Param			language		query		string	false	"Language filter (e.g. py, go)"
//	@Param			repository_id	query		int		false	"Repository ID filter"
//	@Param			limit			query		int		false	"Maximum results (default 10)"
//	@Success		200				{object}	dto.SearchResponse
//	@Failure		400				{object}	middleware.JSONAPIErrorResponse
//	@Failure		500				{object}	middleware.JSONAPIErrorResponse
//	@Router			/search/semantic [get]
func (r *SearchRouter) SemanticSearch(w http.ResponseWriter, req *http.Request) {
	query := strings.TrimSpace(req.URL.Query().Get("query"))
	if query == "" {
		middleware.WriteError(w, req, fmt.Errorf("query is required: %w", middleware.ErrValidation), r.logger)
		return
	}

	var language *string
	if l := req.URL.Query().Get("language"); l != "" {
		language = &l
	}

	var repositoryID *int64
	if s := req.URL.Query().Get("repository_id"); s != "" {
		parsed, parseErr := strconv.ParseInt(s, 10, 64)
		if parseErr != nil || parsed < 1 {
			middleware.WriteError(w, req, fmt.Errorf("invalid repository_id: %w", middleware.ErrValidation), r.logger)
			return
		}
		repositoryID = &parsed
	}

	var limit *int
	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		parsed, parseErr := strconv.Atoi(limitStr)
		if parseErr != nil || parsed < 1 {
			middleware.WriteError(w, req, fmt.Errorf("limit must be at least 1: %w", middleware.ErrValidation), r.logger)
			return
		}
		limit = &parsed
	}

	r.handleSemanticSearch(w, req, query, language, repositoryID, limit)
}

func (r *SearchRouter) handleSemanticSearch(w http.ResponseWriter, req *http.Request, query string, languagePtr *string, repositoryID *int64, limitPtr *int) {
	ctx := req.Context()

	limit := 10
	if limitPtr != nil && *limitPtr > 0 {
		limit = *limitPtr
	}

	if repositoryID != nil {
		if _, err := r.client.Repositories.Get(ctx, repository.WithID(*repositoryID)); err != nil {
			middleware.WriteError(w, req, err, r.logger)
			return
		}
	}

	language := normalizeExtension(languagePtr)

	var filterOpts []search.FiltersOption
	if language != "" {
		filterOpts = append(filterOpts, search.WithLanguages([]string{language}))
	}
	if repositoryID != nil {
		filterOpts = append(filterOpts, search.WithSourceRepos([]int64{*repositoryID}))
	}
	filters := search.NewFilters(filterOpts...)

	enrichments, scores, err := r.client.Search.SearchCodeWithScores(ctx, query, limit, filters)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if len(enrichments) > limit {
		enrichments = enrichments[:limit]
	}

	scoreMap := enrichmentScoreMap(enrichments, scores)
	response, err := r.resolveAndBuildResponse(ctx, enrichments, scoreMap)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, response)
}

// KeywordSearch handles GET /api/v1/search/keyword.
//
//	@Summary		Keyword code search
//	@Description	Search code snippets using BM25 keyword matching
//	@Tags			search
//	@Produce		json
//	@Param			keywords		query		string	true	"Search keywords"
//	@Param			language		query		string	false	"Language filter (e.g. py, go)"
//	@Param			repository_id	query		int		false	"Repository ID filter"
//	@Param			limit			query		int		false	"Maximum results (default 10)"
//	@Success		200				{object}	dto.SearchResponse
//	@Failure		400				{object}	middleware.JSONAPIErrorResponse
//	@Failure		500				{object}	middleware.JSONAPIErrorResponse
//	@Router			/search/keyword [get]
func (r *SearchRouter) KeywordSearch(w http.ResponseWriter, req *http.Request) {
	keywords := req.URL.Query().Get("keywords")
	if keywords == "" {
		middleware.WriteError(w, req, fmt.Errorf("keywords is required: %w", middleware.ErrValidation), r.logger)
		return
	}

	var language *string
	if l := req.URL.Query().Get("language"); l != "" {
		language = &l
	}

	var repositoryID *int64
	if s := req.URL.Query().Get("repository_id"); s != "" {
		parsed, parseErr := strconv.ParseInt(s, 10, 64)
		if parseErr != nil || parsed < 1 {
			middleware.WriteError(w, req, fmt.Errorf("invalid repository_id: %w", middleware.ErrValidation), r.logger)
			return
		}
		repositoryID = &parsed
	}

	var limit *int
	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		parsed, parseErr := strconv.Atoi(limitStr)
		if parseErr != nil || parsed < 1 {
			middleware.WriteError(w, req, fmt.Errorf("limit must be at least 1: %w", middleware.ErrValidation), r.logger)
			return
		}
		limit = &parsed
	}

	r.handleKeywordSearch(w, req, keywords, language, repositoryID, limit)
}

func (r *SearchRouter) handleKeywordSearch(w http.ResponseWriter, req *http.Request, keywords string, languagePtr *string, repositoryID *int64, limitPtr *int) {
	ctx := req.Context()

	limit := 10
	if limitPtr != nil && *limitPtr > 0 {
		limit = *limitPtr
	}

	if repositoryID != nil {
		if _, err := r.client.Repositories.Get(ctx, repository.WithID(*repositoryID)); err != nil {
			middleware.WriteError(w, req, err, r.logger)
			return
		}
	}

	language := normalizeExtension(languagePtr)

	var filterOpts []search.FiltersOption
	if language != "" {
		filterOpts = append(filterOpts, search.WithLanguages([]string{language}))
	}
	if repositoryID != nil {
		filterOpts = append(filterOpts, search.WithSourceRepos([]int64{*repositoryID}))
	}
	filters := search.NewFilters(filterOpts...)

	enrichments, scores, err := r.client.Search.SearchKeywordsWithScores(ctx, keywords, limit, filters)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	if language != "" {
		enrichments = filterByLanguage(enrichments, language)
	}

	if len(enrichments) > limit {
		enrichments = enrichments[:limit]
	}

	scoreMap := enrichmentScoreMap(enrichments, scores)
	response, err := r.resolveAndBuildResponse(ctx, enrichments, scoreMap)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, response)
}

// Ls handles GET /api/v1/search/ls.
//
//	@Summary		List files matching a glob pattern
//	@Description	Returns files from a repository working copy matching a glob pattern, with file:// URIs
//	@Tags			search
//	@Accept			json
//	@Produce		json
//	@Param			repository_id	query		int		true	"Repository ID"
//	@Param			pattern			query		string	true	"Glob/pathspec pattern (e.g. **/*.go, src/*.py)"
//	@Param			page			query		int		false	"Page number (default: 1)"
//	@Param			page_size		query		int		false	"Results per page (default: 20, max: 100)"
//	@Success		200				{object}	dto.LsResponse
//	@Failure		400				{object}	middleware.JSONAPIErrorResponse
//	@Failure		404				{object}	middleware.JSONAPIErrorResponse
//	@Failure		500				{object}	middleware.JSONAPIErrorResponse
//	@Router			/search/ls [get]
func (r *SearchRouter) Ls(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	pagination, err := ParsePagination(req)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	repoIDStr := req.URL.Query().Get("repository_id")
	if repoIDStr == "" {
		middleware.WriteError(w, req, fmt.Errorf("repository_id query parameter is required: %w", middleware.ErrValidation), r.logger)
		return
	}
	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, fmt.Errorf("invalid repository_id: %w", middleware.ErrValidation), r.logger)
		return
	}

	pattern := req.URL.Query().Get("pattern")
	if pattern == "" {
		middleware.WriteError(w, req, fmt.Errorf("pattern query parameter is required: %w", middleware.ErrValidation), r.logger)
		return
	}

	if _, err := r.client.Repositories.Get(ctx, repository.WithID(repoID)); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commits, err := r.client.Commits.Find(ctx, repository.WithRepoID(repoID), repository.WithOrderDesc("date"), repository.WithLimit(1))
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}
	commitSHA := ""
	if len(commits) > 0 {
		commitSHA = commits[0].SHA()
	}

	var files []service.FileEntry
	if commitSHA != "" {
		files, err = r.client.Blobs.ListFilesForCommit(ctx, repoID, commitSHA, pattern)
	} else {
		files, err = r.client.Blobs.ListFiles(ctx, repoID, pattern)
	}
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	// Paginate.
	total := int64(len(files))
	start := pagination.Offset()
	end := start + pagination.Limit()
	if start > len(files) {
		start = len(files)
	}
	if end > len(files) {
		end = len(files)
	}
	page := files[start:end]

	repoIDFmt := strconv.FormatInt(repoID, 10)
	data := make([]dto.LsFileData, 0, len(page))
	for _, f := range page {
		id := f.Path
		if f.BlobSHA != "" {
			id = f.BlobSHA
		}
		link := fmt.Sprintf("/api/v1/repositories/%s/blob/%s/%s", repoIDFmt, commitSHA, f.Path)
		data = append(data, dto.LsFileData{
			Type: "file",
			ID:   id,
			Attributes: dto.LsFileAttributes{
				Path: f.Path,
				Size: f.Size,
			},
			Links: dto.LsFileLinks{
				Self: link,
			},
		})
	}

	middleware.WriteJSON(w, http.StatusOK, dto.LsResponse{
		Data:  data,
		Meta:  PaginationMeta(pagination, total),
		Links: PaginationLinks(req, pagination, total),
	})
}

// Grep handles GET /api/v1/search/grep.
//
//	@Summary		Search file contents with grep
//	@Description	Search file contents in a repository using git grep with regex patterns
//	@Tags			search
//	@Produce		json
//	@Param			repository_id	query		int		true	"Repository ID"
//	@Param			pattern			query		string	true	"Regex pattern to search for"
//	@Param			glob			query		string	false	"File path filter (e.g. *.go, src/**/*.ts)"
//	@Param			limit			query		int		false	"Maximum number of file results (default 10, max 200)"
//	@Success		200				{object}	dto.GrepResponse
//	@Failure		400				{object}	middleware.JSONAPIErrorResponse
//	@Failure		404				{object}	middleware.JSONAPIErrorResponse
//	@Failure		500				{object}	middleware.JSONAPIErrorResponse
//	@Router			/search/grep [get]
func (r *SearchRouter) Grep(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	repoIDStr := req.URL.Query().Get("repository_id")
	if repoIDStr == "" {
		middleware.WriteError(w, req, fmt.Errorf("repository_id query parameter is required: %w", middleware.ErrValidation), r.logger)
		return
	}
	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, fmt.Errorf("invalid repository_id: %w", middleware.ErrValidation), r.logger)
		return
	}

	pattern := req.URL.Query().Get("pattern")
	if pattern == "" {
		middleware.WriteError(w, req, fmt.Errorf("pattern query parameter is required: %w", middleware.ErrValidation), r.logger)
		return
	}

	if _, err := syntax.Parse(pattern, syntax.Perl); err != nil {
		middleware.WriteError(w, req, fmt.Errorf("invalid regex pattern: %w", middleware.ErrValidation), r.logger)
		return
	}

	glob := req.URL.Query().Get("glob")
	if strings.Contains(glob, "..") {
		middleware.WriteError(w, req, fmt.Errorf("glob must not contain path traversal: %w", middleware.ErrValidation), r.logger)
		return
	}

	limit := 10
	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		parsed, parseErr := strconv.Atoi(limitStr)
		if parseErr != nil || parsed < 1 {
			middleware.WriteError(w, req, fmt.Errorf("limit must be at least 1: %w", middleware.ErrValidation), r.logger)
			return
		}
		limit = parsed
	}
	if limit > 200 {
		limit = 200
	}

	if _, err := r.client.Repositories.Get(ctx, repository.WithID(repoID)); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	results, err := r.client.Grep.Search(ctx, repoID, pattern, glob, limit)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	repoIDFmt := strconv.FormatInt(repoID, 10)
	response := dto.GrepResponse{Data: make([]dto.GrepFileSchema, 0, len(results))}
	for _, result := range results {
		matches := make([]dto.GrepMatchSchema, 0, len(result.Matches))
		for _, m := range result.Matches {
			matches = append(matches, dto.GrepMatchSchema{
				Line:    m.Line,
				Content: m.Content,
			})
		}
		response.Data = append(response.Data, dto.GrepFileSchema{
			Path:     result.Path,
			Language: result.Language,
			Matches:  matches,
			Links: &dto.GrepFileLinks{
				File: fmt.Sprintf("/api/v1/repositories/%s/blob/%s/%s", repoIDFmt, result.CommitSHA, result.Path),
			},
		})
	}

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
			opts = append(opts, search.WithLanguages(f.Languages))
		}
		if len(f.Authors) > 0 {
			opts = append(opts, search.WithAuthors(f.Authors))
		}
		if f.StartDate != nil {
			opts = append(opts, search.WithCreatedAfter(*f.StartDate))
		}
		if f.EndDate != nil {
			opts = append(opts, search.WithCreatedBefore(*f.EndDate))
		}
		if len(f.Sources) > 0 {
			ids := make([]int64, 0, len(f.Sources))
			for _, s := range f.Sources {
				id, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return search.MultiRequest{}, fmt.Errorf("invalid source repository ID %q: %w", s, middleware.ErrValidation)
				}
				ids = append(ids, id)
			}
			opts = append(opts, search.WithSourceRepos(ids))
		}
		if len(f.FilePatterns) > 0 {
			opts = append(opts, search.WithFilePaths(f.FilePatterns))
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

// resolveAndBuildResponse resolves enrichment metadata (related enrichments,
// source files, line ranges, commits, repos) and builds a SearchResponse.
func (r *SearchRouter) resolveAndBuildResponse(
	ctx context.Context,
	enrichments []enrichment.Enrichment,
	originalScores map[string][]float64,
) (dto.SearchResponse, error) {
	ids := make([]int64, len(enrichments))
	for i, e := range enrichments {
		ids[i] = e.ID()
	}

	related, err := r.client.Enrichments.RelatedEnrichments(ctx, ids)
	if err != nil {
		r.logger.Warn().Err(err).Msg("failed to fetch related enrichments")
		related = map[string][]enrichment.Enrichment{}
	}

	fileMap, err := sourceFileMap(ctx, r.client, ids)
	if err != nil {
		r.logger.Warn().Err(err).Msg("failed to fetch source files")
		fileMap = map[string][]repository.File{}
	}

	lineRanges, err := r.client.Enrichments.SourceLocations(ctx, ids)
	if err != nil {
		r.logger.Warn().Err(err).Msg("failed to fetch line ranges")
		lineRanges = map[string]sourcelocation.SourceLocation{}
	}

	commits, err := r.commitMap(ctx, fileMap)
	if err != nil {
		r.logger.Warn().Err(err).Msg("failed to fetch commits")
		commits = map[string]repository.Commit{}
	}

	repos, err := r.repositoryMap(ctx, commits)
	if err != nil {
		r.logger.Warn().Err(err).Msg("failed to fetch repositories")
		repos = map[int64]repository.Repository{}
	}

	data := make([]dto.SnippetData, len(enrichments))
	for i, e := range enrichments {
		idStr := strconv.FormatInt(e.ID(), 10)
		lr, hasLR := lineRanges[idStr]
		var lrPtr *sourcelocation.SourceLocation
		if hasLR {
			lrPtr = &lr
		}
		data[i] = enrichmentToSearchResult(e, originalScores[idStr], related[idStr], fileMap[idStr], lrPtr, commits, repos)
	}

	return dto.SearchResponse{Data: data}, nil
}

// normalizeExtension strips a leading dot so that ".py" and "py" compare equal.
// Returns empty string for nil input.
func normalizeExtension(ext *string) string {
	if ext == nil {
		return ""
	}
	return strings.TrimPrefix(*ext, ".")
}

// filterByLanguage returns only enrichments whose language matches.
func filterByLanguage(enrichments []enrichment.Enrichment, language string) []enrichment.Enrichment {
	filtered := make([]enrichment.Enrichment, 0, len(enrichments))
	for _, e := range enrichments {
		if strings.TrimPrefix(e.Language(), ".") == language {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// enrichmentScoreMap converts a flat score map (id→score) into the per-enrichment
// multi-score map (id→[]float64) expected by the response builder.
func enrichmentScoreMap(enrichments []enrichment.Enrichment, scores map[string]float64) map[string][]float64 {
	result := make(map[string][]float64, len(enrichments))
	for _, e := range enrichments {
		idStr := strconv.FormatInt(e.ID(), 10)
		if s, ok := scores[idStr]; ok {
			result[idStr] = []float64{s}
		}
	}
	return result
}

func enrichmentToSearchResult(
	e enrichment.Enrichment,
	scores []float64,
	related []enrichment.Enrichment,
	files []repository.File,
	lr *sourcelocation.SourceLocation,
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

	links := snippetLinks(files, commits, repos)

	content := dto.SnippetContentSchema{
		Value:    e.Content(),
		Language: e.Language(),
	}
	if lr != nil {
		startLine := lr.StartLine()
		endLine := lr.EndLine()
		content.StartLine = &startLine
		content.EndLine = &endLine
	}

	return dto.SnippetData{
		Type: string(e.Subtype()),
		ID:   strconv.FormatInt(e.ID(), 10),
		Attributes: dto.SnippetAttributes{
			CreatedAt:      &createdAt,
			UpdatedAt:      &updatedAt,
			Content:        content,
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
		File:       fmt.Sprintf("/api/v1/repositories/%s/blob/%s/%s", repoID, commit.SHA(), file.Path()),
	}
}
