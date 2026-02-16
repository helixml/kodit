// Package mcp provides Model Context Protocol server functionality.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

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

// Server wraps the MCP server with kodit-specific tools.
type Server struct {
	mcpServer     *server.MCPServer
	searchService Searcher
	enrichments   EnrichmentLookup
	logger        *slog.Logger
}

// NewServer creates a new MCP server with the given dependencies.
func NewServer(searchService Searcher, enrichments EnrichmentLookup, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		searchService: searchService,
		enrichments:   enrichments,
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
		mcp.WithDescription("Get a code snippet by its ID"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("The numeric ID of the snippet enrichment"),
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

// handleGetSnippet handles the get_snippet tool invocation.
func (s *Server) handleGetSnippet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := request.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}

	if s.enrichments == nil {
		return mcp.NewToolResultError("enrichment lookup not configured"), nil
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid id: %s", idStr)), nil
	}

	e, err := s.enrichments.Get(ctx, repository.WithID(id))
	if err != nil {
		s.logger.Error("failed to get enrichment", slog.String("id", idStr), slog.Any("error", err))
		return mcp.NewToolResultError(fmt.Sprintf("failed to get snippet: %v", err)), nil
	}

	type snippetResult struct {
		ID       string `json:"id"`
		Content  string `json:"content"`
		Language string `json:"language"`
	}

	result := snippetResult{
		ID:       strconv.FormatInt(e.ID(), 10),
		Content:  e.Content(),
		Language: e.Language(),
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
