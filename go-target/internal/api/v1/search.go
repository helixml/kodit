package v1

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/internal/api/middleware"
	"github.com/helixml/kodit/internal/api/v1/dto"
	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/search"
)

// SearchRouter handles search API endpoints.
type SearchRouter struct {
	searchService search.Service
	logger        *slog.Logger
}

// NewSearchRouter creates a new SearchRouter.
func NewSearchRouter(searchService search.Service, logger *slog.Logger) *SearchRouter {
	if logger == nil {
		logger = slog.Default()
	}
	return &SearchRouter{
		searchService: searchService,
		logger:        logger,
	}
}

// Routes returns the chi router for search endpoints.
func (r *SearchRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Post("/", r.Search)
	router.Get("/", r.SearchGet)

	return router
}

// Search handles POST /api/v1/search.
func (r *SearchRouter) Search(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var body dto.SearchRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	searchReq := buildSearchRequest(body)
	result, err := r.searchService.Search(ctx, searchReq)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	response := buildSearchResponse(body.Query, result)
	middleware.WriteJSON(w, http.StatusOK, response)
}

// SearchGet handles GET /api/v1/search?q=query.
func (r *SearchRouter) SearchGet(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	query := req.URL.Query().Get("q")
	if query == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "q parameter is required",
		})
		return
	}

	topK := 10
	if topKStr := req.URL.Query().Get("top_k"); topKStr != "" {
		if parsed, err := parseInt(topKStr); err == nil && parsed > 0 {
			topK = parsed
		}
	}

	searchReq := domain.NewMultiSearchRequest(topK, query, query, nil, domain.SnippetSearchFilters{})
	result, err := r.searchService.Search(ctx, searchReq)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	response := buildSearchResponse(query, result)
	middleware.WriteJSON(w, http.StatusOK, response)
}

func buildSearchRequest(body dto.SearchRequest) domain.MultiSearchRequest {
	topK := body.TopK
	if topK <= 0 {
		topK = 10
	}

	textQuery := body.TextQuery
	codeQuery := body.CodeQuery

	// If neither specified, use the general query for both
	if textQuery == "" && codeQuery == "" {
		textQuery = body.Query
		codeQuery = body.Query
	}

	// Build filters
	var opts []domain.SnippetSearchFiltersOption
	if body.Language != "" {
		opts = append(opts, domain.WithLanguage(body.Language))
	}
	if body.Author != "" {
		opts = append(opts, domain.WithAuthor(body.Author))
	}
	if body.SourceRepo != "" {
		opts = append(opts, domain.WithSourceRepo(body.SourceRepo))
	}
	if body.FilePath != "" {
		opts = append(opts, domain.WithFilePath(body.FilePath))
	}
	if len(body.CommitSHAs) > 0 {
		opts = append(opts, domain.WithCommitSHAs(body.CommitSHAs))
	}

	filters := domain.NewSnippetSearchFilters(opts...)

	return domain.NewMultiSearchRequest(topK, textQuery, codeQuery, body.Keywords, filters)
}

func buildSearchResponse(query string, result search.MultiSearchResult) dto.SearchResponse {
	snippets := result.Snippets()
	scores := result.FusedScores()

	results := make([]dto.SearchResultResponse, len(snippets))
	for i, snippet := range snippets {
		results[i] = snippetToSearchResult(snippet, scores[snippet.SHA()])
	}

	return dto.SearchResponse{
		Results:    results,
		TotalCount: len(results),
		Query:      query,
	}
}

func snippetToSearchResult(snippet indexing.Snippet, score float64) dto.SearchResultResponse {
	var filePath string
	derivesFrom := snippet.DerivesFrom()
	if len(derivesFrom) > 0 {
		filePath = derivesFrom[0].Path()
	}

	return dto.SearchResultResponse{
		SnippetSHA: snippet.SHA(),
		Content:    snippet.Content(),
		Extension:  snippet.Extension(),
		Score:      score,
		FilePath:   filePath,
	}
}

func parseInt(s string) (int, error) {
	var result int
	err := json.Unmarshal([]byte(s), &result)
	return result, err
}
