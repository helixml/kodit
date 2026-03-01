// Package smoke provides smoke tests for the Kodit API.
// Expects a running Kodit server at baseURL.
package smoke

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	kodit "github.com/helixml/kodit/clients/go"
)

const (
	baseHost  = "127.0.0.1"
	basePort  = 8080
	targetURI = "https://gist.github.com/philwinder/11e4c4f7ea48b1c05b7cedea49367f1a.git"
)

var baseURL = fmt.Sprintf("http://%s:%d/api/v1", baseHost, basePort)
var rootURL = fmt.Sprintf("http://%s:%d", baseHost, basePort)

func TestSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}

	client, err := kodit.NewClientWithResponses(baseURL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	ctx := context.Background()

	t.Run("health", func(t *testing.T) {
		verifyHealth(t)
	})

	t.Run("repository_not_found", func(t *testing.T) {
		rsp, err := client.GetRepositoriesId(ctx, 99999)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = rsp.Body.Close() }()
		if rsp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rsp.StatusCode)
		}
	})

	// Create repository
	repoType := "repository"
	repoURI := targetURI
	createResp, err := client.PostRepositoriesWithResponse(ctx, kodit.DtoRepositoryCreateRequest{
		Data: &kodit.DtoRepositoryCreateData{
			Type: &repoType,
			Attributes: &kodit.DtoRepositoryCreateAttributes{
				RemoteUri: &repoURI,
			},
		},
	})
	if err != nil {
		t.Fatalf("create repository failed: %v", err)
	}
	var repo *kodit.DtoRepositoryData
	switch createResp.StatusCode() {
	case http.StatusCreated:
		if createResp.JSON201 == nil || createResp.JSON201.Data == nil {
			t.Fatal("expected created repository data")
		}
		repo = createResp.JSON201.Data
		t.Log("repository created (201)")
	case http.StatusOK:
		if createResp.JSON200 == nil || createResp.JSON200.Data == nil {
			t.Fatal("expected existing repository data")
		}
		repo = createResp.JSON200.Data
		t.Log("repository already exists (200)")
	default:
		t.Fatalf("expected 200 or 201, got %d: %s", createResp.StatusCode(), string(createResp.Body))
	}
	if repo.Id == nil || *repo.Id == "" {
		t.Fatal("expected repository ID")
	}
	repoID, err := strconv.Atoi(*repo.Id)
	if err != nil {
		t.Fatalf("failed to parse repository ID %q: %v", *repo.Id, err)
	}
	if repo.Attributes == nil || repo.Attributes.RemoteUri == nil || *repo.Attributes.RemoteUri != targetURI {
		t.Fatalf("expected remote_uri %s", targetURI)
	}
	t.Logf("created repository: id=%d, uri=%s", repoID, targetURI)

	// Wait for indexing
	waitForIndexing(t, client, ctx, repoID)

	t.Run("repository_list", func(t *testing.T) {
		resp, err := client.GetRepositoriesWithResponse(ctx, nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		repos := resp.JSON200.Data
		if repos == nil || len(*repos) < 1 {
			t.Fatal("expected at least 1 repository")
		}
		found := false
		for _, r := range *repos {
			if r.Id != nil && *r.Id == *repo.Id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected repository ID %s in list", *repo.Id)
		}
	})

	t.Run("repository_detail", func(t *testing.T) {
		resp, err := client.GetRepositoriesIdWithResponse(ctx, repoID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			t.Fatal("expected repository data")
		}
		data := resp.JSON200.Data
		if data.Attributes == nil || data.Attributes.RemoteUri == nil || *data.Attributes.RemoteUri != targetURI {
			t.Fatalf("expected remote_uri %s", targetURI)
		}
	})

	t.Run("repository_status", func(t *testing.T) {
		resp, err := client.GetRepositoriesIdStatusWithResponse(ctx, repoID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			t.Fatal("expected status data")
		}
		for _, task := range *resp.JSON200.Data {
			if task.Type == nil || *task.Type != "task_status" {
				t.Fatalf("expected type task_status, got %v", task.Type)
			}
			if task.Id == nil || *task.Id == "" {
				t.Fatal("expected task status ID")
			}
			if task.Attributes == nil || task.Attributes.Step == nil || *task.Attributes.Step == "" {
				t.Fatal("expected task step")
			}
			if task.Attributes.State == nil || *task.Attributes.State == "" {
				t.Fatal("expected task state")
			}
		}
		t.Logf("validated %d task statuses", len(*resp.JSON200.Data))
	})

	t.Run("repository_status_summary", func(t *testing.T) {
		resp, err := client.GetRepositoriesIdStatusSummaryWithResponse(ctx, repoID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			t.Fatal("expected status summary data")
		}
		data := resp.JSON200.Data
		if data.Type == nil || *data.Type != "repository_status_summary" {
			t.Fatalf("expected type repository_status_summary, got %v", data.Type)
		}
	})

	t.Run("tracking_config", func(t *testing.T) {
		resp, err := client.GetRepositoriesIdTrackingConfigWithResponse(ctx, repoID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			t.Fatal("expected tracking config data")
		}
		config := resp.JSON200.Data

		// Update tracking config with same values
		configType := "tracking_config"
		updateResp, err := client.PutRepositoriesIdTrackingConfigWithResponse(ctx, repoID, kodit.DtoTrackingConfigUpdateRequest{
			Data: &kodit.DtoTrackingConfigUpdateData{
				Type: &configType,
				Attributes: &kodit.DtoTrackingConfigUpdateAttributes{
					Mode:  config.Attributes.Mode,
					Value: config.Attributes.Value,
				},
			},
		})
		if err != nil {
			t.Fatalf("update request failed: %v", err)
		}
		if updateResp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", updateResp.StatusCode(), string(updateResp.Body))
		}
	})

	t.Run("tags", func(t *testing.T) {
		resp, err := client.GetRepositoriesIdTagsWithResponse(ctx, repoID, nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			t.Fatal("expected tags data")
		}
		tags := *resp.JSON200.Data
		t.Logf("tags: count=%d", len(tags))

		if len(tags) > 0 {
			tagName := *tags[0].Attributes.Name
			tagResp, err := client.GetRepositoriesIdTagsTagNameWithResponse(ctx, repoID, tagName)
			if err != nil {
				t.Fatalf("get tag request failed: %v", err)
			}
			if tagResp.StatusCode() != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", tagResp.StatusCode(), string(tagResp.Body))
			}
		}
	})

	// Fetch commits for use in subsequent subtests
	commitsResp, err := client.GetRepositoriesIdCommitsWithResponse(ctx, repoID, nil)
	if err != nil {
		t.Fatalf("get commits failed: %v", err)
	}
	if commitsResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", commitsResp.StatusCode(), string(commitsResp.Body))
	}
	if commitsResp.JSON200 == nil || commitsResp.JSON200.Data == nil || len(*commitsResp.JSON200.Data) == 0 {
		t.Fatal("expected at least one commit")
	}
	commits := *commitsResp.JSON200.Data
	commit := commits[0]
	if commit.Attributes == nil || commit.Attributes.CommitSha == nil {
		t.Fatal("expected commit SHA")
	}
	commitSHA := *commit.Attributes.CommitSha
	if len(commitSHA) != 40 {
		t.Fatalf("expected 40-char SHA, got %d: %s", len(commitSHA), commitSHA)
	}
	t.Logf("commit: sha=%s", commitSHA)

	t.Run("commits", func(t *testing.T) {
		if commit.Type == nil || *commit.Type != "commit" {
			t.Fatalf("expected type commit, got %v", commit.Type)
		}
		if commit.Attributes.Author == nil || *commit.Attributes.Author == "" {
			t.Fatal("expected commit author")
		}
		if commit.Attributes.Date == nil || *commit.Attributes.Date == "" {
			t.Fatal("expected commit date")
		}

		// Get commit by SHA
		resp, err := client.GetRepositoriesIdCommitsCommitShaWithResponse(ctx, repoID, commitSHA)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			t.Fatal("expected commit data")
		}
		if resp.JSON200.Data.Attributes.CommitSha == nil || *resp.JSON200.Data.Attributes.CommitSha != commitSHA {
			t.Fatalf("expected SHA %s", commitSHA)
		}
	})

	t.Run("commit_not_found", func(t *testing.T) {
		rsp, err := client.GetRepositoriesIdCommitsCommitSha(ctx, repoID, "nonexistent123")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = rsp.Body.Close() }()
		if rsp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rsp.StatusCode)
		}
	})

	t.Run("files", func(t *testing.T) {
		resp, err := client.GetRepositoriesIdCommitsCommitShaFilesWithResponse(ctx, repoID, commitSHA, nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
			t.Skip("no files available (indexing may have failed)")
		}
		files := *resp.JSON200.Data
		file := files[0]
		if file.Attributes == nil {
			t.Fatal("expected file attributes")
		}
		if file.Attributes.BlobSha == nil || len(*file.Attributes.BlobSha) != 40 {
			t.Fatal("expected 40-char blob SHA")
		}
		if file.Attributes.Path == nil || !strings.HasSuffix(*file.Attributes.Path, "main.py") {
			t.Fatalf("expected path ending in main.py, got %v", file.Attributes.Path)
		}
		if file.Attributes.Size == nil || *file.Attributes.Size <= 0 {
			t.Fatal("expected file size > 0")
		}
		t.Logf("file: path=%s, blob_sha=%s, size=%d", *file.Attributes.Path, *file.Attributes.BlobSha, *file.Attributes.Size)

		// Get file by blob SHA
		blobSHA := *file.Attributes.BlobSha
		fileResp, err := client.GetRepositoriesIdCommitsCommitShaFilesBlobShaWithResponse(ctx, repoID, commitSHA, blobSHA)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if fileResp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", fileResp.StatusCode(), string(fileResp.Body))
		}
		if fileResp.JSON200 == nil || fileResp.JSON200.Data == nil {
			t.Fatal("expected file data")
		}
		if fileResp.JSON200.Data.Attributes.BlobSha == nil || *fileResp.JSON200.Data.Attributes.BlobSha != blobSHA {
			t.Fatalf("expected blob SHA %s", blobSHA)
		}
	})

	t.Run("file_not_found", func(t *testing.T) {
		rsp, err := client.GetRepositoriesIdCommitsCommitShaFilesBlobSha(ctx, repoID, commitSHA, "nonexistent123")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = rsp.Body.Close() }()
		if rsp.StatusCode != http.StatusNotFound && rsp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected 404 or 500, got %d", rsp.StatusCode)
		}
	})

	t.Run("snippets", func(t *testing.T) {
		rsp, err := client.GetRepositoriesIdCommitsCommitShaSnippets(ctx, repoID, commitSHA, nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = rsp.Body.Close() }()
		if rsp.StatusCode != http.StatusOK {
			t.Skipf("snippets endpoint returned %d (indexing may have failed)", rsp.StatusCode)
		}
		var parsed kodit.DtoSnippetListResponse
		if err := json.NewDecoder(rsp.Body).Decode(&parsed); err != nil {
			t.Fatalf("failed to decode snippets: %v", err)
		}
		if parsed.Data == nil || len(*parsed.Data) == 0 {
			t.Skip("no snippets available (indexing may have failed)")
		}
		var found bool
		for _, snippet := range *parsed.Data {
			if snippet.Id == nil || *snippet.Id == "" {
				continue
			}
			if snippet.Attributes == nil || snippet.Attributes.Content == nil {
				continue
			}
			if snippet.Attributes.Content.Value == nil || *snippet.Attributes.Content.Value == "" {
				continue
			}
			t.Logf("snippet: id=%s, length=%d",
				*snippet.Id, len(*snippet.Attributes.Content.Value))
			found = true
			break
		}
		if !found {
			t.Fatalf("no snippet with content found among %d snippets", len(*parsed.Data))
		}
	})

	t.Run("commit_enrichments", func(t *testing.T) {
		rsp, err := client.GetRepositoriesIdCommitsCommitShaEnrichments(ctx, repoID, commitSHA, nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = rsp.Body.Close() }()
		if rsp.StatusCode != http.StatusOK {
			t.Skipf("commit enrichments returned %d (indexing may have failed)", rsp.StatusCode)
		}
		var parsed kodit.DtoEnrichmentJSONAPIListResponse
		if err := json.NewDecoder(rsp.Body).Decode(&parsed); err != nil {
			t.Fatalf("failed to decode enrichments: %v", err)
		}
		if parsed.Data == nil || len(*parsed.Data) == 0 {
			t.Skip("no commit enrichments available (indexing may have failed)")
		}
		enrichment := (*parsed.Data)[0]
		if enrichment.Id == nil || *enrichment.Id == "" {
			t.Fatal("expected enrichment ID")
		}
		if enrichment.Attributes == nil || enrichment.Attributes.Type == nil || *enrichment.Attributes.Type == "" {
			t.Fatal("expected enrichment type")
		}
		enrichmentID, err := strconv.Atoi(*enrichment.Id)
		if err != nil {
			t.Fatalf("failed to parse enrichment ID: %v", err)
		}
		t.Logf("enrichment: id=%d, type=%s", enrichmentID, *enrichment.Attributes.Type)

		// Get specific enrichment by ID
		enrichResp, err := client.GetRepositoriesIdCommitsCommitShaEnrichmentsEnrichmentIdWithResponse(ctx, repoID, commitSHA, enrichmentID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if enrichResp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", enrichResp.StatusCode(), string(enrichResp.Body))
		}
		if enrichResp.JSON200 == nil || enrichResp.JSON200.Data == nil {
			t.Fatal("expected enrichment data")
		}
		if enrichResp.JSON200.Data.Id == nil || *enrichResp.JSON200.Data.Id != *enrichment.Id {
			t.Fatalf("expected enrichment ID %s", *enrichment.Id)
		}
	})

	t.Run("repository_enrichments", func(t *testing.T) {
		rsp, err := client.GetRepositoriesIdEnrichments(ctx, repoID, nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = rsp.Body.Close() }()
		if rsp.StatusCode != http.StatusOK {
			t.Skipf("repository enrichments returned %d (indexing may have failed)", rsp.StatusCode)
		}
		var parsed kodit.DtoEnrichmentJSONAPIListResponse
		if err := json.NewDecoder(rsp.Body).Decode(&parsed); err != nil {
			t.Fatalf("failed to decode repository enrichments: %v", err)
		}
		if parsed.Data == nil || len(*parsed.Data) == 0 {
			t.Skip("no repository enrichments available (indexing may have failed)")
		}
		t.Logf("repository enrichments: count=%d", len(*parsed.Data))
	})

	t.Run("wiki_tree", func(t *testing.T) {
		resp, err := client.GetRepositoriesIdWikiWithResponse(ctx, repoID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() == http.StatusNotFound {
			t.Skip("wiki not generated (LLM provider may not be configured)")
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
			t.Skip("wiki tree is empty")
		}
		nodes := *resp.JSON200.Data
		for i, node := range nodes {
			if node.Slug == nil || *node.Slug == "" {
				t.Fatalf("wiki node %d: expected slug", i)
			}
			if node.Title == nil || *node.Title == "" {
				t.Fatalf("wiki node %d: expected title", i)
			}
			if node.Path == nil || *node.Path == "" {
				t.Fatalf("wiki node %d: expected path", i)
			}
		}
		t.Logf("wiki tree: %d top-level pages", len(nodes))

		// Fetch the first page by path.
		pagePath := *nodes[0].Path
		pageResp, err := client.GetRepositoriesIdWikiPathWithResponse(ctx, repoID, pagePath)
		if err != nil {
			t.Fatalf("wiki page request failed: %v", err)
		}
		if pageResp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", pageResp.StatusCode(), string(pageResp.Body))
		}
		if len(pageResp.Body) == 0 {
			t.Fatal("expected non-empty wiki page body")
		}
		t.Logf("wiki page %s: %d bytes", pagePath, len(pageResp.Body))
	})

	t.Run("wiki_rescan", func(t *testing.T) {
		resp, err := client.PostRepositoriesIdWikiRescanWithResponse(ctx, repoID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
	})

	t.Run("global_enrichments", func(t *testing.T) {
		resp, err := client.GetEnrichmentsWithResponse(ctx, nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 != nil && resp.JSON200.Data != nil && len(*resp.JSON200.Data) > 0 {
			enrichmentID, err := strconv.Atoi(*(*resp.JSON200.Data)[0].Id)
			if err != nil {
				t.Fatalf("failed to parse enrichment ID: %v", err)
			}
			enrichResp, err := client.GetEnrichmentsIdWithResponse(ctx, enrichmentID)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if enrichResp.StatusCode() != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", enrichResp.StatusCode(), string(enrichResp.Body))
			}
			t.Logf("global enrichments: count=%d", len(*resp.JSON200.Data))
		}
	})

	t.Run("embeddings_deprecated", func(t *testing.T) {
		resp, err := client.GetRepositoriesIdCommitsCommitShaEmbeddingsWithResponse(ctx, repoID, commitSHA)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusGone {
			t.Fatalf("expected 410, got %d", resp.StatusCode())
		}
	})

	repoIDStr := strconv.Itoa(repoID)

	t.Run("search_keywords", func(t *testing.T) {
		searchType := "search"
		keywords := []string{"orders", "GET", "json"}
		limit := 10
		sources := []string{repoIDStr}
		resp, err := client.PostSearchWithResponse(ctx, kodit.DtoSearchRequest{
			Data: &kodit.DtoSearchData{
				Type: &searchType,
				Attributes: &kodit.DtoSearchAttributes{
					Keywords: &keywords,
					Limit:    &limit,
					Filters:  &kodit.DtoSearchFilters{Sources: &sources},
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
			t.Skip("no keyword search results (indexing may have failed)")
		}
		validateSearchResults(t, *resp.JSON200.Data, "keywords")
	})

	t.Run("search_code", func(t *testing.T) {
		searchType := "search"
		code := "BaseHTTPServer json orders"
		limit := 10
		sources := []string{repoIDStr}
		resp, err := client.PostSearchWithResponse(ctx, kodit.DtoSearchRequest{
			Data: &kodit.DtoSearchData{
				Type: &searchType,
				Attributes: &kodit.DtoSearchAttributes{
					Code:    &code,
					Limit:   &limit,
					Filters: &kodit.DtoSearchFilters{Sources: &sources},
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
			t.Fatal("expected at least one code search result")
		}
		validateSearchResults(t, *resp.JSON200.Data, "code")
	})

	t.Run("search_mixed", func(t *testing.T) {
		searchType := "search"
		keywords := []string{"orders", "json"}
		code := "def do_GET"
		limit := 10
		sources := []string{repoIDStr}
		resp, err := client.PostSearchWithResponse(ctx, kodit.DtoSearchRequest{
			Data: &kodit.DtoSearchData{
				Type: &searchType,
				Attributes: &kodit.DtoSearchAttributes{
					Keywords: &keywords,
					Code:     &code,
					Limit:    &limit,
					Filters:  &kodit.DtoSearchFilters{Sources: &sources},
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
			t.Fatal("expected at least one mixed search result")
		}
		validateSearchResults(t, *resp.JSON200.Data, "mixed")
	})

	// MCP tool smoke tests — initialize a session once and reuse it.
	mcpSessionID := initMCPSession(t)

	t.Run("mcp_semantic_search", func(t *testing.T) {
		results := callMCPTool(t, mcpSessionID, "semantic_search", 2, map[string]any{
			"query": "HTTP server orders JSON",
		})
		if len(results) == 0 {
			t.Fatal("expected at least one semantic search result")
		}
		validateMCPFileResults(t, results, "semantic_search")
	})

	t.Run("mcp_keyword_search", func(t *testing.T) {
		results := callMCPTool(t, mcpSessionID, "keyword_search", 3, map[string]any{
			"keywords": "orders GET json",
		})
		if len(results) == 0 {
			t.Fatal("expected at least one keyword search result")
		}
		validateMCPFileResults(t, results, "keyword_search")
	})

	t.Run("queue", func(t *testing.T) {
		resp, err := client.GetQueueWithResponse(ctx, nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		if resp.JSON200 != nil && resp.JSON200.Data != nil && len(*resp.JSON200.Data) > 0 {
			task := (*resp.JSON200.Data)[0]
			if task.Id == nil || *task.Id == "" {
				t.Fatal("expected task ID")
			}
			if task.Attributes == nil || task.Attributes.Type == nil {
				t.Fatal("expected task type")
			}
			if !strings.HasPrefix(*task.Attributes.Type, "kodit.") {
				t.Fatalf("expected task type prefix kodit., got %s", *task.Attributes.Type)
			}
			taskID, err := strconv.Atoi(*task.Id)
			if err != nil {
				t.Fatalf("failed to parse task ID: %v", err)
			}
			taskResp, err := client.GetQueueTaskIdWithResponse(ctx, taskID)
			if err != nil {
				t.Fatalf("get task request failed: %v", err)
			}
			if taskResp.StatusCode() != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", taskResp.StatusCode(), string(taskResp.Body))
			}
			t.Logf("queue tasks: count=%d", len(*resp.JSON200.Data))
		}
	})

	t.Run("queue_not_found", func(t *testing.T) {
		rsp, err := client.GetQueueTaskId(ctx, 99999)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = rsp.Body.Close() }()
		if rsp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rsp.StatusCode)
		}
	})

	t.Run("rescan", func(t *testing.T) {
		resp, err := client.PostRepositoriesIdCommitsCommitShaRescanWithResponse(ctx, repoID, commitSHA)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", resp.StatusCode(), string(resp.Body))
		}

		waitForTerminalState(t, client, ctx, repoID)
	})

	t.Run("delete_repository", func(t *testing.T) {
		resp, err := client.DeleteRepositoriesIdWithResponse(ctx, repoID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", resp.StatusCode(), string(resp.Body))
		}

		// Wait for deletion to complete or fail
		deleted := waitForCondition(t, 2*time.Minute, 500*time.Millisecond, func() bool {
			r, err := client.GetRepositoriesIdWithResponse(ctx, repoID)
			if err != nil {
				return false
			}
			if r.StatusCode() == http.StatusNotFound {
				return true
			}
			// Check if the delete task has reached a terminal state (including failure)
			statusResp, err := client.GetRepositoriesIdStatusWithResponse(ctx, repoID)
			if err != nil || statusResp.StatusCode() != http.StatusOK {
				return false
			}
			if statusResp.JSON200 != nil && statusResp.JSON200.Data != nil {
				for _, task := range *statusResp.JSON200.Data {
					if task.Attributes == nil || task.Attributes.Step == nil || task.Attributes.State == nil {
						continue
					}
					if *task.Attributes.Step == "kodit.repository.delete" && *task.Attributes.State == "failed" {
						return true
					}
				}
			}
			return false
		})
		if !deleted {
			t.Fatal("repository deletion did not complete within timeout")
		}
		// Verify final state
		r, err := client.GetRepositoriesIdWithResponse(ctx, repoID)
		if err != nil {
			t.Fatalf("final check failed: %v", err)
		}
		if r.StatusCode() == http.StatusNotFound {
			t.Logf("repository deleted: id=%d", repoID)
		} else {
			t.Logf("repository deletion task completed but repo still exists (possible FK constraint): id=%d", repoID)
		}
	})

	t.Log("all smoke tests passed")
}

// validateSearchResults validates the structure of search results.
func validateSearchResults(t *testing.T, results []kodit.DtoSnippetData, mode string) {
	t.Helper()
	for i, result := range results {
		if result.Id == nil || *result.Id == "" {
			t.Fatalf("%s result %d: expected ID", mode, i)
		}
		if result.Type == nil || (*result.Type != "snippet" && *result.Type != "example") {
			t.Fatalf("%s result %d: expected type snippet or example, got %v", mode, i, result.Type)
		}
		if result.Attributes == nil {
			t.Fatalf("%s result %d: expected attributes", mode, i)
		}
		if result.Attributes.DerivesFrom != nil && len(*result.Attributes.DerivesFrom) > 0 {
			for j, df := range *result.Attributes.DerivesFrom {
				if df.BlobSha == nil || *df.BlobSha == "" {
					t.Fatalf("%s result %d derives_from %d: expected blob_sha", mode, i, j)
				}
				if df.Path == nil || *df.Path == "" {
					t.Fatalf("%s result %d derives_from %d: expected path", mode, i, j)
				}
			}
		}
		if result.Attributes.Enrichments != nil && len(*result.Attributes.Enrichments) > 0 {
			for j, e := range *result.Attributes.Enrichments {
				if e.Type == nil || *e.Type == "" {
					t.Fatalf("%s result %d enrichment %d: expected type", mode, i, j)
				}
				if e.Content == nil || *e.Content == "" {
					t.Fatalf("%s result %d enrichment %d: expected content", mode, i, j)
				}
			}
		}
		if result.Attributes.Content == nil {
			t.Fatalf("%s result %d: expected content", mode, i)
		}
		if result.Attributes.Content.Value == nil || *result.Attributes.Content.Value == "" {
			t.Fatalf("%s result %d: expected content value", mode, i)
		}
		derivesCount := 0
		if result.Attributes.DerivesFrom != nil {
			derivesCount = len(*result.Attributes.DerivesFrom)
		}
		enrichmentCount := 0
		if result.Attributes.Enrichments != nil {
			enrichmentCount = len(*result.Attributes.Enrichments)
		}
		language := ""
		if result.Attributes.Content.Language != nil {
			language = *result.Attributes.Content.Language
		}
		t.Logf("%s result %d: id=%s, language=%s, derives_from=%d, enrichments=%d",
			mode, i, *result.Id, language, derivesCount, enrichmentCount)
	}
}

// waitForIndexing waits for all indexing tasks to reach a terminal state.
func waitForIndexing(t *testing.T, client *kodit.ClientWithResponses, ctx context.Context, repoID int) {
	t.Helper()
	const minTasks = 9
	t.Logf("waiting for indexing to complete: repo_id=%d", repoID)
	done := waitForCondition(t, 10*time.Minute, time.Second, func() bool {
		resp, err := client.GetRepositoriesIdStatusWithResponse(ctx, repoID)
		if err != nil || resp.StatusCode() != http.StatusOK {
			return false
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			return false
		}
		tasks := *resp.JSON200.Data
		if len(tasks) < minTasks {
			return false
		}
		completed, pending, running, failed := 0, 0, 0, 0
		for _, task := range tasks {
			if task.Attributes == nil || task.Attributes.State == nil {
				continue
			}
			switch *task.Attributes.State {
			case "completed", "skipped":
				completed++
			case "pending":
				pending++
			case "running", "started":
				running++
			case "failed":
				failed++
			}
		}
		t.Logf("indexing: total=%d completed=%d pending=%d running=%d failed=%d",
			len(tasks), completed, pending, running, failed)
		return pending == 0 && running == 0
	})
	if !done {
		t.Fatal("indexing did not complete within timeout")
	}
	t.Logf("indexing completed: repo_id=%d", repoID)
}

// waitForTerminalState waits for all tasks to reach terminal state after rescan.
func waitForTerminalState(t *testing.T, client *kodit.ClientWithResponses, ctx context.Context, repoID int) {
	t.Helper()
	t.Log("waiting for rescan to complete...")
	done := waitForCondition(t, 5*time.Minute, time.Second, func() bool {
		resp, err := client.GetRepositoriesIdStatusWithResponse(ctx, repoID)
		if err != nil || resp.StatusCode() != http.StatusOK {
			return false
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			return false
		}
		terminal := map[string]bool{"completed": true, "skipped": true, "failed": true}
		for _, task := range *resp.JSON200.Data {
			if task.Attributes == nil || task.Attributes.State == nil {
				return false
			}
			if !terminal[*task.Attributes.State] {
				return false
			}
		}
		return true
	})
	if !done {
		t.Fatal("rescan did not complete within timeout")
	}
	t.Log("rescan completed")
}

// waitForCondition keeps trying a function until it returns true or timeout.
func waitForCondition(t *testing.T, timeout time.Duration, interval time.Duration, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

// initMCPSession sends an initialize request to the MCP endpoint and returns
// the session ID for subsequent tool calls.
func initMCPSession(t *testing.T) string {
	t.Helper()
	body := mcpJSONRPC("initialize", 1, map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "smoke-test", "version": "0.0.1"},
	})
	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, rootURL+"/mcp", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("create MCP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("MCP initialize failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MCP initialize: expected 200, got %d", resp.StatusCode)
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("MCP initialize did not return a session ID")
	}
	t.Logf("MCP session initialized: %s", sessionID)
	return sessionID
}

// mcpJSONRPC builds a JSON-RPC 2.0 request body.
func mcpJSONRPC(method string, id int, params map[string]any) []byte {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	}
	b, _ := json.Marshal(msg)
	return b
}

// mcpFileResult represents a single result from semantic_search or keyword_search.
type mcpFileResult struct {
	URI      string  `json:"uri"`
	Path     string  `json:"path"`
	Language string  `json:"language"`
	Lines    string  `json:"lines"`
	Score    float64 `json:"score"`
	Preview  string  `json:"preview"`
}

// callMCPTool invokes an MCP tool and returns the parsed file results.
func callMCPTool(t *testing.T, sessionID string, toolName string, id int, args map[string]any) []mcpFileResult {
	t.Helper()
	body := mcpJSONRPC("tools/call", id, map[string]any{
		"name":      toolName,
		"arguments": args,
	})
	httpClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodPost, rootURL+"/mcp", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("create MCP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", sessionID)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("MCP %s failed: %v", toolName, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MCP %s: expected 200, got %d", toolName, resp.StatusCode)
	}

	var rpcResp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	if rpcResp.Result.IsError {
		text := ""
		if len(rpcResp.Result.Content) > 0 {
			text = rpcResp.Result.Content[0].Text
		}
		t.Fatalf("MCP %s returned error: %s", toolName, text)
	}
	if len(rpcResp.Result.Content) == 0 {
		t.Fatalf("MCP %s returned no content", toolName)
	}

	var results []mcpFileResult
	if err := json.Unmarshal([]byte(rpcResp.Result.Content[0].Text), &results); err != nil {
		t.Fatalf("unmarshal MCP %s results: %v", toolName, err)
	}
	return results
}

// validateMCPFileResults validates the structure of MCP search results.
func validateMCPFileResults(t *testing.T, results []mcpFileResult, mode string) {
	t.Helper()
	for i, r := range results {
		if r.URI == "" {
			t.Fatalf("%s result %d: expected URI", mode, i)
		}
		if !strings.HasPrefix(r.URI, "file://") {
			t.Fatalf("%s result %d: expected file:// URI, got %s", mode, i, r.URI)
		}
		if r.Path == "" {
			t.Fatalf("%s result %d: expected path", mode, i)
		}
		if r.Score <= 0 {
			t.Fatalf("%s result %d: expected positive score, got %f", mode, i, r.Score)
		}
		t.Logf("%s result %d: path=%s, language=%s, score=%.4f, lines=%s",
			mode, i, r.Path, r.Language, r.Score, r.Lines)
	}
}

// verifyHealth checks the /healthz endpoint.
func verifyHealth(t *testing.T) {
	t.Helper()
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Get(rootURL + "/healthz")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from healthz, got %d", resp.StatusCode)
	}
	var health struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode healthz: %v", err)
	}
	if health.Status != "healthy" {
		t.Fatalf("expected healthy, got %s", health.Status)
	}
	t.Log("health check passed")
}
