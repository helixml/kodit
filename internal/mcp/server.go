// Package mcp provides Model Context Protocol server functionality.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/chunk"
	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/domain/search"
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

// SemanticSearcher provides code vector search with scores.
type SemanticSearcher interface {
	SearchCodeWithScores(ctx context.Context, query string, topK int) ([]enrichment.Enrichment, map[string]float64, error)
}

// KeywordSearcher provides BM25 keyword search with scores.
type KeywordSearcher interface {
	SearchKeywordsWithScores(ctx context.Context, query string, limit int, filters search.Filters) ([]enrichment.Enrichment, map[string]float64, error)
}

// EnrichmentResolver provides enrichment-to-entity resolution.
type EnrichmentResolver interface {
	SourceFiles(ctx context.Context, enrichmentIDs []int64) (map[string][]int64, error)
	LineRanges(ctx context.Context, enrichmentIDs []int64) (map[string]chunk.LineRange, error)
	RepositoryIDs(ctx context.Context, enrichmentIDs []int64) (map[string]int64, error)
}

// FileFinder provides file lookups by options.
type FileFinder interface {
	Find(ctx context.Context, options ...repository.Option) ([]repository.File, error)
}

// Server wraps the MCP server with kodit-specific tools.
type Server struct {
	mcpServer          *server.MCPServer
	repositories       RepositoryLister
	commits            CommitFinder
	enrichmentQuery    EnrichmentQuery
	fileContent        FileContentReader
	semanticSearch     SemanticSearcher
	keywordSearch      KeywordSearcher
	enrichmentResolver EnrichmentResolver
	files              FileFinder
	version            string
	logger             *slog.Logger
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
	"- semantic_search() - Find files matching a natural language query (returns resource URIs)\n" +
	"- keyword_search() - Find files matching keywords using BM25 search (returns resource URIs)\n\n" +
	"**Reading file content:**\n" +
	"Use the file resource template: file://{id}/{blob_name}/{+path}\n" +
	"where id is the repository ID, blob_name is a commit SHA, tag, or branch name, " +
	"and path is the file path within the repository.\n" +
	"Optional query parameters: ?lines=L17-L26,L45 and ?line_numbers=true\n\n" +
	"Choose the most appropriate tool based on what information you need. " +
	"Often starting with architecture or API docs provides better context than " +
	"immediately searching for code snippets."

// NewServer creates a new MCP server with the given dependencies.
func NewServer(
	repositories RepositoryLister,
	commits CommitFinder,
	enrichmentQuery EnrichmentQuery,
	fileContent FileContentReader,
	semanticSearch SemanticSearcher,
	keywordSearch KeywordSearcher,
	enrichmentResolver EnrichmentResolver,
	files FileFinder,
	version string,
	logger *slog.Logger,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		repositories:       repositories,
		commits:            commits,
		enrichmentQuery:    enrichmentQuery,
		fileContent:        fileContent,
		semanticSearch:     semanticSearch,
		keywordSearch:      keywordSearch,
		enrichmentResolver: enrichmentResolver,
		files:              files,
		version:            version,
		logger:             logger,
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
func (s *Server) registerTools(mcpServer *server.MCPServer) {
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

	mcpServer.AddTool(mcp.NewTool("semantic_search",
		mcp.WithDescription("Search indexed files using semantic similarity and return file resource URIs"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Natural language description of what you are looking for"),
		),
		mcp.WithString("language",
			mcp.Description("Filter by file extension (e.g. .go, .py)"),
		),
		mcp.WithString("source_repo",
			mcp.Description("Filter by source repository URL"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results (default 10)"),
		),
	), s.handleSemanticSearch)

	mcpServer.AddTool(mcp.NewTool("keyword_search",
		mcp.WithDescription("Search indexed files using keyword-based BM25 search and return file resource URIs"),
		mcp.WithString("keywords",
			mcp.Required(),
			mcp.Description("Keywords to search for"),
		),
		mcp.WithString("source_repo",
			mcp.Description("Filter by source repository URL"),
		),
		mcp.WithString("language",
			mcp.Description("Filter by programming language"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results (default 10)"),
		),
	), s.handleKeywordSearch)
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
		return mcp.NewToolResultError(fmt.Sprintf("repo_url is required: %v", err)), nil
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
			repository.WithOrderDesc("date"),
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

// fileResult holds the resolved file information for a search result.
type fileResult struct {
	URI      string  `json:"uri"`
	Path     string  `json:"path"`
	Language string  `json:"language"`
	Lines    string  `json:"lines"`
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

	lineRanges, err := s.enrichmentResolver.LineRanges(ctx, ids)
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
		if lr, found := lineRanges[idStr]; found && lr.StartLine() > 0 {
			uri = uri.WithLineRange(lr.StartLine(), lr.EndLine())
			lines = fmt.Sprintf("L%d-L%d", lr.StartLine(), lr.EndLine())
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
		repos, repoErr := s.repositories.Find(ctx, repository.WithRemoteURL(repoURL))
		if repoErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolve source_repo: %v", repoErr)), nil
		}
		if len(repos) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		sourceRepoID = repos[0].ID()
	}

	enrichments, scores, err := s.semanticSearch.SearchCodeWithScores(ctx, query, limit)
	if err != nil {
		s.logger.Error("semantic search failed", slog.Any("error", err))
		return mcp.NewToolResultError(fmt.Sprintf("semantic search failed: %v", err)), nil
	}

	// Post-filter by language if specified.
	if language != "" {
		filtered := make([]enrichment.Enrichment, 0, len(enrichments))
		for _, e := range enrichments {
			if normalizeExtension(e.Language()) == language {
				filtered = append(filtered, e)
			}
		}
		enrichments = filtered
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
		repos, repoErr := s.repositories.Find(ctx, repository.WithRemoteURL(repoURL))
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
		s.logger.Error("keyword search failed", slog.Any("error", err))
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

	result, err := s.fileContent.Content(ctx, repoID, blobName, filePath)
	if err != nil {
		return nil, fmt.Errorf("read file content: %w", err)
	}

	content := result.Content()
	query := parsed.Query()
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
