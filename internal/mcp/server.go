// Package mcp provides Model Context Protocol server functionality.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Searcher provides code search operations for MCP tools.
type Searcher interface {
	Search(ctx context.Context, request search.MultiRequest) (service.MultiSearchResult, error)
}

// EnrichmentLookup provides enrichment retrieval by ID for MCP tools.
type EnrichmentLookup interface {
	Get(ctx context.Context, options ...repository.Option) (enrichment.Enrichment, error)
}

// RepositoryLister provides repository listing for MCP tools.
type RepositoryLister interface {
	Find(ctx context.Context, options ...repository.Option) ([]repository.Repository, error)
}

// CommitFinder provides commit querying for MCP tools.
type CommitFinder interface {
	Find(ctx context.Context, options ...repository.Option) ([]repository.Commit, error)
}

// EnrichmentQuery provides enrichment listing for MCP tools.
type EnrichmentQuery interface {
	List(ctx context.Context, params *service.EnrichmentListParams) ([]enrichment.Enrichment, error)
}

// Server wraps the MCP server with kodit-specific tools.
type Server struct {
	mcpServer       *server.MCPServer
	searchService   Searcher
	repositories    RepositoryLister
	commits         CommitFinder
	enrichmentQuery EnrichmentQuery
	version         string
	logger          *slog.Logger
}

const instructions = "This server provides access to code knowledge through multiple " +
	"complementary tools:\n\n" +
	"**Discovery workflow:**\n" +
	"1. Use list_repositories() first to see available repositories\n" +
	"2. Then use repository-specific tools with the discovered repo URLs\n\n" +
	"**Available tools:**\n" +
	"- list_repositories() - Discover available repositories (call this first!)\n" +
	"- get_architecture_docs() - High-level structure and design\n" +
	"- get_api_docs() - Interface documentation\n" +
	"- get_commit_description() - Recent changes and context\n" +
	"- get_database_schema() - Data models\n" +
	"- get_cookbook() - Complete usage examples\n" +
	"- search() - Find specific code snippets matching keywords\n\n" +
	"Choose the most appropriate tool based on what information you need. " +
	"Often starting with architecture or API docs provides better context than " +
	"immediately searching for code snippets."

// NewServer creates a new MCP server with the given dependencies.
func NewServer(
	searchService Searcher,
	repositories RepositoryLister,
	commits CommitFinder,
	enrichmentQuery EnrichmentQuery,
	version string,
	logger *slog.Logger,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		searchService:   searchService,
		repositories:    repositories,
		commits:         commits,
		enrichmentQuery: enrichmentQuery,
		version:         version,
		logger:          logger,
	}

	mcpServer := server.NewMCPServer(
		"kodit",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithInstructions(instructions),
	)

	s.registerTools(mcpServer)

	s.mcpServer = mcpServer
	return s
}

// registerTools registers all kodit tools with the MCP server.
func (s *Server) registerTools(mcpServer *server.MCPServer) {
	mcpServer.AddTool(mcp.NewTool("search",
		mcp.WithDescription("Search the codebase using hybrid BM25 and vector search"),
		mcp.WithString("user_intent",
			mcp.Required(),
			mcp.Description("Natural language description of what you are looking for"),
		),
		mcp.WithArray("keywords",
			mcp.Required(),
			mcp.Description("Keywords to search for"),
			mcp.WithStringItems(),
		),
		mcp.WithArray("related_file_paths",
			mcp.Required(),
			mcp.Description("Paths of files related to the search context"),
			mcp.WithStringItems(),
		),
		mcp.WithArray("related_file_contents",
			mcp.Required(),
			mcp.Description("Contents of files related to the search context"),
			mcp.WithStringItems(),
		),
		mcp.WithString("language",
			mcp.Description("Filter by programming language"),
		),
		mcp.WithString("author",
			mcp.Description("Filter by commit author"),
		),
		mcp.WithString("created_after",
			mcp.Description("Filter for items created after this date (ISO 8601)"),
		),
		mcp.WithString("created_before",
			mcp.Description("Filter for items created before this date (ISO 8601)"),
		),
		mcp.WithString("source_repo",
			mcp.Description("Filter by source repository URL"),
		),
		mcp.WithArray("enrichment_subtypes",
			mcp.Description("Filter by enrichment subtypes"),
			mcp.WithStringItems(),
		),
	), s.handleSearch)

	mcpServer.AddTool(mcp.NewTool("get_version",
		mcp.WithDescription("Get the kodit server version"),
	), s.handleGetVersion)

	mcpServer.AddTool(mcp.NewTool("list_repositories",
		mcp.WithDescription("List all repositories tracked by kodit"),
	), s.handleListRepositories)

	mcpServer.AddTool(mcp.NewTool("get_architecture_docs",
		mcp.WithDescription("Get high-level architecture documentation for a repository"),
		mcp.WithString("repo_url",
			mcp.Required(),
			mcp.Description("The remote URL of the repository"),
		),
		mcp.WithString("commit_sha",
			mcp.Description("The commit SHA to get docs for (defaults to latest)"),
		),
	), s.handleGetArchitectureDocs)

	mcpServer.AddTool(mcp.NewTool("get_api_docs",
		mcp.WithDescription("Get API documentation for a repository"),
		mcp.WithString("repo_url",
			mcp.Required(),
			mcp.Description("The remote URL of the repository"),
		),
		mcp.WithString("commit_sha",
			mcp.Description("The commit SHA to get docs for (defaults to latest)"),
		),
	), s.handleGetAPIDocs)

	mcpServer.AddTool(mcp.NewTool("get_commit_description",
		mcp.WithDescription("Get commit description for a repository"),
		mcp.WithString("repo_url",
			mcp.Required(),
			mcp.Description("The remote URL of the repository"),
		),
		mcp.WithString("commit_sha",
			mcp.Description("The commit SHA to get docs for (defaults to latest)"),
		),
	), s.handleGetCommitDescription)

	mcpServer.AddTool(mcp.NewTool("get_database_schema",
		mcp.WithDescription("Get database schema documentation for a repository"),
		mcp.WithString("repo_url",
			mcp.Required(),
			mcp.Description("The remote URL of the repository"),
		),
		mcp.WithString("commit_sha",
			mcp.Description("The commit SHA to get docs for (defaults to latest)"),
		),
	), s.handleGetDatabaseSchema)

	mcpServer.AddTool(mcp.NewTool("get_cookbook",
		mcp.WithDescription("Get cookbook with usage examples for a repository"),
		mcp.WithString("repo_url",
			mcp.Required(),
			mcp.Description("The remote URL of the repository"),
		),
		mcp.WithString("commit_sha",
			mcp.Description("The commit SHA to get docs for (defaults to latest)"),
		),
	), s.handleGetCookbook)
}

// handleSearch handles the search tool invocation.
func (s *Server) handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	userIntent, err := request.RequireString("user_intent")
	if err != nil {
		return mcp.NewToolResultError("user_intent is required"), nil
	}

	keywords, err := request.RequireStringSlice("keywords")
	if err != nil {
		return mcp.NewToolResultError("keywords is required"), nil
	}

	relatedFilePaths := request.GetStringSlice("related_file_paths", nil)
	relatedFileContents := request.GetStringSlice("related_file_contents", nil)

	s.logger.Info("search",
		slog.String("user_intent", userIntent),
		slog.Any("keywords", keywords),
		slog.Any("related_file_paths", relatedFilePaths),
	)

	codeQuery := strings.Join(relatedFileContents, "\n")

	var opts []search.FiltersOption
	if lang := request.GetString("language", ""); lang != "" {
		opts = append(opts, search.WithLanguage(lang))
	}
	if author := request.GetString("author", ""); author != "" {
		opts = append(opts, search.WithAuthor(author))
	}
	if after := request.GetString("created_after", ""); after != "" {
		if t, parseErr := time.Parse(time.RFC3339, after); parseErr == nil {
			opts = append(opts, search.WithCreatedAfter(t))
		}
	}
	if before := request.GetString("created_before", ""); before != "" {
		if t, parseErr := time.Parse(time.RFC3339, before); parseErr == nil {
			opts = append(opts, search.WithCreatedBefore(t))
		}
	}
	if repo := request.GetString("source_repo", ""); repo != "" {
		opts = append(opts, search.WithSourceRepo(repo))
	}
	if subtypes := request.GetStringSlice("enrichment_subtypes", nil); len(subtypes) > 0 {
		opts = append(opts, search.WithEnrichmentSubtypes(subtypes))
	}

	filters := search.NewFilters(opts...)
	searchReq := search.NewMultiRequest(10, userIntent, codeQuery, keywords, filters)

	result, err := s.searchService.Search(ctx, searchReq)
	if err != nil {
		s.logger.Error("search failed", slog.Any("error", err))
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	enrichments := result.Enrichments()
	scores := result.FusedScores()

	type searchResult struct {
		ID       string  `json:"id"`
		Content  string  `json:"content"`
		Language string  `json:"language"`
		Score    float64 `json:"score"`
	}

	results := make([]searchResult, len(enrichments))
	for i, e := range enrichments {
		idStr := strconv.FormatInt(e.ID(), 10)
		results[i] = searchResult{
			ID:       idStr,
			Content:  e.Content(),
			Language: e.Language(),
			Score:    scores[idStr],
		}
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleGetVersion returns the kodit server version.
func (s *Server) handleGetVersion(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText(s.version), nil
}

// handleListRepositories lists all tracked repositories.
func (s *Server) handleListRepositories(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repos, err := s.repositories.Find(ctx)
	if err != nil {
		s.logger.Error("failed to list repositories", slog.Any("error", err))
		return mcp.NewToolResultError(fmt.Sprintf("failed to list repositories: %v", err)), nil
	}

	if len(repos) == 0 {
		return mcp.NewToolResultText("No repositories found."), nil
	}

	var b strings.Builder
	for _, repo := range repos {
		fmt.Fprintf(&b, "- %s", repo.RemoteURL())

		if repo.HasTrackingConfig() {
			tc := repo.TrackingConfig()
			switch {
			case tc.IsBranch():
				fmt.Fprintf(&b, " (tracking branch: %s)", tc.Branch())
			case tc.IsTag():
				fmt.Fprintf(&b, " (tracking tag: %s)", tc.Tag())
			case tc.IsCommit():
				fmt.Fprintf(&b, " (tracking commit: %s)", tc.Commit())
			}
		}

		commits, commitErr := s.commits.Find(ctx,
			repository.WithRepoID(repo.ID()),
			repository.WithOrderDesc("committed_at"),
			repository.WithLimit(1),
		)
		if commitErr == nil && len(commits) > 0 {
			fmt.Fprintf(&b, " [latest: %s]", commits[0].ShortSHA())
		}

		b.WriteString("\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func (s *Server) handleGetArchitectureDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	typ := enrichment.TypeArchitecture
	subtype := enrichment.SubtypePhysical
	return s.handleEnrichmentDocs(ctx, request, typ, subtype)
}

func (s *Server) handleGetAPIDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	typ := enrichment.TypeUsage
	subtype := enrichment.SubtypeAPIDocs
	return s.handleEnrichmentDocs(ctx, request, typ, subtype)
}

func (s *Server) handleGetCommitDescription(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	typ := enrichment.TypeHistory
	subtype := enrichment.SubtypeCommitDescription
	return s.handleEnrichmentDocs(ctx, request, typ, subtype)
}

func (s *Server) handleGetDatabaseSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	typ := enrichment.TypeArchitecture
	subtype := enrichment.SubtypeDatabaseSchema
	return s.handleEnrichmentDocs(ctx, request, typ, subtype)
}

func (s *Server) handleGetCookbook(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	typ := enrichment.TypeUsage
	subtype := enrichment.SubtypeCookbook
	return s.handleEnrichmentDocs(ctx, request, typ, subtype)
}

// handleEnrichmentDocs is the shared handler for enrichment doc tools.
func (s *Server) handleEnrichmentDocs(
	ctx context.Context,
	request mcp.CallToolRequest,
	typ enrichment.Type,
	subtype enrichment.Subtype,
) (*mcp.CallToolResult, error) {
	repoURL, err := request.RequireString("repo_url")
	if err != nil {
		return mcp.NewToolResultError("repo_url is required"), nil
	}

	repos, err := s.repositories.Find(ctx, repository.WithRemoteURL(repoURL))
	if err != nil {
		s.logger.Error("failed to find repository", slog.String("repo_url", repoURL), slog.Any("error", err))
		return mcp.NewToolResultError(fmt.Sprintf("failed to find repository: %v", err)), nil
	}
	if len(repos) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("repository not found: %s", repoURL)), nil
	}

	commitSHA := request.GetString("commit_sha", "")
	if commitSHA == "" {
		commits, commitErr := s.commits.Find(ctx,
			repository.WithRepoID(repos[0].ID()),
			repository.WithOrderDesc("committed_at"),
			repository.WithLimit(1),
		)
		if commitErr != nil {
			s.logger.Error("failed to find latest commit", slog.Any("error", commitErr))
			return mcp.NewToolResultError(fmt.Sprintf("failed to find latest commit: %v", commitErr)), nil
		}
		if len(commits) == 0 {
			return mcp.NewToolResultError("no commits found for repository"), nil
		}
		commitSHA = commits[0].SHA()
	}

	enrichments, err := s.enrichmentQuery.List(ctx, &service.EnrichmentListParams{
		CommitSHA: commitSHA,
		Type:      &typ,
		Subtype:   &subtype,
	})
	if err != nil {
		s.logger.Error("failed to list enrichments", slog.Any("error", err))
		return mcp.NewToolResultError(fmt.Sprintf("failed to get enrichments: %v", err)), nil
	}

	if len(enrichments) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No %s/%s docs found for this commit.", typ, subtype)), nil
	}

	parts := make([]string, len(enrichments))
	for i, e := range enrichments {
		parts[i] = e.Content()
	}
	return mcp.NewToolResultText(strings.Join(parts, "\n\n")), nil
}

// MCPServer returns the underlying MCP server for stdio serving.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// ServeStdio runs the MCP server on stdio.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}
