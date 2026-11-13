package kodit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/repository"
)

// TestIntegration_ScopedMCPHandler_CannotReachOtherRepository indexes two
// real repositories and checks, through the public handler constructors, that
// a handler scoped to repository A cannot read repository B's documents in
// any read mode, while the unscoped handler can.
func TestIntegration_ScopedMCPHandler_CannotReachOtherRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	tmpDir := t.TempDir()
	client, err := kodit.New(
		kodit.WithSQLite(filepath.Join(tmpDir, "test.db")),
		kodit.WithDataDir(filepath.Join(tmpDir, "data")),
		kodit.WithCloneDir(filepath.Join(tmpDir, "repos")),
		kodit.WithModelDir("infrastructure/provider/models"),
		kodit.WithWorkerPollPeriod(testPollPeriod),
		kodit.WithLocalGitRepositories(),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	require.NotNil(t, client.Rasterizers(), "raster mode must be available")
	require.NotNil(t, client.TextRenderers(), "text mode must be available")

	ctx := context.Background()
	codePath := createTestGitRepo(t)
	docsPath := createTestGitRepoWithDocuments(t)
	repoA, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(codePath), Branch: currentBranch(t, codePath)})
	require.NoError(t, err)
	repoB, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{URL: fileURI(docsPath), Branch: currentBranch(t, docsPath)})
	require.NoError(t, err)
	waitForTasks(ctx, t, client, 120*time.Second)

	commitsB, err := client.Commits.Find(ctx, repository.WithRepoID(repoB.ID()))
	require.NoError(t, err)
	require.NotEmpty(t, commitsB)
	shaB := commitsB[0].SHA()

	const docText = "Installation guide"
	pdfB := fmt.Sprintf("file://%d/%s/docs/guide.pdf", repoB.ID(), shaB)
	docxB := fmt.Sprintf("file://%d/%s/docs/readme.docx", repoB.ID(), shaB)
	readsOfB := []string{
		pdfB,
		pdfB + "?mode=text",
		pdfB + "?mode=text&page=1",
		pdfB + "?mode=raster&page=1",
		docxB + "?mode=text&page=1",
	}

	unscoped := startMCP(t, kodit.NewMCPHandler(client))
	text, isErr := callTool(t, unscoped, "kodit_read_resource", map[string]any{"uri": pdfB + "?mode=text&page=1"})
	require.False(t, isErr, "unscoped handler should read repository B: %s", text)
	require.Contains(t, text, docText)
	text, _ = callTool(t, unscoped, "kodit_repositories", map[string]any{})
	assert.Contains(t, text, repoB.Repo().UpstreamURL())

	scoped := startMCP(t, kodit.NewScopedMCPHandler(client, []int64{repoA.ID()}))
	text, isErr = callTool(t, scoped, "kodit_repositories", map[string]any{})
	require.False(t, isErr, text)
	assert.Contains(t, text, repoA.Repo().UpstreamURL())
	assert.NotContains(t, text, repoB.Repo().UpstreamURL())

	for _, uri := range readsOfB {
		t.Run(uri, func(t *testing.T) {
			text, isErr := callTool(t, scoped, "kodit_read_resource", map[string]any{"uri": uri})
			assert.True(t, isErr, "scoped handler read repository B: %s", text)
			assert.NotContains(t, text, docText)

			result, err := scoped.ReadResource(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
			assert.Error(t, err, "scoped resource read reached repository B: %+v", result)
		})
	}

	for _, tool := range []string{"kodit_architecture_docs", "kodit_wiki"} {
		text, isErr = callTool(t, scoped, tool, map[string]any{"repo_url": repoA.Repo().UpstreamURL(), "commit_sha": shaB})
		assert.True(t, isErr, "%s accepted repository B's commit: %s", tool, text)
	}

	empty := startMCP(t, kodit.NewScopedMCPHandler(client, nil))
	text, _ = callTool(t, empty, "kodit_repositories", map[string]any{})
	assert.Contains(t, text, "No repositories found")
}

func startMCP(t *testing.T, handler http.Handler) *mcpclient.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := mcpclient.NewStreamableHttpClient(srv.URL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	require.NoError(t, c.Start(ctx))
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "0.0.1"}
	_, err = c.Initialize(ctx, init)
	require.NoError(t, err)
	return c
}

// callTool returns the tool result's content as text and whether it is an
// error result.
func callTool(t *testing.T, c *mcpclient.Client, name string, args map[string]any) (string, bool) {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	result, err := c.CallTool(context.Background(), req)
	require.NoError(t, err)
	b, err := json.Marshal(result.Content)
	require.NoError(t, err)
	return string(b), result.IsError
}
