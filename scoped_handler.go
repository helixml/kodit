package kodit

import (
	"net/http"

	mcpinternal "github.com/helixml/kodit/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewMCPHandler creates an HTTP handler for the MCP protocol that sees every
// repository known to the client.
func NewMCPHandler(client *Client) http.Handler {
	return newMCPHandler(client)
}

// NewScopedMCPHandler creates an HTTP handler for the MCP protocol scoped to
// the given repository IDs. Only repositories in repoIDs are visible through
// the returned handler's tools, searches and resource reads (including the
// raster and text read modes).
//
// When repoIDs is nil or empty, the handler sees no repositories. Use
// NewMCPHandler for a handler that sees all repositories.
func NewScopedMCPHandler(client *Client, repoIDs []int64) http.Handler {
	return newMCPHandler(client, mcpinternal.WithRepositoryScope(repoIDs))
}

func newMCPHandler(client *Client, opts ...mcpinternal.ServerOption) http.Handler {
	var mcpOpts []mcpinternal.ServerOption
	if client.Rasterizers() != nil {
		mcpOpts = append(mcpOpts, mcpinternal.WithRasterization(client.Blobs, client.Rasterizers()))
	}
	if client.TextRenderers() != nil {
		mcpOpts = append(mcpOpts, mcpinternal.WithTextRendering(client.Blobs, client.TextRenderers()))
	}
	mcpOpts = append(mcpOpts, opts...)

	srv := mcpinternal.NewServer(
		client.Repositories,
		client.Commits,
		client.Enrichments,
		client.Blobs,
		client.Search,
		client.Search,
		client.Search,
		client.Enrichments,
		client.Blobs,
		client.Files,
		client.Grep,
		"1.0.0",
		client.logger,
		mcpOpts...,
	)
	return server.NewStreamableHTTPServer(srv.MCPServer())
}
