// Package smoke provides smoke tests for the Kodit API.
// Expects a running Kodit server at baseURL.
package smoke

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	kodit "github.com/helixml/kodit/clients/go"
)

const (
	baseHost    = "127.0.0.1"
	basePort    = 8080
	targetURI   = "https://gist.github.com/philwinder/11e4c4f7ea48b1c05b7cedea49367f1a.git"
	companyDocs = "https://github.com/helix-acme-corp-demo/company-docs.git"
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

	t.Run("chunking_config", func(t *testing.T) {
		// GET defaults
		getURL := fmt.Sprintf("%s/repositories/%d/config/chunking", baseURL, repoID)
		resp := getJSON(t, getURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET chunking config: expected 200, got %d", resp.StatusCode)
		}
		var getResult struct {
			Data struct {
				Type       string `json:"type"`
				Attributes struct {
					ChunkSize    int `json:"chunk_size"`
					ChunkOverlap int `json:"chunk_overlap"`
					MinChunkSize int `json:"min_chunk_size"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&getResult); err != nil {
			t.Fatalf("decode GET response: %v", err)
		}
		if getResult.Data.Type != "chunking-config" {
			t.Fatalf("expected type chunking-config, got %s", getResult.Data.Type)
		}
		if getResult.Data.Attributes.ChunkSize != 1500 {
			t.Fatalf("expected default chunk_size 1500, got %d", getResult.Data.Attributes.ChunkSize)
		}
		if getResult.Data.Attributes.ChunkOverlap != 200 {
			t.Fatalf("expected default chunk_overlap 200, got %d", getResult.Data.Attributes.ChunkOverlap)
		}
		if getResult.Data.Attributes.MinChunkSize != 50 {
			t.Fatalf("expected default min_chunk_size 50, got %d", getResult.Data.Attributes.MinChunkSize)
		}
		t.Logf("chunking config defaults: size=%d overlap=%d min=%d",
			getResult.Data.Attributes.ChunkSize, getResult.Data.Attributes.ChunkOverlap, getResult.Data.Attributes.MinChunkSize)

		// PUT new values
		putURL := fmt.Sprintf("%s/repositories/%d/config/chunking", baseURL, repoID)
		putBody := `{"data":{"type":"chunking-config","attributes":{"chunk_size":2000,"chunk_overlap":300,"min_chunk_size":100}}}`
		httpClient := &http.Client{Timeout: 10 * time.Second}
		putReq, err := http.NewRequest(http.MethodPut, putURL, strings.NewReader(putBody))
		if err != nil {
			t.Fatalf("create PUT request: %v", err)
		}
		putReq.Header.Set("Content-Type", "application/json")
		putResp, err := httpClient.Do(putReq)
		if err != nil {
			t.Fatalf("PUT chunking config failed: %v", err)
		}
		defer func() { _ = putResp.Body.Close() }()
		if putResp.StatusCode != http.StatusOK {
			t.Fatalf("PUT chunking config: expected 200, got %d", putResp.StatusCode)
		}

		// Verify GET returns updated values
		verifyResp := getJSON(t, getURL)
		defer func() { _ = verifyResp.Body.Close() }()
		if verifyResp.StatusCode != http.StatusOK {
			t.Fatalf("GET after PUT: expected 200, got %d", verifyResp.StatusCode)
		}
		var verifyResult struct {
			Data struct {
				Attributes struct {
					ChunkSize    int `json:"chunk_size"`
					ChunkOverlap int `json:"chunk_overlap"`
					MinChunkSize int `json:"min_chunk_size"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(verifyResp.Body).Decode(&verifyResult); err != nil {
			t.Fatalf("decode verify response: %v", err)
		}
		if verifyResult.Data.Attributes.ChunkSize != 2000 {
			t.Fatalf("expected updated chunk_size 2000, got %d", verifyResult.Data.Attributes.ChunkSize)
		}
		if verifyResult.Data.Attributes.ChunkOverlap != 300 {
			t.Fatalf("expected updated chunk_overlap 300, got %d", verifyResult.Data.Attributes.ChunkOverlap)
		}
		if verifyResult.Data.Attributes.MinChunkSize != 100 {
			t.Fatalf("expected updated min_chunk_size 100, got %d", verifyResult.Data.Attributes.MinChunkSize)
		}
		t.Logf("chunking config updated: size=%d overlap=%d min=%d",
			verifyResult.Data.Attributes.ChunkSize, verifyResult.Data.Attributes.ChunkOverlap, verifyResult.Data.Attributes.MinChunkSize)

		// Restore defaults so subsequent tests aren't affected
		restoreBody := `{"data":{"type":"chunking-config","attributes":{"chunk_size":1500,"chunk_overlap":200,"min_chunk_size":50}}}`
		restoreReq, _ := http.NewRequest(http.MethodPut, putURL, strings.NewReader(restoreBody))
		restoreReq.Header.Set("Content-Type", "application/json")
		restoreResp, err := httpClient.Do(restoreReq)
		if err != nil {
			t.Fatalf("restore chunking config failed: %v", err)
		}
		_ = restoreResp.Body.Close()
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

	t.Run("search_semantic", func(t *testing.T) {
		semanticURL := fmt.Sprintf("%s/search/semantic?query=%s", baseURL, url.QueryEscape("HTTP server orders JSON"))
		resp := getJSON(t, semanticURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected at least one semantic search result")
		}
		t.Logf("semantic search: %d results", len(result.Data))
	})

	t.Run("search_keyword", func(t *testing.T) {
		keywordURL := fmt.Sprintf("%s/search/keyword?keywords=%s", baseURL, url.QueryEscape("orders GET json"))
		resp := getJSON(t, keywordURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected at least one keyword search result")
		}
		t.Logf("keyword search: %d results", len(result.Data))
	})

	t.Run("search_visual", func(t *testing.T) {
		visualURL := fmt.Sprintf("%s/search/visual?query=%s", baseURL, url.QueryEscape("HTTP server orders JSON"))
		resp := getJSON(t, visualURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		// The gist repo may not have rasterisable documents, so zero results
		// is acceptable — the key assertion is that the endpoint returns 200.
		t.Logf("visual search: %d results", len(result.Data))
	})

	t.Run("search_visual_missing_query", func(t *testing.T) {
		visualURL := fmt.Sprintf("%s/search/visual", baseURL)
		resp := getJSON(t, visualURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	// MCP tool smoke tests — initialize a session once and reuse it.
	mcpSessionID := initMCPSession(t)

	t.Run("mcp_semantic_search", func(t *testing.T) {
		results := callMCPTool(t, mcpSessionID, "kodit_semantic_search", 2, map[string]any{
			"query": "HTTP server orders JSON",
		})
		if len(results) == 0 {
			t.Fatal("expected at least one semantic search result")
		}
		validateMCPFileResults(t, results, "semantic_search")
	})

	t.Run("mcp_keyword_search", func(t *testing.T) {
		results := callMCPTool(t, mcpSessionID, "kodit_keyword_search", 3, map[string]any{
			"keywords": "orders GET json",
		})
		if len(results) == 0 {
			t.Fatal("expected at least one keyword search result")
		}
		validateMCPFileResults(t, results, "keyword_search")
	})

	t.Run("mcp_visual_search", func(t *testing.T) {
		// The gist repo may not have rasterisable documents, so zero results
		// is acceptable — the key assertion is that the tool executes without error.
		results := callMCPTool(t, mcpSessionID, "kodit_visual_search", 4, map[string]any{
			"query": "HTTP server orders JSON",
		})
		t.Logf("mcp visual search: %d results", len(results))
	})

	t.Run("mcp_ls", func(t *testing.T) {
		results := callMCPLs(t, mcpSessionID, 5, map[string]any{
			"repo_url": targetURI,
			"pattern":  "**/*.py",
		})
		if len(results) == 0 {
			t.Fatal("expected at least one ls result")
		}
		for i, r := range results {
			if r.URI == "" {
				t.Fatalf("ls result %d: expected uri", i)
			}
			if !strings.HasPrefix(r.URI, "file://") {
				t.Fatalf("ls result %d: expected file:// URI, got %s", i, r.URI)
			}
			t.Logf("ls result %d: uri=%s, size=%d", i, r.URI, r.Size)
		}
	})

	t.Run("mcp_read_resource", func(t *testing.T) {
		// First get a URI from semantic_search, then read it via read_resource.
		results := callMCPTool(t, mcpSessionID, "kodit_semantic_search", 6, map[string]any{
			"query": "HTTP server orders JSON",
			"limit": 1,
		})
		if len(results) == 0 {
			t.Fatal("expected at least one semantic search result to use as URI source")
		}
		uri := results[0].URI
		t.Logf("reading resource: %s", uri)

		text := callMCPToolText(t, mcpSessionID, "kodit_read_resource", 7, map[string]any{
			"uri": uri,
		})
		if text == "" {
			t.Fatal("expected non-empty file content from read_resource")
		}
		t.Logf("read_resource: %d bytes", len(text))
	})

	t.Run("ls", func(t *testing.T) {
		lsURL := fmt.Sprintf("%s/search/ls?repository_id=%d&pattern=%s", baseURL, repoID, url.QueryEscape("**/*.py"))
		resp := getJSON(t, lsURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []struct {
				Attributes struct {
					Path string `json:"path"`
				} `json:"attributes"`
				Links struct {
					Self string `json:"self"`
				} `json:"links"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected at least one file")
		}
		for _, f := range result.Data {
			if f.Attributes.Path == "" {
				t.Fatal("expected file path")
			}
			if !strings.HasSuffix(f.Attributes.Path, ".py") {
				t.Fatalf("expected .py file, got %s", f.Attributes.Path)
			}
			if !strings.HasPrefix(f.Links.Self, "/api/v1/repositories/") {
				t.Fatalf("expected blob API link, got %s", f.Links.Self)
			}
		}
		t.Logf("ls: %d matches", len(result.Data))
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

	// Switch the repository to the "rag" pipeline, rescan, and verify the
	// status only contains the smaller RAG-only operation set.
	t.Run("switch_pipeline_and_rescan", func(t *testing.T) {
		// List pipelines to find "default" and "rag" IDs.
		pipelinesResp, err := client.GetPipelinesWithResponse(ctx, nil)
		if err != nil {
			t.Fatalf("list pipelines failed: %v", err)
		}
		if pipelinesResp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", pipelinesResp.StatusCode(), string(pipelinesResp.Body))
		}
		if pipelinesResp.JSON200 == nil || pipelinesResp.JSON200.Data == nil {
			t.Fatal("expected pipeline data")
		}
		var defaultPipelineID, ragPipelineID int
		for _, p := range *pipelinesResp.JSON200.Data {
			if p.Attributes == nil || p.Attributes.Name == nil || p.Id == nil {
				continue
			}
			switch *p.Attributes.Name {
			case "default":
				defaultPipelineID = *p.Id
			case "rag":
				ragPipelineID = *p.Id
			}
		}
		if defaultPipelineID == 0 || ragPipelineID == 0 {
			t.Fatalf("expected both default and rag pipelines, got default=%d rag=%d", defaultPipelineID, ragPipelineID)
		}
		t.Logf("pipelines: default=%d rag=%d", defaultPipelineID, ragPipelineID)

		// Verify the repo is currently on the default pipeline.
		configResp, err := client.GetRepositoriesIdConfigPipelineWithResponse(ctx, repoID, nil)
		if err != nil {
			t.Fatalf("get pipeline config failed: %v", err)
		}
		if configResp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", configResp.StatusCode(), string(configResp.Body))
		}
		if configResp.JSON200 == nil || configResp.JSON200.Data == nil ||
			configResp.JSON200.Data.Attributes == nil || configResp.JSON200.Data.Attributes.PipelineId == nil {
			t.Fatal("expected pipeline config attributes")
		}
		if *configResp.JSON200.Data.Attributes.PipelineId != defaultPipelineID {
			t.Fatalf("expected pipeline_id=%d, got %d", defaultPipelineID, *configResp.JSON200.Data.Attributes.PipelineId)
		}

		// Switch to the "rag" pipeline.
		configType := "pipeline-config"
		putResp, err := client.PutRepositoriesIdConfigPipelineWithResponse(ctx, repoID, kodit.DtoPipelineConfigUpdateRequest{
			Data: &kodit.DtoPipelineConfigUpdateData{
				Type: &configType,
				Attributes: &kodit.DtoPipelineConfigAttributes{
					PipelineId: &ragPipelineID,
				},
			},
		})
		if err != nil {
			t.Fatalf("update pipeline config failed: %v", err)
		}
		if putResp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", putResp.StatusCode(), string(putResp.Body))
		}

		// Rescan the commit under the RAG pipeline.
		rescanResp, err := client.PostRepositoriesIdCommitsCommitShaRescanWithResponse(ctx, repoID, commitSHA)
		if err != nil {
			t.Fatalf("rescan request failed: %v", err)
		}
		if rescanResp.StatusCode() != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", rescanResp.StatusCode(), string(rescanResp.Body))
		}

		waitForTerminalState(t, client, ctx, repoID)

		// Verify the status only contains RAG-only operations (no enrichment steps).
		statusResp, err := client.GetRepositoriesIdStatusWithResponse(ctx, repoID)
		if err != nil {
			t.Fatalf("get status failed: %v", err)
		}
		if statusResp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", statusResp.StatusCode(), string(statusResp.Body))
		}
		if statusResp.JSON200 == nil || statusResp.JSON200.Data == nil {
			t.Fatal("expected status data")
		}

		// RAG-only pipeline must NOT contain enrichment operations.
		enrichmentOps := map[string]bool{
			"kodit.commit.create_summary_enrichment":         true,
			"kodit.commit.create_summary_embeddings":         true,
			"kodit.commit.create_public_api_docs":            true,
			"kodit.commit.create_architecture_enrichment":    true,
			"kodit.commit.create_commit_description":         true,
			"kodit.commit.create_database_schema":            true,
			"kodit.commit.create_cookbook":                   true,
			"kodit.commit.generate_wiki":                     true,
			"kodit.commit.create_example_summary":            true,
			"kodit.commit.create_example_summary_embeddings": true,
		}
		for _, task := range *statusResp.JSON200.Data {
			if task.Attributes == nil || task.Attributes.Step == nil {
				continue
			}
			step := *task.Attributes.Step
			if enrichmentOps[step] {
				t.Errorf("RAG pipeline should not contain enrichment step %q", step)
			}
		}

		// Restore the default pipeline for subsequent tests.
		putResp, err = client.PutRepositoriesIdConfigPipelineWithResponse(ctx, repoID, kodit.DtoPipelineConfigUpdateRequest{
			Data: &kodit.DtoPipelineConfigUpdateData{
				Type: &configType,
				Attributes: &kodit.DtoPipelineConfigAttributes{
					PipelineId: &defaultPipelineID,
				},
			},
		})
		if err != nil {
			t.Fatalf("restore pipeline config failed: %v", err)
		}
		if putResp.StatusCode() != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", putResp.StatusCode(), string(putResp.Body))
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

	// ── Document extraction smoke tests ────────────────────────────────
	// Index a repository containing binary documents (PDF, DOCX, PPTX, XLSX, ODT)
	// and verify that their text is searchable.

	docRepoURI := companyDocs
	docCreateResp, err := client.PostRepositoriesWithResponse(ctx, kodit.DtoRepositoryCreateRequest{
		Data: &kodit.DtoRepositoryCreateData{
			Type: &repoType,
			Attributes: &kodit.DtoRepositoryCreateAttributes{
				RemoteUri: &docRepoURI,
			},
		},
	})
	if err != nil {
		t.Fatalf("create document repository failed: %v", err)
	}
	var docRepo *kodit.DtoRepositoryData
	switch docCreateResp.StatusCode() {
	case http.StatusCreated:
		docRepo = docCreateResp.JSON201.Data
		t.Log("document repository created (201)")
	case http.StatusOK:
		docRepo = docCreateResp.JSON200.Data
		t.Log("document repository already exists (200)")
	default:
		t.Fatalf("expected 200 or 201, got %d: %s", docCreateResp.StatusCode(), string(docCreateResp.Body))
	}
	docRepoID, err := strconv.Atoi(*docRepo.Id)
	if err != nil {
		t.Fatalf("failed to parse document repo ID: %v", err)
	}
	t.Logf("document repository: id=%d, uri=%s", docRepoID, companyDocs)

	waitForIndexing(t, client, ctx, docRepoID)

	t.Run("doc_search_parental_leave_pdf", func(t *testing.T) {
		semanticURL := fmt.Sprintf("%s/search/semantic?query=%s&repository_id=%d",
			baseURL, url.QueryEscape("What are the rules about Parental Leave?"), docRepoID)
		resp := getJSON(t, semanticURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []struct {
				Attributes struct {
					Content struct {
						Value *string `json:"value"`
					} `json:"content"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected at least one result for parental leave query")
		}
		found := false
		for _, r := range result.Data {
			if r.Attributes.Content.Value != nil && strings.Contains(*r.Attributes.Content.Value, "Parental Leave") {
				if strings.Contains(*r.Attributes.Content.Value, "16 weeks") {
					found = true
					t.Logf("found parental leave snippet: %.120s...", *r.Attributes.Content.Value)
					break
				}
			}
		}
		if !found {
			t.Fatal("expected a result containing 'Parental Leave' and '16 weeks' from the PDF")
		}
	})

	t.Run("doc_visual_search_parental_leave_pdf", func(t *testing.T) {
		visualURL := fmt.Sprintf("%s/search/visual?query=%s&repository_id=%d",
			baseURL, url.QueryEscape("parental leave policy"), docRepoID)
		resp := getJSON(t, visualURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		// Vision embeddings require the SigLIP2 model; if not available the
		// pipeline step is skipped and zero results is acceptable.
		t.Logf("visual search (parental leave PDF): %d results", len(result.Data))
	})

	t.Run("doc_search_meeting_notes_docx", func(t *testing.T) {
		semanticURL := fmt.Sprintf("%s/search/semantic?query=%s&repository_id=%d",
			baseURL, url.QueryEscape("quarterly business review action items"), docRepoID)
		resp := getJSON(t, semanticURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected at least one result for meeting notes query")
		}
		t.Logf("meeting notes search: %d results", len(result.Data))
	})

	t.Run("doc_search_annual_report_odt", func(t *testing.T) {
		semanticURL := fmt.Sprintf("%s/search/semantic?query=%s&repository_id=%d",
			baseURL, url.QueryEscape("annual report revenue growth"), docRepoID)
		resp := getJSON(t, semanticURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected at least one result for annual report query")
		}
		t.Logf("annual report search: %d results", len(result.Data))
	})

	t.Run("doc_search_presentation_pptx", func(t *testing.T) {
		semanticURL := fmt.Sprintf("%s/search/semantic?query=%s&repository_id=%d",
			baseURL, url.QueryEscape("company strategic direction and vision"), docRepoID)
		resp := getJSON(t, semanticURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected at least one result for presentation query")
		}
		t.Logf("presentation search: %d results", len(result.Data))
	})

	t.Run("doc_search_financial_xlsx", func(t *testing.T) {
		semanticURL := fmt.Sprintf("%s/search/semantic?query=%s&repository_id=%d",
			baseURL, url.QueryEscape("financial statements operating expenses"), docRepoID)
		resp := getJSON(t, semanticURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected at least one result for financial query")
		}
		t.Logf("financial search: %d results", len(result.Data))
	})

	t.Run("doc_files_indexed", func(t *testing.T) {
		// Verify the document files were discovered by the scanner.
		docCommitsResp, err := client.GetRepositoriesIdCommitsWithResponse(ctx, docRepoID, nil)
		if err != nil {
			t.Fatalf("get commits failed: %v", err)
		}
		if docCommitsResp.JSON200 == nil || docCommitsResp.JSON200.Data == nil || len(*docCommitsResp.JSON200.Data) == 0 {
			t.Fatal("expected at least one commit")
		}
		docCommitSHA := *(*docCommitsResp.JSON200.Data)[0].Attributes.CommitSha
		filesResp, err := client.GetRepositoriesIdCommitsCommitShaFilesWithResponse(ctx, docRepoID, docCommitSHA, nil)
		if err != nil {
			t.Fatalf("get files failed: %v", err)
		}
		if filesResp.JSON200 == nil || filesResp.JSON200.Data == nil {
			t.Fatal("expected file data")
		}
		files := *filesResp.JSON200.Data
		expectedExts := map[string]bool{".pdf": false, ".docx": false, ".pptx": false, ".xlsx": false, ".odt": false}
		for _, f := range files {
			if f.Attributes == nil || f.Attributes.Path == nil {
				continue
			}
			path := *f.Attributes.Path
			for ext := range expectedExts {
				if strings.HasSuffix(path, ext) {
					expectedExts[ext] = true
					t.Logf("found %s file: %s", ext, path)
				}
			}
		}
		for ext, found := range expectedExts {
			if !found {
				t.Errorf("expected a %s file in the repository", ext)
			}
		}
	})

	t.Run("delete_document_repository", func(t *testing.T) {
		resp, err := client.DeleteRepositoriesIdWithResponse(ctx, docRepoID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode() != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", resp.StatusCode(), string(resp.Body))
		}
		deleted := waitForCondition(t, 2*time.Minute, 500*time.Millisecond, func() bool {
			r, err := client.GetRepositoriesIdWithResponse(ctx, docRepoID)
			if err != nil {
				return false
			}
			if r.StatusCode() == http.StatusNotFound {
				return true
			}
			statusResp, err := client.GetRepositoriesIdStatusWithResponse(ctx, docRepoID)
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
			t.Fatal("document repository deletion did not complete within timeout")
		}
		t.Logf("document repository deleted: id=%d", docRepoID)
	})

	t.Log("all smoke tests passed")
}

// TestSmoke_LocalDirectory verifies that a plain (non-git) local directory can
// be indexed via a file:// URI, that the synthetic commit SHA is stable across
// re-syncs when files have not changed (idempotency), and that a keyword search
// returns results from the indexed content.
//
// The test writes source files into a shared temporary directory that is
// bind-mounted into the kodit container at the same path. The default
// location is /tmp/kodit-smoke/smoke-plain-dir (override with
// KODIT_SMOKE_PLAIN_DIR). docker-compose.dev.yaml mounts /tmp/kodit-smoke
// into the container so the path is identical on host and in-container.
func TestSmoke_LocalDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}

	// Shared path visible to both the host and the container.
	fixtureDir := os.Getenv("KODIT_SMOKE_PLAIN_DIR")
	if fixtureDir == "" {
		fixtureDir = "/tmp/kodit-smoke/smoke-plain-dir"
	}
	plainDirURI := "file://" + fixtureDir

	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(fixtureDir) })

	const goSource = `package main

import "fmt"

// add returns the sum of two integers.
func add(a, b int) int {
	return a + b
}

// multiply returns the product of two integers.
func multiply(a, b int) int {
	return a * b
}

// subtract returns the difference of two integers.
func subtract(a, b int) int {
	return a - b
}

func main() {
	fmt.Println(add(1, 2))
	fmt.Println(multiply(3, 4))
	fmt.Println(subtract(9, 3))
}
`
	if err := os.WriteFile(filepath.Join(fixtureDir, "main.go"), []byte(goSource), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	client, err := kodit.NewClientWithResponses(baseURL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	ctx := context.Background()

	// Register the file:// repository.
	repoType := "repository"
	createResp, err := client.PostRepositoriesWithResponse(ctx, kodit.DtoRepositoryCreateRequest{
		Data: &kodit.DtoRepositoryCreateData{
			Type: &repoType,
			Attributes: &kodit.DtoRepositoryCreateAttributes{
				RemoteUri: &plainDirURI,
			},
		},
	})
	if err != nil {
		t.Fatalf("create repository failed: %v", err)
	}
	var repo *kodit.DtoRepositoryData
	switch createResp.StatusCode() {
	case http.StatusCreated:
		repo = createResp.JSON201.Data
		t.Log("local-directory repository created (201)")
	case http.StatusOK:
		repo = createResp.JSON200.Data
		t.Log("local-directory repository already exists (200)")
	default:
		t.Fatalf("expected 200 or 201, got %d: %s", createResp.StatusCode(), string(createResp.Body))
	}
	repoID, err := strconv.Atoi(*repo.Id)
	if err != nil {
		t.Fatalf("parse repo ID: %v", err)
	}
	t.Logf("local-directory repository: id=%d uri=%s", repoID, plainDirURI)

	t.Cleanup(func() {
		_, _ = client.DeleteRepositoriesIdWithResponse(ctx, repoID)
	})

	// Wait for the first indexing pass to finish.
	waitForIndexing(t, client, ctx, repoID)

	// Fetch the commit list — expect exactly one synthetic commit.
	commitsResp, err := client.GetRepositoriesIdCommitsWithResponse(ctx, repoID, nil)
	if err != nil {
		t.Fatalf("get commits failed: %v", err)
	}
	if commitsResp.JSON200 == nil || commitsResp.JSON200.Data == nil || len(*commitsResp.JSON200.Data) == 0 {
		t.Fatal("expected at least one commit after first sync")
	}
	firstSHA := *(*commitsResp.JSON200.Data)[0].Attributes.CommitSha
	if len(firstSHA) != 40 {
		t.Fatalf("expected 40-char SHA, got %d: %s", len(firstSHA), firstSHA)
	}
	t.Logf("first sync commit SHA: %s", firstSHA)

	t.Run("commit_has_synthetic_metadata", func(t *testing.T) {
		commit := (*commitsResp.JSON200.Data)[0]
		msg := commit.Attributes.Message
		author := commit.Attributes.Author
		if msg == nil || *msg != "Directory snapshot" {
			t.Fatalf("expected message 'Directory snapshot', got %v", msg)
		}
		if author == nil || *author != "kodit" {
			t.Fatalf("expected author 'kodit', got %v", author)
		}
	})

	// Second sync — files unchanged → same SHA (idempotent).
	syncResp, err := client.PostRepositoriesIdSync(ctx, repoID)
	if err != nil {
		t.Fatalf("trigger second sync: %v", err)
	}
	_ = syncResp.Body.Close()
	if syncResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 from sync, got %d", syncResp.StatusCode)
	}

	waitForIndexing(t, client, ctx, repoID)

	t.Run("second_sync_is_idempotent", func(t *testing.T) {
		commitsResp2, err := client.GetRepositoriesIdCommitsWithResponse(ctx, repoID, nil)
		if err != nil {
			t.Fatalf("get commits after second sync: %v", err)
		}
		if commitsResp2.JSON200 == nil || commitsResp2.JSON200.Data == nil || len(*commitsResp2.JSON200.Data) == 0 {
			t.Fatal("expected commits after second sync")
		}
		secondSHA := *(*commitsResp2.JSON200.Data)[0].Attributes.CommitSha
		if secondSHA != firstSHA {
			t.Fatalf("expected same SHA after unchanged sync: first=%s second=%s", firstSHA, secondSHA)
		}
		t.Logf("second sync SHA unchanged: %s", secondSHA)
	})

	// File change → new SHA → re-indexed.
	const updatedSource = `package main

import "fmt"

// add returns the sum of two integers.
func add(a, b int) int {
	return a + b
}

// multiply returns the product of two integers.
func multiply(a, b int) int {
	return a * b
}

// subtract returns the difference of two integers.
func subtract(a, b int) int {
	return a - b
}

// divide returns the integer quotient of a divided by b.
func divide(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}

func main() {
	fmt.Println(add(1, 2))
	fmt.Println(multiply(3, 4))
	fmt.Println(subtract(9, 3))
	fmt.Println(divide(10, 2))
}
`
	if err := os.WriteFile(filepath.Join(fixtureDir, "main.go"), []byte(updatedSource), 0o644); err != nil {
		t.Fatalf("update fixture file: %v", err)
	}

	syncResp2, err := client.PostRepositoriesIdSync(ctx, repoID)
	if err != nil {
		t.Fatalf("trigger third sync: %v", err)
	}
	_ = syncResp2.Body.Close()
	if syncResp2.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 from sync, got %d", syncResp2.StatusCode)
	}

	// Poll the commits endpoint until the SHA changes — this confirms the sync
	// task has processed the new directory hash and created a new commit record.
	// Only then is it safe to call waitForIndexing, because by the time the new
	// commit appears in the DB the commit.scan task will be in the queue.
	var thirdSHA string
	gotNewCommit := waitForCondition(t, 2*time.Minute, time.Second, func() bool {
		resp, err := client.GetRepositoriesIdCommitsWithResponse(ctx, repoID, nil)
		if err != nil || resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
			return false
		}
		sha := *(*resp.JSON200.Data)[0].Attributes.CommitSha
		if sha != firstSHA {
			thirdSHA = sha
			return true
		}
		return false
	})
	if !gotNewCommit {
		t.Fatal("new commit SHA did not appear after file change (timeout)")
	}
	t.Logf("new commit SHA detected: %s → %s", firstSHA[:8], thirdSHA[:8])

	waitForIndexing(t, client, ctx, repoID)

	t.Run("file_change_produces_new_sha", func(t *testing.T) {
		if len(thirdSHA) != 40 {
			t.Fatalf("expected 40-char SHA, got %d: %s", len(thirdSHA), thirdSHA)
		}
		t.Logf("file change produced new SHA: %s → %s", firstSHA[:8], thirdSHA[:8])
	})

	t.Run("keyword_search_returns_results", func(t *testing.T) {
		keywordURL := fmt.Sprintf("%s/search/keyword?keywords=%s&repository_id=%d",
			baseURL, url.QueryEscape("add multiply"), repoID)
		resp := getJSON(t, keywordURL)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var result struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode keyword search response: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected at least one keyword search result from local directory")
		}
		t.Logf("keyword search: %d results", len(result.Data))
	})
}

// validateSearchResults validates the structure of search results.
func validateSearchResults(t *testing.T, results []kodit.DtoSnippetData, mode string) {
	t.Helper()
	for i, result := range results {
		if result.Id == nil || *result.Id == "" {
			t.Fatalf("%s result %d: expected ID", mode, i)
		}
		if result.Type == nil || *result.Type == "" {
			t.Fatalf("%s result %d: expected non-empty type", mode, i)
		}
		if result.Attributes == nil {
			t.Fatalf("%s result %d: expected attributes", mode, i)
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
		enrichmentCount := 0
		if result.Attributes.Enrichments != nil {
			enrichmentCount = len(*result.Attributes.Enrichments)
		}
		language := ""
		if result.Attributes.Content.Language != nil {
			language = *result.Attributes.Content.Language
		}
		t.Logf("%s result %d: id=%s, language=%s, enrichments=%d",
			mode, i, *result.Id, language, enrichmentCount)
	}
}

// waitForIndexing waits for the repository status summary to reach a terminal state.
func waitForIndexing(t *testing.T, client *kodit.ClientWithResponses, ctx context.Context, repoID int) {
	t.Helper()
	t.Logf("waiting for indexing to complete: repo_id=%d", repoID)
	done := waitForCondition(t, 10*time.Minute, time.Second, func() bool {
		// Log individual task states for visibility.
		if statusResp, err := client.GetRepositoriesIdStatusWithResponse(ctx, repoID); err == nil &&
			statusResp.JSON200 != nil && statusResp.JSON200.Data != nil {
			completed, pending, running, failed := 0, 0, 0, 0
			for _, task := range *statusResp.JSON200.Data {
				if task.Attributes == nil || task.Attributes.State == nil {
					continue
				}
				switch *task.Attributes.State {
				case "completed", "skipped":
					completed++
				case "pending":
					pending++
				case "started", "in_progress":
					running++
				case "failed":
					failed++
				}
			}
			t.Logf("indexing: total=%d completed=%d pending=%d running=%d failed=%d",
				len(*statusResp.JSON200.Data), completed, pending, running, failed)
		}

		// Use the summary API for the completion check.
		resp, err := client.GetRepositoriesIdStatusSummaryWithResponse(ctx, repoID)
		if err != nil || resp.StatusCode() != http.StatusOK {
			return false
		}
		if resp.JSON200 == nil || resp.JSON200.Data == nil || resp.JSON200.Data.Attributes == nil {
			return false
		}
		status := ""
		if resp.JSON200.Data.Attributes.Status != nil {
			status = *resp.JSON200.Data.Attributes.Status
		}
		return status == "completed" || status == "completed_with_errors" || status == "failed"
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

// callMCPToolText invokes an MCP tool and returns the raw text content.
func callMCPToolText(t *testing.T, sessionID string, toolName string, id int, args map[string]any) string {
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

	return rpcResp.Result.Content[0].Text
}

// mcpLsResult represents a single result from ls.
type mcpLsResult struct {
	URI  string `json:"uri"`
	Size int64  `json:"size"`
}

// callMCPLs invokes the ls MCP tool and returns the parsed results.
func callMCPLs(t *testing.T, sessionID string, id int, args map[string]any) []mcpLsResult {
	t.Helper()
	body := mcpJSONRPC("tools/call", id, map[string]any{
		"name":      "kodit_ls",
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
		t.Fatalf("MCP ls failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MCP ls: expected 200, got %d", resp.StatusCode)
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
		t.Fatalf("MCP ls returned error: %s", text)
	}
	if len(rpcResp.Result.Content) == 0 {
		t.Fatalf("MCP ls returned no content")
	}

	var results []mcpLsResult
	if err := json.Unmarshal([]byte(rpcResp.Result.Content[0].Text), &results); err != nil {
		t.Fatalf("unmarshal MCP ls results: %v", err)
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

// getJSON sends a GET request and returns the response.
func getJSON(t *testing.T, url string) *http.Response {
	t.Helper()
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	return resp
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
