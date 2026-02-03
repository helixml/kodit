package v1

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

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
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/search [post]
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

	response := buildSearchResponse(result)
	middleware.WriteJSON(w, http.StatusOK, response)
}

func buildSearchRequest(body dto.SearchRequest) domain.MultiSearchRequest {
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

	// If neither specified, use keywords combined
	if textQuery == "" && codeQuery == "" && len(attrs.Keywords) > 0 {
		combined := strings.Join(attrs.Keywords, " ")
		textQuery = combined
		codeQuery = combined
	}

	// Build filters
	var opts []domain.SnippetSearchFiltersOption
	if attrs.Filters != nil {
		f := attrs.Filters
		if len(f.Languages) > 0 {
			opts = append(opts, domain.WithLanguage(f.Languages[0]))
		}
		if len(f.Authors) > 0 {
			opts = append(opts, domain.WithAuthor(f.Authors[0]))
		}
		if f.StartDate != nil {
			opts = append(opts, domain.WithCreatedAfter(*f.StartDate))
		}
		if f.EndDate != nil {
			opts = append(opts, domain.WithCreatedBefore(*f.EndDate))
		}
		if len(f.Sources) > 0 {
			opts = append(opts, domain.WithSourceRepo(f.Sources[0]))
		}
		if len(f.FilePatterns) > 0 {
			opts = append(opts, domain.WithFilePath(f.FilePatterns[0]))
		}
		if len(f.EnrichmentTypes) > 0 {
			opts = append(opts, domain.WithEnrichmentTypes(f.EnrichmentTypes))
		}
		if len(f.EnrichmentSubtypes) > 0 {
			opts = append(opts, domain.WithEnrichmentSubtypes(f.EnrichmentSubtypes))
		}
		if len(f.CommitSHA) > 0 {
			opts = append(opts, domain.WithCommitSHAs(f.CommitSHA))
		}
	}

	filters := domain.NewSnippetSearchFilters(opts...)

	return domain.NewMultiSearchRequest(topK, textQuery, codeQuery, attrs.Keywords, filters)
}

func buildSearchResponse(result search.MultiSearchResult) dto.SearchResponse {
	snippets := result.Snippets()
	scores := result.FusedScores()

	data := make([]dto.SnippetData, len(snippets))
	for i, snippet := range snippets {
		data[i] = snippetToSearchResult(snippet, scores[snippet.SHA()])
	}

	return dto.SearchResponse{
		Data: data,
	}
}

func snippetToSearchResult(snippet indexing.Snippet, score float64) dto.SnippetData {
	derivesFrom := snippet.DerivesFrom()
	derivesFromSchemas := make([]dto.GitFileSchema, len(derivesFrom))
	for i, f := range derivesFrom {
		derivesFromSchemas[i] = dto.GitFileSchema{
			BlobSHA:  f.BlobSHA(),
			Path:     f.Path(),
			MimeType: f.MimeType(),
			Size:     f.Size(),
		}
	}

	createdAt := snippet.CreatedAt()
	updatedAt := snippet.UpdatedAt()

	return dto.SnippetData{
		Type: "snippet",
		ID:   snippet.SHA(),
		Attributes: dto.SnippetAttributes{
			CreatedAt:   &createdAt,
			UpdatedAt:   &updatedAt,
			DerivesFrom: derivesFromSchemas,
			Content: dto.SnippetContentSchema{
				Value:    snippet.Content(),
				Language: snippet.Extension(),
			},
			Enrichments:    []dto.EnrichmentSchema{},
			OriginalScores: []float64{score},
		},
	}
}
