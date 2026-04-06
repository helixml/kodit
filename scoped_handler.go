package kodit

import (
	"net/http"

	mcpinternal "github.com/helixml/kodit/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewScopedMCPHandler creates an HTTP handler for the MCP protocol scoped to
// the given repository IDs. Only repositories in repoIDs are visible through
// the returned handler's tools and searches.
//
// When repoIDs is nil or empty, the handler is unscoped — identical to the
// full MCP endpoint that sees all repositories.
func NewScopedMCPHandler(client *Client, repoIDs []int64) http.Handler {
	repositories := mcpinternal.RepositoryLister(client.Repositories)
	fileContent := mcpinternal.FileContentReader(client.Blobs)
	semanticSearch := mcpinternal.SemanticSearcher(client.Search)
	keywordSearch := mcpinternal.KeywordSearcher(client.Search)
	grepper := mcpinternal.Grepper(client.Grep)
	fileLister := mcpinternal.FileLister(client.Blobs)

	if len(repoIDs) > 0 {
		repositories, fileContent, semanticSearch, keywordSearch, grepper, fileLister =
			mcpinternal.Scope(repositories, fileContent, semanticSearch, keywordSearch, grepper, fileLister, repoIDs)
	}

	var mcpOpts []mcpinternal.ServerOption
	if client.Rasterizers() != nil {
		mcpOpts = append(mcpOpts, mcpinternal.WithRasterization(client.Blobs, client.Rasterizers()))
	}
	srv := mcpinternal.NewServer(
		repositories,
		client.Commits,
		client.Enrichments,
		fileContent,
		semanticSearch,
		keywordSearch,
		client.Search,
		client.Enrichments,
		fileLister,
		client.Files,
		grepper,
		"1.0.0",
		client.logger,
		mcpOpts...,
	)
	return server.NewStreamableHTTPServer(srv.MCPServer())
}
