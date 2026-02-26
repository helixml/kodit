package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/helixml/kodit"
	apimiddleware "github.com/helixml/kodit/infrastructure/api/middleware"
	v1 "github.com/helixml/kodit/infrastructure/api/v1"
	mcpinternal "github.com/helixml/kodit/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// APIServer provides an HTTP API backed by a kodit Client.
type APIServer struct {
	client       *kodit.Client
	apiKeys      []string
	server       *Server
	router       chi.Router
	routerCalled bool
	logger       *slog.Logger
}

// NewAPIServer creates a new APIServer wired to the given kodit Client.
// apiKeys configures write-protection: mutating endpoints (POST, PUT, PATCH,
// DELETE) on /api/v1/repositories and /api/v1/enrichments require a valid key.
// Read-only endpoints, search, MCP, and docs remain open.
func NewAPIServer(client *kodit.Client, apiKeys []string) *APIServer {
	return &APIServer{
		client:  client,
		apiKeys: apiKeys,
		logger:  client.Logger(),
	}
}

// Router returns the chi router for customization before starting.
// Call this first, add custom middleware with router.Use(), then call MountRoutes().
// If not called, ListenAndServe creates a default router with all standard routes.
func (a *APIServer) Router() chi.Router {
	if a.router != nil {
		return a.router
	}

	a.router = chi.NewRouter()
	a.routerCalled = true
	return a.router
}

// MountRoutes wires up all v1 API routes on the router.
// Call this after adding any custom middleware via Router().Use().
func (a *APIServer) MountRoutes() {
	if a.router == nil {
		a.Router()
	}
	a.mountRoutes(a.router)
}

// mountRoutes wires up all v1 API routes on the given router.
func (a *APIServer) mountRoutes(router chi.Router) {
	c := a.client

	reposRouter := v1.NewRepositoriesRouter(c)
	queueRouter := v1.NewQueueRouter(c)
	enrichmentsRouter := v1.NewEnrichmentsRouter(c)
	searchRouter := v1.NewSearchRouter(c)

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(chimiddleware.Timeout(60 * time.Second))

		// Open routes — search is a read-only POST, queue is GET-only.
		r.Mount("/search", searchRouter.Routes())
		r.Mount("/queue", queueRouter.Routes())

		// Write-protected routes — mutating methods require a valid API key.
		r.Group(func(r chi.Router) {
			r.Use(apimiddleware.WriteProtectAuth(a.apiKeys))
			r.Mount("/repositories", reposRouter.Routes())
			r.Mount("/enrichments", enrichmentsRouter.Routes())
		})
	})

	// MCP (Model Context Protocol) endpoint — no timeout middleware.
	// MCP uses streaming responses and manages its own session state via
	// response headers, which is incompatible with chi's Timeout middleware
	// that wraps the ResponseWriter.
	mcpSrv := mcpinternal.NewServer(c.Search, c.Repositories, c.Commits, c.Enrichments, c.Blobs, "1.0.0", a.logger)
	httpHandler := server.NewStreamableHTTPServer(mcpSrv.MCPServer())
	router.Mount("/mcp", httpHandler)
}

// DocsRouter returns a router for Swagger UI and OpenAPI spec.
func (a *APIServer) DocsRouter(specURL string) *DocsRouter {
	return NewDocsRouter(specURL)
}

// ListenAndServe starts the HTTP server on the given address.
func (a *APIServer) ListenAndServe(addr string) error {
	server := NewServer(addr, a.logger)
	a.server = &server

	if a.routerCalled && a.router != nil {
		server.Router().Mount("/", a.router)
	} else {
		a.mountRoutes(server.Router())
	}

	return server.Start()
}

// Shutdown gracefully shuts down the server.
func (a *APIServer) Shutdown(ctx context.Context) error {
	if a.server == nil {
		return nil
	}
	return a.server.Shutdown(ctx)
}

// Handler returns the router as an http.Handler for use with custom servers.
func (a *APIServer) Handler() http.Handler {
	if a.router == nil {
		a.Router()
		a.MountRoutes()
	}
	return a.router
}
