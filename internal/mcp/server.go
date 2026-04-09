// Package mcp provides Model Context Protocol server functionality.
package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/sourcelocation"
	"github.com/helixml/kodit/domain/wiki"
	"github.com/helixml/kodit/infrastructure/extraction"
	"github.com/helixml/kodit/infrastructure/rasterization"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

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

// FileContentReader provides raw file content from Git repositories.
type FileContentReader interface {
	Content(ctx context.Context, repoID int64, blobName, filePath string) (service.BlobContent, error)
}

// DiskPathResolver resolves a blob reference to an on-disk file path.
type DiskPathResolver interface {
	DiskPath(ctx context.Context, repoID int64, blobName, filePath string) (string, string, error)
}

// SemanticSearcher provides code vector search with scores.
type SemanticSearcher interface {
	SearchCodeWithScores(ctx context.Context, query string, topK int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error)
}

// KeywordSearcher provides BM25 keyword search with scores.
type KeywordSearcher interface {
	SearchKeywordsWithScores(ctx context.Context, query string, limit int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error)
}

// VisualSearcher provides cross-modal visual search with scores.
type VisualSearcher interface {
	SearchVisualWithScores(ctx context.Context, query string, topK int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error)
}

// EnrichmentResolver provides enrichment-to-entity resolution.
type EnrichmentResolver interface {
	SourceFiles(ctx context.Context, enrichmentIDs []int64) (map[string][]int64, error)
	SourceLocations(ctx context.Context, enrichmentIDs []int64) (map[string]sourcelocation.SourceLocation, error)
	RepositoryIDs(ctx context.Context, enrichmentIDs []int64) (map[string]int64, error)
}

// FileLister provides pattern-based file listing from repository working copies.
type FileLister interface {
	ListFiles(ctx context.Context, repoID int64, pattern string) ([]service.FileEntry, error)
}

// FileFinder provides file lookups by options.
type FileFinder interface {
	Find(ctx context.Context, options ...repository.Option) ([]repository.File, error)
}

// Grepper provides git grep search over repositories.
type Grepper interface {
	Search(ctx context.Context, repoID int64, pattern string, pathspec string, maxFiles int) ([]service.GrepResult, error)
}

// Server wraps the MCP server with kodit-specific tools.
type Server struct {
	mcpServer          *server.MCPServer
	repositories       RepositoryLister
	commits            CommitFinder
	enrichmentQuery    EnrichmentQuery
	fileContent        FileContentReader
	diskPathResolver   DiskPathResolver
	rasterizers        *rasterization.Registry
	textRenderers      *extraction.TextRendererRegistry
	semanticSearch     SemanticSearcher
	keywordSearch      KeywordSearcher
	visualSearch       VisualSearcher
	enrichmentResolver EnrichmentResolver
	fileLister         FileLister
	files              FileFinder
	grepper            Grepper
	version            string
	logger             zerolog.Logger
}

const instructions = "This server provides access to code knowledge through multiple " +
	"complementary tools:\n\n" +
	"**Discovery workflow:**\n" +
	"1. Use kodit_repositories() first to see available repositories\n" +
	"2. Then use repository-specific tools with the discovered repo URLs\n\n" +
	"**Available tools:**\n" +
	"- kodit_repositories() - Discover available repositories (call this first!)\n" +
	"- kodit_architecture_docs() - High-level structure and design\n" +
	"- kodit_api_docs() - Interface documentation\n" +
	"- kodit_commit_description() - Recent changes and context\n" +
	"- kodit_database_schema() - Data models\n" +
	"- kodit_cookbook() - Complete usage examples\n" +
	"- kodit_wiki() - Get the table of contents for a repository's wiki\n" +
	"- kodit_wiki_page() - Get the content of a specific wiki page by slug\n" +
	"- kodit_semantic_search() - Find files matching a natural language query (returns resource URIs)\n" +
	"- kodit_keyword_search() - Find files matching keywords using BM25 search (returns resource URIs)\n" +
	"- kodit_visual_search() - Find document pages (PDFs, etc.) matching a text query using visual similarity\n" +
	"- kodit_grep() - Search file contents using git grep with regex patterns (returns resource URIs)\n" +
	"- kodit_ls() - List files matching a glob pattern in a repository\n" +
	"- kodit_read_resource() - Read file content from a resource URI returned by search tools\n\n" +
	"**Reading file content:**\n" +
	"Use kodit_read_resource() with the URI returned by search tools, or the file resource " +
	"template: file://{id}/{blob_name}/{+path}\n" +
	"where id is the repository ID, blob_name is a commit SHA, tag, or branch name, " +
	"and path is the file path within the repository.\n" +
	"Optional query parameters: ?lines=L17-L26,L45 and ?line_numbers=true\n" +
	"For document pages (PDFs, DOCX, XLSX, PPTX): ?page=N&mode=raster returns a rendered image, " +
		"?page=N&mode=text returns extracted text, ?mode=text returns the page count\n\n" +
	"Choose the most appropriate tool based on what information you need. " +
	"Often starting with architecture or API docs provides better context than " +
	"immediately searching for code snippets."

// ServerOption configures optional Server dependencies.
type ServerOption func(*Server)

// WithRasterization enables raster mode for file resource reads.
func WithRasterization(diskPaths DiskPathResolver, rasterizers *rasterization.Registry) ServerOption {
	return func(s *Server) {
		s.diskPathResolver = diskPaths
		s.rasterizers = rasterizers
	}
}

// WithTextRendering enables text mode for file resource reads.
func WithTextRendering(diskPaths DiskPathResolver, textRenderers *extraction.TextRendererRegistry) ServerOption {
	return func(s *Server) {
		s.diskPathResolver = diskPaths
		s.textRenderers = textRenderers
	}
}

// NewServer creates a new MCP server with the given dependencies.
func NewServer(
	repositories RepositoryLister,
	commits CommitFinder,
	enrichmentQuery EnrichmentQuery,
	fileContent FileContentReader,
	semanticSearch SemanticSearcher,
	keywordSearch KeywordSearcher,
	visualSearch VisualSearcher,
	enrichmentResolver EnrichmentResolver,
	fileLister FileLister,
	files FileFinder,
	grepper Grepper,
	version string,
	logger zerolog.Logger,
	opts ...ServerOption,
) *Server {

	s := &Server{
		repositories:       repositories,
		commits:            commits,
		enrichmentQuery:    enrichmentQuery,
		fileContent:        fileContent,
		semanticSearch:     semanticSearch,
		keywordSearch:      keywordSearch,
		visualSearch:       visualSearch,
		enrichmentResolver: enrichmentResolver,
		fileLister:         fileLister,
		files:              files,
		grepper:            grepper,
		version:            version,
		logger:             logger,
	}
	for _, opt := range opts {
		opt(s)
	}

	mcpServer := server.NewMCPServer(
		"kodit",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, false),
		server.WithInstructions(instructions),
	)

	s.registerTools(mcpServer)
	s.registerResources(mcpServer)

	s.mcpServer = mcpServer
	return s
}

// registerTools registers all kodit tools with the MCP server.
// Tool definitions come from the shared tools() list in catalog.go;
// this method pairs each tool name with its handler.
func (s *Server) registerTools(mcpServer *server.MCPServer) {
	handlers := map[string]server.ToolHandlerFunc{
		"kodit_version":            s.handleGetVersion,
		"kodit_repositories":       s.handleListRepositories,
		"kodit_architecture_docs":  s.handleGetArchitectureDocs,
		"kodit_api_docs":           s.handleGetAPIDocs,
		"kodit_commit_description": s.handleGetCommitDescription,
		"kodit_database_schema":    s.handleGetDatabaseSchema,
		"kodit_cookbook":           s.handleGetCookbook,
		"kodit_wiki":               s.handleGetWiki,
		"kodit_wiki_page":          s.handleGetWikiPage,
		"kodit_semantic_search":    s.handleSemanticSearch,
		"kodit_keyword_search":     s.handleKeywordSearch,
		"kodit_visual_search":      s.handleVisualSearch,
		"kodit_grep":               s.handleGrep,
		"kodit_read_resource":      s.handleReadResource,
		"kodit_ls":                 s.handleLs,
	}

	for _, def := range tools() {
		handler, ok := handlers[def.name]
		if !ok {
			continue
		}
		mcpServer.AddTool(mcpTool(def), handler)
	}
}

// handleGetVersion returns the kodit server version.
func (s *Server) handleGetVersion(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText(s.version), nil
}

// handleListRepositories lists all tracked repositories.
func (s *Server) handleListRepositories(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repos, err := s.repositories.Find(ctx)
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("failed to list repositories")
		return mcp.NewToolResultError(fmt.Sprintf("failed to list repositories: %v", err)), nil
	}

	if len(repos) == 0 {
		return mcp.NewToolResultText("No repositories found."), nil
	}

	var b strings.Builder
	for _, repo := range repos {
		fmt.Fprintf(&b, "- %s", repo.UpstreamURL())

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
			repository.WithOrderDesc("date"),
			repository.WithLimit(1),
		)
		if commitErr == nil && len(commits) > 0 {
			fmt.Fprintf(&b, " [latest: %s]", commits[0].ShortSHA())
		}

		b.WriteString("\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

// resolveRepository finds repositories by sanitized URL first, falling back to
// upstream URL. This lets LLMs use the upstream URL they saw in the repository
// listing even when the internal sanitized URL differs.
func (s *Server) resolveRepository(ctx context.Context, repoURL string) ([]repository.Repository, error) {
	repos, err := s.repositories.Find(ctx, repository.WithRemoteURL(repoURL))
	if err != nil {
		return nil, err
	}
	if len(repos) > 0 {
		return repos, nil
	}
	return s.repositories.Find(ctx, repository.WithUpstreamURL(repoURL))
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

// handleGetWiki returns the wiki table of contents for a repository.
func (s *Server) handleGetWiki(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoURL, err := request.RequireString("repo_url")
	if err != nil {
		return mcp.NewToolResultError("repo_url is required"), nil
	}

	repos, err := s.resolveRepository(ctx, repoURL)
	if err != nil {
		s.logger.Error().Str("repo_url", repoURL).Interface("error", err).Msg("failed to find repository")
		return mcp.NewToolResultError(fmt.Sprintf("failed to find repository: %v", err)), nil
	}
	if len(repos) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("repository not found: %s", repoURL)), nil
	}

	commitSHA := request.GetString("commit_sha", "")
	if commitSHA == "" {
		commits, commitErr := s.commits.Find(ctx,
			repository.WithRepoID(repos[0].ID()),
			repository.WithOrderDesc("date"),
			repository.WithLimit(1),
		)
		if commitErr != nil {
			s.logger.Error().Interface("error", commitErr).Msg("failed to find latest commit")
			return mcp.NewToolResultError(fmt.Sprintf("failed to find latest commit: %v", commitErr)), nil
		}
		if len(commits) == 0 {
			return mcp.NewToolResultError("no commits found for repository"), nil
		}
		commitSHA = commits[0].SHA()
	}

	typ := enrichment.TypeUsage
	subtype := enrichment.SubtypeWiki
	enrichments, err := s.enrichmentQuery.List(ctx, &service.EnrichmentListParams{
		CommitSHA: commitSHA,
		Type:      &typ,
		Subtype:   &subtype,
	})
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("failed to list enrichments")
		return mcp.NewToolResultError(fmt.Sprintf("failed to get wiki: %v", err)), nil
	}
	if len(enrichments) == 0 {
		return mcp.NewToolResultText("No wiki found for this commit."), nil
	}

	w, err := wiki.ParseWiki(enrichments[0].Content())
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("failed to parse wiki")
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse wiki: %v", err)), nil
	}

	pathIndex := w.PathIndex()
	var b strings.Builder
	b.WriteString("Wiki pages (use kodit_wiki_page with the slug to read a page):\n\n")
	formatPageTree(&b, w.Pages(), pathIndex, 0)
	return mcp.NewToolResultText(b.String()), nil
}

// handleGetWikiPage returns the markdown content of a specific wiki page.
func (s *Server) handleGetWikiPage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoURL, err := request.RequireString("repo_url")
	if err != nil {
		return mcp.NewToolResultError("repo_url is required"), nil
	}

	pageSlug, err := request.RequireString("page_slug")
	if err != nil {
		return mcp.NewToolResultError("page_slug is required"), nil
	}

	repos, err := s.resolveRepository(ctx, repoURL)
	if err != nil {
		s.logger.Error().Str("repo_url", repoURL).Interface("error", err).Msg("failed to find repository")
		return mcp.NewToolResultError(fmt.Sprintf("failed to find repository: %v", err)), nil
	}
	if len(repos) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("repository not found: %s", repoURL)), nil
	}

	commitSHA := request.GetString("commit_sha", "")
	if commitSHA == "" {
		commits, commitErr := s.commits.Find(ctx,
			repository.WithRepoID(repos[0].ID()),
			repository.WithOrderDesc("date"),
			repository.WithLimit(1),
		)
		if commitErr != nil {
			s.logger.Error().Interface("error", commitErr).Msg("failed to find latest commit")
			return mcp.NewToolResultError(fmt.Sprintf("failed to find latest commit: %v", commitErr)), nil
		}
		if len(commits) == 0 {
			return mcp.NewToolResultError("no commits found for repository"), nil
		}
		commitSHA = commits[0].SHA()
	}

	typ := enrichment.TypeUsage
	subtype := enrichment.SubtypeWiki
	enrichments, err := s.enrichmentQuery.List(ctx, &service.EnrichmentListParams{
		CommitSHA: commitSHA,
		Type:      &typ,
		Subtype:   &subtype,
	})
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("failed to list enrichments")
		return mcp.NewToolResultError(fmt.Sprintf("failed to get wiki: %v", err)), nil
	}
	if len(enrichments) == 0 {
		return mcp.NewToolResultError("no wiki found for this commit"), nil
	}

	w, err := wiki.ParseWiki(enrichments[0].Content())
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("failed to parse wiki")
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse wiki: %v", err)), nil
	}

	page, ok := w.Page(pageSlug)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("wiki page not found: %s", pageSlug)), nil
	}

	return mcp.NewToolResultText(page.Content()), nil
}

// formatPageTree writes a hierarchical listing of wiki pages to the builder.
func formatPageTree(b *strings.Builder, pages []wiki.Page, pathIndex map[string]string, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, p := range pages {
		path := pathIndex[p.Slug()]
		fmt.Fprintf(b, "%s- %s: %s (slug: %s)\n", indent, path, p.Title(), p.Slug())
		formatPageTree(b, p.Children(), pathIndex, depth+1)
	}
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
		return mcp.NewToolResultError(fmt.Sprintf("repo_url is required: %v", err)), nil
	}

	repos, err := s.resolveRepository(ctx, repoURL)
	if err != nil {
		s.logger.Error().Str("repo_url", repoURL).Interface("error", err).Msg("failed to find repository")
		return mcp.NewToolResultError(fmt.Sprintf("failed to find repository: %v", err)), nil
	}
	if len(repos) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("repository not found: %s", repoURL)), nil
	}

	commitSHA := request.GetString("commit_sha", "")
	if commitSHA == "" {
		commits, commitErr := s.commits.Find(ctx,
			repository.WithRepoID(repos[0].ID()),
			repository.WithOrderDesc("date"),
			repository.WithLimit(1),
		)
		if commitErr != nil {
			s.logger.Error().Interface("error", commitErr).Msg("failed to find latest commit")
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
		s.logger.Error().Interface("error", err).Msg("failed to list enrichments")
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

// fileResult holds the resolved file information for a search result.
type fileResult struct {
	URI      string  `json:"uri"`
	Path     string  `json:"path"`
	Language string  `json:"language"`
	Lines    string  `json:"lines"`
	Page     int     `json:"page,omitempty"`
	Score    float64 `json:"score"`
	Preview  string  `json:"preview"`
}

// resolveFileResults converts enrichments and scores into file-based results
// with resource URIs. It resolves source files, line ranges, and repository IDs
// for each enrichment, then builds the file result list. If sourceRepoID > 0,
// results are post-filtered to only include files from that repository.
func (s *Server) resolveFileResults(
	ctx context.Context,
	enrichments []enrichment.Enrichment,
	scores map[string]float64,
	sourceRepoID int64,
) ([]fileResult, error) {
	if len(enrichments) == 0 {
		return nil, nil
	}

	ids := make([]int64, len(enrichments))
	for i, e := range enrichments {
		ids[i] = e.ID()
	}

	sourceFiles, err := s.enrichmentResolver.SourceFiles(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve source files: %w", err)
	}

	lineRanges, err := s.enrichmentResolver.SourceLocations(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve line ranges: %w", err)
	}

	repoIDs, err := s.enrichmentResolver.RepositoryIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve repository IDs: %w", err)
	}

	if sourceRepoID > 0 {
		filtered := enrichments[:0]
		for _, e := range enrichments {
			idStr := strconv.FormatInt(e.ID(), 10)
			if repoIDs[idStr] == sourceRepoID {
				filtered = append(filtered, e)
			}
		}
		enrichments = filtered
		if len(enrichments) == 0 {
			return nil, nil
		}
	}

	var allFileIDs []int64
	for _, fileIDs := range sourceFiles {
		allFileIDs = append(allFileIDs, fileIDs...)
	}

	filesByID := make(map[int64]repository.File)
	if len(allFileIDs) > 0 {
		files, fileErr := s.files.Find(ctx, repository.WithIDIn(allFileIDs))
		if fileErr != nil {
			return nil, fmt.Errorf("fetch files: %w", fileErr)
		}
		for _, f := range files {
			filesByID[f.ID()] = f
		}
	}

	results := make([]fileResult, 0, len(enrichments))
	for _, e := range enrichments {
		idStr := strconv.FormatInt(e.ID(), 10)

		fileIDs := sourceFiles[idStr]
		if len(fileIDs) == 0 {
			continue
		}

		file, ok := filesByID[fileIDs[0]]
		if !ok {
			continue
		}

		repoID := repoIDs[idStr]
		filePath := repoRelativePath(file.Path())
		uri := NewFileURI(repoID, file.CommitSHA(), filePath)

		var lines string
		var page int
		if lr, found := lineRanges[idStr]; found {
			if lr.StartLine() > 0 {
				uri = uri.WithLineRange(lr.StartLine(), lr.EndLine())
				lines = fmt.Sprintf("L%d-L%d", lr.StartLine(), lr.EndLine())
			}
			if lr.Page() > 0 {
				uri = uri.WithPage(lr.Page())
				page = lr.Page()
			}
		}

		preview := e.Content()
		if len(preview) > 200 {
			preview = preview[:200]
		}

		results = append(results, fileResult{
			URI:      uri.String(),
			Path:     filePath,
			Language: e.Language(),
			Lines:    lines,
			Page:     page,
			Score:    scores[idStr],
			Preview:  preview,
		})
	}

	return results, nil
}

// handleSemanticSearch handles the semantic_search tool invocation.
func (s *Server) handleSemanticSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query is required: %v", err)), nil
	}
	if strings.TrimSpace(query) == "" {
		return mcp.NewToolResultError("query must not be empty"), nil
	}

	limit := int(request.GetFloat("limit", 10))
	if limit < 0 {
		return mcp.NewToolResultError("limit must not be negative"), nil
	}
	if limit == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	language := normalizeExtension(request.GetString("language", ""))

	// Resolve source_repo URL to a repository ID for post-filtering.
	var sourceRepoID int64
	if repoURL := request.GetString("source_repo", ""); repoURL != "" {
		repos, repoErr := s.resolveRepository(ctx, repoURL)
		if repoErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolve source_repo: %v", repoErr)), nil
		}
		if len(repos) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		sourceRepoID = repos[0].ID()
	}

	var filterOpts []search.FiltersOption
	if language != "" {
		filterOpts = append(filterOpts, search.WithLanguages([]string{language}))
	}
	if sourceRepoID > 0 {
		filterOpts = append(filterOpts, search.WithSourceRepos([]int64{sourceRepoID}))
	}
	filters := search.NewFilters(filterOpts...)

	enrichments, scores, err := s.semanticSearch.SearchCodeWithScores(ctx, query, limit, filters)
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("semantic search failed")
		return mcp.NewToolResultError(fmt.Sprintf("semantic search failed: %v", err)), nil
	}

	// Cap results to the requested limit.
	if len(enrichments) > limit {
		enrichments = enrichments[:limit]
	}

	if len(enrichments) == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	results, err := s.resolveFileResults(ctx, enrichments, scores, sourceRepoID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleKeywordSearch handles the keyword_search tool invocation.
func (s *Server) handleKeywordSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	keywords, err := request.RequireString("keywords")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("keywords is required: %v", err)), nil
	}
	if strings.TrimSpace(keywords) == "" {
		return mcp.NewToolResultError("keywords must not be empty"), nil
	}

	limit := int(request.GetFloat("limit", 10))
	if limit < 0 {
		return mcp.NewToolResultError("limit must not be negative"), nil
	}
	if limit == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	language := normalizeExtension(request.GetString("language", ""))

	// Resolve source_repo URL to a repository ID for post-filtering.
	var sourceRepoID int64
	if repoURL := request.GetString("source_repo", ""); repoURL != "" {
		repos, repoErr := s.resolveRepository(ctx, repoURL)
		if repoErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolve source_repo: %v", repoErr)), nil
		}
		if len(repos) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		sourceRepoID = repos[0].ID()
	}

	var opts []search.FiltersOption
	if language != "" {
		opts = append(opts, search.WithLanguages([]string{language}))
	}
	filters := search.NewFilters(opts...)

	enrichments, scores, err := s.keywordSearch.SearchKeywordsWithScores(ctx, keywords, limit, filters)
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("keyword search failed")
		return mcp.NewToolResultError(fmt.Sprintf("keyword search failed: %v", err)), nil
	}

	// Post-filter by language if specified (enrichment language may differ from filter).
	if language != "" {
		filtered := make([]enrichment.Enrichment, 0, len(enrichments))
		for _, e := range enrichments {
			if normalizeExtension(e.Language()) == language {
				filtered = append(filtered, e)
			}
		}
		enrichments = filtered
	}

	if len(enrichments) > limit {
		enrichments = enrichments[:limit]
	}

	if len(enrichments) == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	results, err := s.resolveFileResults(ctx, enrichments, scores, sourceRepoID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleVisualSearch handles the visual_search tool invocation.
func (s *Server) handleVisualSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.visualSearch == nil {
		return mcp.NewToolResultError("visual search is not available — vision model not configured"), nil
	}

	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query is required: %v", err)), nil
	}
	if strings.TrimSpace(query) == "" {
		return mcp.NewToolResultError("query must not be empty"), nil
	}

	limit := int(request.GetFloat("limit", 10))
	if limit < 0 {
		return mcp.NewToolResultError("limit must not be negative"), nil
	}
	if limit == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	// Resolve source_repo URL to a repository ID for post-filtering.
	var sourceRepoID int64
	if repoURL := request.GetString("source_repo", ""); repoURL != "" {
		repos, repoErr := s.resolveRepository(ctx, repoURL)
		if repoErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolve source_repo: %v", repoErr)), nil
		}
		if len(repos) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		sourceRepoID = repos[0].ID()
	}

	var filterOpts []search.FiltersOption
	if sourceRepoID > 0 {
		filterOpts = append(filterOpts, search.WithSourceRepos([]int64{sourceRepoID}))
	}
	filters := search.NewFilters(filterOpts...)

	enrichments, scores, err := s.visualSearch.SearchVisualWithScores(ctx, query, limit, filters)
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("visual search failed")
		return mcp.NewToolResultError(fmt.Sprintf("visual search failed: %v", err)), nil
	}

	if len(enrichments) > limit {
		enrichments = enrichments[:limit]
	}

	if len(enrichments) == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	results, err := s.resolveFileResults(ctx, enrichments, scores, sourceRepoID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleGrep handles the grep tool invocation.
func (s *Server) handleGrep(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoURL, err := request.RequireString("repo_url")
	if err != nil {
		return mcp.NewToolResultError("repo_url is required"), nil
	}

	pattern, err := request.RequireString("pattern")
	if err != nil {
		return mcp.NewToolResultError("pattern is required"), nil
	}
	if strings.TrimSpace(pattern) == "" {
		return mcp.NewToolResultError("pattern must not be empty"), nil
	}

	repos, err := s.resolveRepository(ctx, repoURL)
	if err != nil {
		s.logger.Error().Str("repo_url", repoURL).Interface("error", err).Msg("failed to find repository")
		return mcp.NewToolResultError(fmt.Sprintf("failed to find repository: %v", err)), nil
	}
	if len(repos) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("repository not found: %s", repoURL)), nil
	}

	glob := request.GetString("glob", "")
	limit := int(request.GetFloat("limit", 50))
	if limit < 0 {
		return mcp.NewToolResultError("limit must not be negative"), nil
	}
	if limit == 0 {
		return mcp.NewToolResultText("[]"), nil
	}
	if limit > 200 {
		limit = 200
	}

	results, err := s.grepper.Search(ctx, repos[0].ID(), pattern, glob, limit)
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("grep failed")
		return mcp.NewToolResultError(fmt.Sprintf("grep failed: %v", err)), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	fileResults := make([]fileResult, 0, len(results))
	for _, r := range results {
		if len(r.Matches) == 0 {
			continue
		}

		firstLine := r.Matches[0].Line
		lastLine := r.Matches[len(r.Matches)-1].Line

		uri := NewFileURI(r.RepoID, r.CommitSHA, r.Path)
		uri = uri.WithLineRange(firstLine, lastLine)

		var preview strings.Builder
		for i, m := range r.Matches {
			if i >= 5 {
				fmt.Fprintf(&preview, "... and %d more matches", len(r.Matches)-i)
				break
			}
			fmt.Fprintf(&preview, "L%d: %s\n", m.Line, m.Content)
		}

		fileResults = append(fileResults, fileResult{
			URI:      uri.String(),
			Path:     r.Path,
			Language: r.Language,
			Lines:    fmt.Sprintf("L%d-L%d", firstLine, lastLine),
			Score:    0,
			Preview:  strings.TrimSpace(preview.String()),
		})
	}

	jsonBytes, err := json.Marshal(fileResults)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// lsResult holds the resolved file information for an ls match.
type lsResult struct {
	URI  string `json:"uri"`
	Size int64  `json:"size"`
}

// handleLs handles the ls tool invocation.
func (s *Server) handleLs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repoURL, err := request.RequireString("repo_url")
	if err != nil {
		return mcp.NewToolResultError("repo_url is required"), nil
	}

	pattern, err := request.RequireString("pattern")
	if err != nil {
		return mcp.NewToolResultError("pattern is required"), nil
	}
	if strings.TrimSpace(pattern) == "" {
		return mcp.NewToolResultError("pattern must not be empty"), nil
	}

	repos, err := s.resolveRepository(ctx, repoURL)
	if err != nil {
		s.logger.Error().Str("repo_url", repoURL).Interface("error", err).Msg("failed to find repository")
		return mcp.NewToolResultError(fmt.Sprintf("failed to find repository: %v", err)), nil
	}
	if len(repos) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("repository not found: %s", repoURL)), nil
	}

	commits, err := s.commits.Find(ctx,
		repository.WithRepoID(repos[0].ID()),
		repository.WithOrderDesc("date"),
		repository.WithLimit(1),
	)
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("failed to find latest commit")
		return mcp.NewToolResultError(fmt.Sprintf("failed to find latest commit: %v", err)), nil
	}
	if len(commits) == 0 {
		return mcp.NewToolResultError("no commits found for repository"), nil
	}
	commitSHA := commits[0].SHA()

	files, err := s.fileLister.ListFiles(ctx, repos[0].ID(), pattern)
	if err != nil {
		s.logger.Error().Interface("error", err).Msg("list files failed")
		return mcp.NewToolResultError(fmt.Sprintf("ls failed: %v", err)), nil
	}

	results := make([]lsResult, 0, len(files))
	for _, f := range files {
		uri := NewFileURI(repos[0].ID(), commitSHA, f.Path)
		results = append(results, lsResult{
			URI:  uri.String(),
			Size: f.Size,
		})
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("[]"), nil
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleReadResource handles the read_resource tool invocation.
// It delegates to the file resource handler, allowing clients that do not
// support MCP resources to read file content through a tool call.
func (s *Server) handleReadResource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uri, err := request.RequireString("uri")
	if err != nil {
		return mcp.NewToolResultError("uri is required"), nil
	}

	resourceRequest := mcp.ReadResourceRequest{}
	resourceRequest.Params.URI = uri

	contents, err := s.handleReadFile(ctx, resourceRequest)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid file URI: %v", err)), nil
	}

	if len(contents) == 0 {
		return mcp.NewToolResultText(""), nil
	}

	switch c := contents[0].(type) {
	case mcp.TextResourceContents:
		return mcp.NewToolResultText(c.Text), nil
	case mcp.BlobResourceContents:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewImageContent(c.Blob, c.MIMEType),
			},
		}, nil
	default:
		return mcp.NewToolResultError("unexpected resource content type"), nil
	}
}

// registerResources registers MCP resource templates with the server.
func (s *Server) registerResources(mcpServer *server.MCPServer) {
	mcpServer.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"file://{id}/{blob_name}/{+path}",
			"File content",
			mcp.WithTemplateDescription("Raw file content from a Git repository at a given commit, tag, or branch"),
			mcp.WithTemplateMIMEType("text/plain"),
		),
		s.handleReadFile,
	)
}

// handleReadFile handles resource reads for file://{id}/{blob_name}/{+path}.
// Supports optional query parameters:
//   - lines: line ranges to extract (e.g. L17-L26,L45)
//   - line_numbers: "true" to prefix each line with its 1-based number
func (s *Server) handleReadFile(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := request.Params.URI

	// Parse: file://{id}/{blob_name}/{+path}[?lines=...&line_numbers=true]
	// URI looks like: file://1/main/src/foo.go?lines=L1-L10&line_numbers=true
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid file URI: %w", err)
	}
	if parsed.Scheme != "file" {
		return nil, fmt.Errorf("invalid file URI: %s", uri)
	}

	// parsed.Host = "1", parsed.Path = "/main/src/foo.go"
	rest := parsed.Host + parsed.Path
	// rest = "1/main/src/foo.go"

	// Split into at least 3 parts: id / blob_name / path...
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid file URI: expected file://{id}/{blob_name}/{path}, got %s", uri)
	}

	repoID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid repository ID %q: %w", parts[0], err)
	}
	blobName := parts[1]
	filePath := parts[2]

	query := parsed.Query()
	mode := query.Get("mode")
	pageParam := query.Get("page")

	if mode != "" && mode != "raster" && mode != "text" {
		return nil, fmt.Errorf("unsupported mode %q, valid modes: raster, text", mode)
	}

	if pageParam != "" && mode == "" {
		return nil, fmt.Errorf("page parameter requires mode=raster or mode=text")
	}

	// Raster mode: render a document page as a base64-encoded PNG.
	if mode == "raster" {
		return s.handleRasterRead(ctx, uri, repoID, blobName, filePath, pageParam)
	}

	// Text mode: extract text from a document page.
	if mode == "text" {
		return s.handleTextRead(ctx, uri, repoID, blobName, filePath, pageParam, query)
	}

	result, err := s.fileContent.Content(ctx, repoID, blobName, filePath)
	if err != nil {
		return nil, fmt.Errorf("read file content: %w", err)
	}

	content := result.Content()
	linesParam := query.Get("lines")
	lineNumbers := query.Get("line_numbers") == "true"

	if linesParam != "" || lineNumbers {
		filter, filterErr := service.NewLineFilter(linesParam)
		if filterErr != nil {
			return nil, fmt.Errorf("invalid lines parameter: %w", filterErr)
		}

		if lineNumbers {
			content = filter.ApplyWithLineNumbers(content)
		} else {
			content = filter.Apply(content)
		}
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "text/plain",
			Text:     string(content),
		},
	}, nil
}

// handleRasterRead renders a document page and returns it as a base64-encoded PNG blob.
func (s *Server) handleRasterRead(ctx context.Context, uri string, repoID int64, blobName, filePath, pageStr string) ([]mcp.ResourceContents, error) {
	if s.diskPathResolver == nil || s.rasterizers == nil {
		return nil, fmt.Errorf("rasterization not available")
	}

	if pageStr == "" {
		return nil, fmt.Errorf("page parameter is required for raster mode")
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return nil, fmt.Errorf("page must be a positive integer")
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	rast, ok := s.rasterizers.For(ext)
	if !ok {
		return nil, fmt.Errorf("rasterization not supported for %s files", ext)
	}

	diskPath, _, err := s.diskPathResolver.DiskPath(ctx, repoID, blobName, filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve disk path: %w", err)
	}

	img, err := rast.Render(diskPath, page)
	if err != nil {
		return nil, fmt.Errorf("render page %d: %w", page, err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.BlobResourceContents{
			URI:      uri,
			MIMEType: "image/jpeg",
			Blob:     base64.StdEncoding.EncodeToString(buf.Bytes()),
		},
	}, nil
}

// handleTextRead extracts text from a document page and returns it as text content.
func (s *Server) handleTextRead(ctx context.Context, uri string, repoID int64, blobName, filePath, pageStr string, query url.Values) ([]mcp.ResourceContents, error) {
	if s.diskPathResolver == nil || s.textRenderers == nil {
		return nil, fmt.Errorf("text rendering not available")
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	renderer, ok := s.textRenderers.For(ext)
	if !ok {
		return nil, fmt.Errorf("text extraction not supported for %s files", ext)
	}

	diskPath, _, err := s.diskPathResolver.DiskPath(ctx, repoID, blobName, filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve disk path: %w", err)
	}

	// No page parameter: return page count.
	if pageStr == "" {
		count, countErr := renderer.PageCount(diskPath)
		if countErr != nil {
			return nil, fmt.Errorf("get page count: %w", countErr)
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "text/plain",
				Text:     fmt.Sprintf("Page count: %d", count),
			},
		}, nil
	}

	page, parseErr := strconv.Atoi(pageStr)
	if parseErr != nil || page < 1 {
		return nil, fmt.Errorf("page must be a positive integer")
	}

	text, err := renderer.Render(diskPath, page)
	if err != nil {
		return nil, fmt.Errorf("extract text from page %d: %w", page, err)
	}

	content := []byte(text)
	linesParam := query.Get("lines")
	lineNumbers := query.Get("line_numbers") == "true"

	if linesParam != "" || lineNumbers {
		filter, filterErr := service.NewLineFilter(linesParam)
		if filterErr != nil {
			return nil, fmt.Errorf("invalid lines parameter: %w", filterErr)
		}
		if lineNumbers {
			content = filter.ApplyWithLineNumbers(content)
		} else {
			content = filter.Apply(content)
		}
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "text/plain",
			Text:     string(content),
		},
	}, nil
}

// repoRelativePath normalizes a file path to be repository-relative.
// File records from legacy database migrations may contain absolute clone paths
// (e.g., /root/.kodit/clones/repo-name/src/main.go). This strips the prefix
// so URIs and paths are directly usable by resource readers.
func repoRelativePath(filePath string) string {
	if !filepath.IsAbs(filePath) {
		return filePath
	}

	parts := strings.Split(filepath.Clean(filePath), string(filepath.Separator))
	lastIdx := -1
	for i, part := range parts {
		if part == "clones" || part == "repos" {
			lastIdx = i
		}
	}
	if lastIdx >= 0 && lastIdx+2 < len(parts) {
		return filepath.Join(parts[lastIdx+2:]...)
	}

	return filePath
}

// normalizeExtension strips a leading dot so that ".py" and "py" compare equal.
func normalizeExtension(ext string) string {
	return strings.TrimPrefix(ext, ".")
}

// MCPServer returns the underlying MCP server for stdio serving.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// ServeStdio runs the MCP server on stdio.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}
