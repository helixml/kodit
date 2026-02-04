// Package mcp provides Model Context Protocol server functionality.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/helixml/kodit/internal/domain"
	"github.com/helixml/kodit/internal/indexing"
	"github.com/helixml/kodit/internal/search"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with kodit-specific tools.
type Server struct {
	mcpServer     *server.MCPServer
	searchService search.Service
	snippetRepo   indexing.SnippetRepository
	logger        *slog.Logger
}

// NewServer creates a new MCP server with the given dependencies.
func NewServer(searchService search.Service, snippetRepo indexing.SnippetRepository, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		searchService: searchService,
		snippetRepo:   snippetRepo,
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
	var opts []domain.SnippetSearchFiltersOption
	if lang := request.GetString("language", ""); lang != "" {
		opts = append(opts, domain.WithLanguage(lang))
	}

	filters := domain.NewSnippetSearchFilters(opts...)
	searchReq := domain.NewMultiSearchRequest(topK, query, query, nil, filters)

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
	for i, snippet := range snippets {
		results[i] = searchResult{
			SHA:      snippet.SHA(),
			Content:  snippet.Content(),
			Language: snippet.Extension(),
			Score:    scores[snippet.SHA()],
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

	if s.snippetRepo == nil {
		return mcp.NewToolResultError("snippet repository not configured"), nil
	}

	snippet, err := s.snippetRepo.BySHA(ctx, sha)
	if err != nil {
		s.logger.Error("failed to get snippet", slog.String("sha", sha), slog.Any("error", err))
		return mcp.NewToolResultError(fmt.Sprintf("failed to get snippet: %v", err)), nil
	}

	// Check if snippet was found (empty SHA means not found)
	if snippet.SHA() == "" {
		return mcp.NewToolResultError(fmt.Sprintf("snippet not found: %s", sha)), nil
	}

	type snippetResult struct {
		SHA       string `json:"sha"`
		Content   string `json:"content"`
		Extension string `json:"extension"`
	}

	result := snippetResult{
		SHA:       snippet.SHA(),
		Content:   snippet.Content(),
		Extension: snippet.Extension(),
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
