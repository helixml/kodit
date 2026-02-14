// Package mcp provides Model Context Protocol server functionality.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/search"
	"github.com/helixml/kodit/domain/snippet"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Searcher provides code search operations for MCP tools.
type Searcher interface {
	Search(ctx context.Context, request search.MultiRequest) (service.MultiSearchResult, error)
}

// SnippetLookup provides snippet retrieval by SHA for MCP tools.
type SnippetLookup interface {
	BySHA(ctx context.Context, sha string) (snippet.Snippet, error)
}

// Server wraps the MCP server with kodit-specific tools.
type Server struct {
	mcpServer     *server.MCPServer
	searchService Searcher
	snippets      SnippetLookup
	logger        *slog.Logger
}

// NewServer creates a new MCP server with the given dependencies.
func NewServer(searchService Searcher, snippets SnippetLookup, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		searchService: searchService,
		snippets:      snippets,
		logger:        logger,
	}

	// Create MCP server with server info
	mcpServer := server.NewMCPServer(
		"kodit",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	// Register tools
	s.registerTools(mcpServer)

	s.mcpServer = mcpServer
	return s
}

// registerTools registers all kodit tools with the MCP server.
func (s *Server) registerTools(mcpServer *server.MCPServer) {
	// Search tool
	searchTool := mcp.NewTool("search",
		mcp.WithDescription("Search the codebase using hybrid BM25 and vector search"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query"),
		),
		mcp.WithNumber("top_k",
			mcp.Description("Number of results to return (default: 10)"),
		),
		mcp.WithString("language",
			mcp.Description("Filter by programming language"),
		),
	)

	mcpServer.AddTool(searchTool, s.handleSearch)

	// Get snippet tool
	getSnippetTool := mcp.NewTool("get_snippet",
		mcp.WithDescription("Get a code snippet by its SHA"),
		mcp.WithString("sha",
			mcp.Required(),
			mcp.Description("The SHA256 hash of the snippet"),
		),
	)

	mcpServer.AddTool(getSnippetTool, s.handleGetSnippet)
}

// handleSearch handles the search tool invocation.
func (s *Server) handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract arguments using helper methods
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query is required"), nil
	}

	topK := request.GetInt("top_k", 10)

	// Build filters
	var opts []search.FiltersOption
	if lang := request.GetString("language", ""); lang != "" {
		opts = append(opts, search.WithLanguage(lang))
	}

	filters := search.NewFilters(opts...)
	searchReq := search.NewMultiRequest(topK, query, query, nil, filters)

	// Execute search
	result, err := s.searchService.Search(ctx, searchReq)
	if err != nil {
		s.logger.Error("search failed", slog.Any("error", err))
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	// Format results
	snippets := result.Snippets()
	scores := result.FusedScores()

	type searchResult struct {
		SHA      string  `json:"sha"`
		Content  string  `json:"content"`
		Language string  `json:"language"`
		Score    float64 `json:"score"`
	}

	results := make([]searchResult, len(snippets))
	for i, snip := range snippets {
		results[i] = searchResult{
			SHA:      snip.SHA(),
			Content:  snip.Content(),
			Language: snip.Extension(),
			Score:    scores[snip.SHA()],
		}
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleGetSnippet handles the get_snippet tool invocation.
func (s *Server) handleGetSnippet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sha, err := request.RequireString("sha")
	if err != nil {
		return mcp.NewToolResultError("sha is required"), nil
	}

	if s.snippets == nil {
		return mcp.NewToolResultError("snippet lookup not configured"), nil
	}

	snip, err := s.snippets.BySHA(ctx, sha)
	if err != nil {
		s.logger.Error("failed to get snippet", slog.String("sha", sha), slog.Any("error", err))
		return mcp.NewToolResultError(fmt.Sprintf("failed to get snippet: %v", err)), nil
	}

	// Check if snippet was found (empty SHA means not found)
	if snip.SHA() == "" {
		return mcp.NewToolResultError(fmt.Sprintf("snippet not found: %s", sha)), nil
	}

	type snippetResult struct {
		SHA       string `json:"sha"`
		Content   string `json:"content"`
		Extension string `json:"extension"`
	}

	result := snippetResult{
		SHA:       snip.SHA(),
		Content:   snip.Content(),
		Extension: snip.Extension(),
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// MCPServer returns the underlying MCP server for stdio serving.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// ServeStdio runs the MCP server on stdio.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}
