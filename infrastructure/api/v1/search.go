package v1

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
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
	result, err := r.client.Search.Search(ctx, searchReq)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	response := buildSearchResponse(result)
	middleware.WriteJSON(w, http.StatusOK, response)
}

func buildSearchRequest(body dto.SearchRequest) search.MultiRequest {
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
			opts = append(opts, search.WithSourceRepo(f.Sources[0]))
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

	return search.NewMultiRequest(topK, textQuery, codeQuery, attrs.Keywords, filters)
}

func buildSearchResponse(result service.MultiSearchResult) dto.SearchResponse {
	enrichments := result.Enrichments()
	scores := result.FusedScores()

	data := make([]dto.SnippetData, len(enrichments))
	for i, e := range enrichments {
		data[i] = enrichmentToSearchResult(e, scores[strconv.FormatInt(e.ID(), 10)])
	}

	return dto.SearchResponse{
		Data: data,
	}
}

func enrichmentToSearchResult(e enrichment.Enrichment, score float64) dto.SnippetData {
	createdAt := e.CreatedAt()
	updatedAt := e.UpdatedAt()

	return dto.SnippetData{
		Type: "snippet",
		ID:   strconv.FormatInt(e.ID(), 10),
		Attributes: dto.SnippetAttributes{
			CreatedAt:   &createdAt,
			UpdatedAt:   &updatedAt,
			DerivesFrom: []dto.GitFileSchema{},
			Content: dto.SnippetContentSchema{
				Value:    e.Content(),
				Language: e.Language(),
			},
			Enrichments:    []dto.EnrichmentSchema{},
			OriginalScores: []float64{score},
		},
	}
}
