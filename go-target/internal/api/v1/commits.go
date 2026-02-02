package v1

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/internal/api/middleware"
	"github.com/helixml/kodit/internal/api/v1/dto"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/repository"
)

// CommitsRouter handles commit API endpoints.
type CommitsRouter struct {
	queryService *repository.QueryService
	logger       *slog.Logger
}

// NewCommitsRouter creates a new CommitsRouter.
func NewCommitsRouter(queryService *repository.QueryService, logger *slog.Logger) *CommitsRouter {
	if logger == nil {
		logger = slog.Default()
	}
	return &CommitsRouter{
		queryService: queryService,
		logger:       logger,
	}
}

// Routes returns the chi router for commit endpoints.
func (r *CommitsRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/", r.ListByRepository)

	return router
}

// ListByRepository handles GET /api/v1/commits?repository_id=X.
func (r *CommitsRouter) ListByRepository(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	repoIDStr := req.URL.Query().Get("repository_id")
	if repoIDStr == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "repository_id is required",
		})
		return
	}

	repoID, err := strconv.ParseInt(repoIDStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	commits, err := r.queryService.CommitsForRepository(ctx, repoID)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	response := dto.CommitListResponse{
		Data:       commitsToDTO(commits),
		TotalCount: len(commits),
	}

	middleware.WriteJSON(w, http.StatusOK, response)
}

func commitsToDTO(commits []git.Commit) []dto.CommitResponse {
	result := make([]dto.CommitResponse, len(commits))
	for i, commit := range commits {
		result[i] = commitToDTO(commit)
	}
	return result
}

func commitToDTO(commit git.Commit) dto.CommitResponse {
	return dto.CommitResponse{
		SHA:          commit.SHA(),
		RepositoryID: commit.RepoID(),
		Message:      commit.Message(),
		AuthorName:   commit.Author().Name(),
		AuthorEmail:  commit.Author().Email(),
		CommittedAt:  commit.CommittedAt(),
		CreatedAt:    commit.CreatedAt(),
		UpdatedAt:    commit.CreatedAt(), // Commits are immutable, use CreatedAt
	}
}
