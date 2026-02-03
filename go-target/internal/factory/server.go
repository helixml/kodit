// Package factory provides dependency injection and service construction.
package factory

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/internal/api"
	apimiddleware "github.com/helixml/kodit/internal/api/middleware"
	v1 "github.com/helixml/kodit/internal/api/v1"
	"github.com/helixml/kodit/internal/enrichment"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/provider"
	"github.com/helixml/kodit/internal/queue"
	"github.com/helixml/kodit/internal/repository"
	"github.com/helixml/kodit/internal/search"
	"gorm.io/gorm"
)

// ServerDependencies contains all dependencies needed to construct a server.
type ServerDependencies struct {
	DB              *gorm.DB
	Logger          *slog.Logger
	TextGenerator   provider.TextGenerator
	Embedder        provider.Embedder
	GitAdapter      git.Adapter
	DataDir         string
	ServerAddr      string
	APIKey          string // Optional API key for authentication
}

// ServerFactory constructs and wires all services for the server.
type ServerFactory struct {
	deps ServerDependencies

	// Repositories
	repoRepo         git.RepoRepository
	commitRepo       git.CommitRepository
	branchRepo       git.BranchRepository
	tagRepo          git.TagRepository
	fileRepo         git.FileRepository
	snippetRepo      indexing.SnippetRepository
	bm25Repo         indexing.BM25Repository
	vectorRepo       indexing.VectorSearchRepository
	enrichmentRepo   enrichment.EnrichmentRepository
	associationRepo  enrichment.AssociationRepository
	taskRepo         queue.TaskRepository
	taskStatusRepo   queue.TaskStatusRepository

	// Services
	queueService     *queue.Service
	repoQueryService *repository.QueryService
	repoSyncService  *repository.SyncService
	searchService    search.Service
}

// NewServerFactory creates a new ServerFactory with the given dependencies.
func NewServerFactory(deps ServerDependencies) *ServerFactory {
	return &ServerFactory{deps: deps}
}

// Build constructs all services and returns a configured API server.
func (f *ServerFactory) Build() (api.Server, error) {
	// Create repositories (implementations would be injected in real usage)
	// For now, we'll leave these as nil and let the routers handle missing deps

	// Create services
	if f.taskRepo != nil {
		f.queueService = queue.NewService(f.taskRepo, f.deps.Logger)
	}

	if f.repoRepo != nil && f.commitRepo != nil && f.branchRepo != nil && f.tagRepo != nil {
		f.repoQueryService = repository.NewQueryService(f.repoRepo, f.commitRepo, f.branchRepo, f.tagRepo)
	}

	if f.repoRepo != nil && f.queueService != nil {
		f.repoSyncService = repository.NewSyncService(f.repoRepo, f.queueService, f.deps.Logger)
	}

	if f.bm25Repo != nil && f.vectorRepo != nil && f.snippetRepo != nil && f.enrichmentRepo != nil {
		f.searchService = search.NewService(
			f.bm25Repo,
			f.vectorRepo,
			f.snippetRepo,
			f.enrichmentRepo,
			f.deps.Logger,
		)
	}

	// Create server
	server := api.NewServer(f.deps.ServerAddr, f.deps.Logger)
	router := server.Router()

	// Apply middleware
	router.Use(apimiddleware.Logging(f.deps.Logger))
	router.Use(apimiddleware.CorrelationID)

	// Apply authentication middleware if API key is configured
	authConfig := apimiddleware.NewAuthConfig(f.deps.APIKey)

	// Register API routes
	router.Route("/api/v1", func(r chi.Router) {
		// Apply auth middleware to all /api/v1 routes
		r.Use(apimiddleware.APIKey(authConfig))
		if f.repoQueryService != nil && f.repoSyncService != nil {
			reposRouter := v1.NewRepositoriesRouter(f.repoQueryService, f.repoSyncService, f.deps.Logger)
			r.Mount("/repositories", reposRouter.Routes())
		}

		if f.repoQueryService != nil {
			commitsRouter := v1.NewCommitsRouter(f.repoQueryService, f.deps.Logger)
			r.Mount("/commits", commitsRouter.Routes())
		}

		searchRouter := v1.NewSearchRouter(f.searchService, f.deps.Logger)
		r.Mount("/search", searchRouter.Routes())

		if f.enrichmentRepo != nil {
			enrichmentsRouter := v1.NewEnrichmentsRouter(f.enrichmentRepo, f.deps.Logger)
			r.Mount("/enrichments", enrichmentsRouter.Routes())
		}

		if f.taskRepo != nil && f.taskStatusRepo != nil {
			queueRouter := v1.NewQueueRouter(f.taskRepo, f.taskStatusRepo, f.deps.Logger)
			r.Mount("/queue", queueRouter.Routes())
		}
	})

	// Health check (matches Python API /healthz endpoint)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return server, nil
}

// WithRepoRepository sets the repository repository.
func (f *ServerFactory) WithRepoRepository(repo git.RepoRepository) *ServerFactory {
	f.repoRepo = repo
	return f
}

// WithCommitRepository sets the commit repository.
func (f *ServerFactory) WithCommitRepository(repo git.CommitRepository) *ServerFactory {
	f.commitRepo = repo
	return f
}

// WithBranchRepository sets the branch repository.
func (f *ServerFactory) WithBranchRepository(repo git.BranchRepository) *ServerFactory {
	f.branchRepo = repo
	return f
}

// WithTagRepository sets the tag repository.
func (f *ServerFactory) WithTagRepository(repo git.TagRepository) *ServerFactory {
	f.tagRepo = repo
	return f
}

// WithFileRepository sets the file repository.
func (f *ServerFactory) WithFileRepository(repo git.FileRepository) *ServerFactory {
	f.fileRepo = repo
	return f
}

// WithSnippetRepository sets the snippet repository.
func (f *ServerFactory) WithSnippetRepository(repo indexing.SnippetRepository) *ServerFactory {
	f.snippetRepo = repo
	return f
}

// WithBM25Repository sets the BM25 repository.
func (f *ServerFactory) WithBM25Repository(repo indexing.BM25Repository) *ServerFactory {
	f.bm25Repo = repo
	return f
}

// WithVectorRepository sets the vector search repository.
func (f *ServerFactory) WithVectorRepository(repo indexing.VectorSearchRepository) *ServerFactory {
	f.vectorRepo = repo
	return f
}

// WithEnrichmentRepository sets the enrichment repository.
func (f *ServerFactory) WithEnrichmentRepository(repo enrichment.EnrichmentRepository) *ServerFactory {
	f.enrichmentRepo = repo
	return f
}

// WithAssociationRepository sets the association repository.
func (f *ServerFactory) WithAssociationRepository(repo enrichment.AssociationRepository) *ServerFactory {
	f.associationRepo = repo
	return f
}

// WithTaskRepository sets the task repository.
func (f *ServerFactory) WithTaskRepository(repo queue.TaskRepository) *ServerFactory {
	f.taskRepo = repo
	return f
}

// WithTaskStatusRepository sets the task status repository.
func (f *ServerFactory) WithTaskStatusRepository(repo queue.TaskStatusRepository) *ServerFactory {
	f.taskStatusRepo = repo
	return f
}
