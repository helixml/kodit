// Package smoke provides smoke tests for the Kodit API.
package smoke

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	baseHost  = "127.0.0.1"
	basePort  = 8080
	targetURI = "https://gist.github.com/philwinder/7aa38185e20433c04c533f2b28f4e217.git"
)

var baseURL = fmt.Sprintf("http://%s:%d", baseHost, basePort)

// portAvailable checks if a port is available by trying to listen on it.
func portAvailable(host string, port int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
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

// TestSmoke runs the full smoke test suite.
func TestSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}

	// Check port is available
	if !portAvailable(baseHost, basePort) {
		t.Fatalf("port %d is already in use", basePort)
	}

	// Create temp file for env (to avoid loading local .env)
	tmpEnv, err := os.CreateTemp("", "smoke-env-*")
	if err != nil {
		t.Fatalf("failed to create temp env file: %v", err)
	}
	tmpEnvPath := tmpEnv.Name()
	_ = tmpEnv.Close()
	defer func() { _ = os.Remove(tmpEnvPath) }()

	// Build command - use go run in tests
	cmdDir := findCmdDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	buildTags := os.Getenv("SMOKE_BUILD_TAGS")
	if buildTags == "" {
		buildTags = "fts5 ORT embed_model"
	}
	cmd := exec.CommandContext(ctx, "go", "run", "-tags="+buildTags, cmdDir,
		"serve",
		"--host", baseHost,
		"--port", strconv.Itoa(basePort),
		"--env-file", tmpEnvPath,
	)

	// Create a new process group so we can kill all child processes
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Check required API keys are set
	enrichmentAPIKey := os.Getenv("ENRICHMENT_ENDPOINT_API_KEY")
	if enrichmentAPIKey == "" {
		t.Fatal("ENRICHMENT_ENDPOINT_API_KEY environment variable is required")
	}

	// Set environment - use in-memory SQLite by default
	// Embeddings use the built-in jina-embeddings-v2-base-code model (no external API needed)
	cmd.Env = append(os.Environ(),
		"DISABLE_TELEMETRY=true",
		"DB_URL=sqlite:///:memory:",
		// Enrichment provider (OpenRouter with Ministral 8B)
		"ENRICHMENT_ENDPOINT_BASE_URL=https://openrouter.ai/api/v1",
		"ENRICHMENT_ENDPOINT_MODEL=mistralai/ministral-8b-2512",
		"ENRICHMENT_ENDPOINT_API_KEY="+enrichmentAPIKey,
	)
	if smokeDBURL := os.Getenv("SMOKE_DB_URL"); smokeDBURL != "" {
		cmd.Env = append(cmd.Env, "DB_URL="+smokeDBURL)
	}

	// Capture output for debugging
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start server
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Ensure cleanup
	defer func() {
		cancel()
		// Kill the entire process group to ensure child processes are also killed
		if cmd.Process != nil {
			// Send SIGKILL to the process group (negative PID)
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("server stdout:\n%s", stdout.String())
			t.Logf("server stderr:\n%s", stderr.String())
		}
	}()

	// Create HTTP client
	client := &http.Client{Timeout: 30 * time.Second}

	// Wait for server to start
	t.Log("waiting for server to start listening...")
	serverStarted := waitForCondition(t, 60*time.Second, time.Second, func() bool {
		return !portAvailable(baseHost, basePort)
	})
	if !serverStarted {
		t.Fatal("server failed to start within timeout")
	}

	// Test health endpoint
	t.Log("testing health endpoint...")
	resp := doRequest(t, client, "GET", baseURL+"/healthz", nil)
	assertStatus(t, resp, http.StatusOK)
	healthResp := decodeJSON[healthResponse](t, resp.Body)
	if healthResp.Status != "healthy" {
		t.Fatalf("expected healthy status, got %s", healthResp.Status)
	}
	t.Log("server health check passed")

	// Test 404 for non-existent repository
	t.Log("testing 404 for non-existent repository...")
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/99999", nil)
	assertStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
	t.Log("correctly returns 404 for non-existent repository")

	// Test repository lifecycle
	t.Log("testing repository lifecycle...")

	// List existing repositories (should be empty initially)
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories", nil)
	assertStatus(t, resp, http.StatusOK)
	repos := decodeJSON[repositoryListResponse](t, resp.Body)
	if len(repos.Data) != 0 {
		t.Fatalf("expected 0 repositories initially, got %d", len(repos.Data))
	}
	t.Log("verified repository list is empty initially")

	createPayload := repositoryCreateRequest{
		Data: repositoryCreateData{
			Type: "repository",
			Attributes: repositoryCreateAttributes{
				RemoteURI: targetURI,
			},
		},
	}
	resp = doRequest(t, client, "POST", baseURL+"/api/v1/repositories", createPayload)
	assertStatus(t, resp, http.StatusCreated)
	createdRepo := decodeJSON[repositoryResponse](t, resp.Body)

	// Verify created repository data
	if createdRepo.Data.Type != "repository" {
		t.Fatalf("expected type 'repository', got %s", createdRepo.Data.Type)
	}
	if createdRepo.Data.ID == "" {
		t.Fatal("expected repository ID to be set")
	}
	if createdRepo.Data.Attributes.RemoteURI != targetURI {
		t.Fatalf("expected remote_uri %s, got %s", targetURI, createdRepo.Data.Attributes.RemoteURI)
	}
	if createdRepo.Data.Attributes.CreatedAt == "" {
		t.Fatal("expected created_at to be set")
	}
	repoID := createdRepo.Data.ID
	t.Logf("created repository: id=%s, uri=%s", repoID, targetURI)

	// Verify repository appears in list
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories", nil)
	assertStatus(t, resp, http.StatusOK)
	repos = decodeJSON[repositoryListResponse](t, resp.Body)
	if len(repos.Data) != 1 {
		t.Fatalf("expected 1 repository, got %d", len(repos.Data))
	}
	if repos.Data[0].ID != repoID {
		t.Fatalf("expected repository ID %s in list, got %s", repoID, repos.Data[0].ID)
	}
	t.Log("verified repository appears in list")

	// Get repository by ID and verify data
	t.Logf("testing repository endpoints: repo_id=%s", repoID)
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID, nil)
	assertStatus(t, resp, http.StatusOK)
	repoDetail := decodeJSON[repositoryResponse](t, resp.Body)
	if repoDetail.Data.Attributes.RemoteURI != targetURI {
		t.Fatalf("GET repository: expected remote_uri %s, got %s", targetURI, repoDetail.Data.Attributes.RemoteURI)
	}
	t.Logf("verified repository details: remote_uri=%s", repoDetail.Data.Attributes.RemoteURI)

	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/status", nil)
	assertStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()

	// Wait for indexing to complete (including enrichments and embeddings)
	// We expect at least these tasks: clone, sync, scan, extract_snippets, extract_examples,
	// create_bm25_index, create_code_embeddings, create_summary_enrichment,
	// create_architecture_enrichment = minimum 9 tasks
	const minExpectedTasks = 9
	t.Logf("waiting for indexing to complete: repo_id=%s", repoID)
	indexingDone := waitForCondition(t, 10*time.Minute, time.Second, func() bool {
		resp := doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/status", nil)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return false
		}

		status := decodeJSON[taskStatusListResponse](t, resp.Body)

		// Count tasks by state
		completed := 0
		pending := 0
		running := 0
		failed := 0
		for _, task := range status.Data {
			switch task.Attributes.State {
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
		t.Logf("indexing status: total=%d, completed=%d, pending=%d, running=%d, failed=%d",
			len(status.Data), completed, pending, running, failed)

		// Need at least the minimum expected tasks to have been created
		if len(status.Data) < minExpectedTasks {
			return false
		}

		// Need all tasks in terminal state (no pending, running, or started)
		if pending > 0 || running > 0 {
			return false
		}

		return true
	})
	if !indexingDone {
		t.Fatal("indexing did not complete within timeout")
	}
	t.Logf("indexing completed: repo_id=%s", repoID)

	// Test status summary endpoint
	t.Logf("testing status summary: repo_id=%s", repoID)
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/status/summary", nil)
	assertStatus(t, resp, http.StatusOK)
	statusSummary := decodeJSON[statusSummaryResponse](t, resp.Body)
	if statusSummary.Data.Type != "repository_status_summary" {
		t.Fatalf("expected type 'repository_status_summary', got %s", statusSummary.Data.Type)
	}
	t.Logf("status summary: total_tasks=%d, completed=%d, failed=%d, pending=%d, running=%d",
		statusSummary.Data.Attributes.TotalTasks,
		statusSummary.Data.Attributes.CompletedTasks,
		statusSummary.Data.Attributes.FailedTasks,
		statusSummary.Data.Attributes.PendingTasks,
		statusSummary.Data.Attributes.RunningTasks)

	// Verify task status data
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/status", nil)
	assertStatus(t, resp, http.StatusOK)
	taskStatuses := decodeJSON[taskStatusListResponse](t, resp.Body)
	if len(taskStatuses.Data) == 0 {
		t.Fatal("expected at least one task status")
	}
	// Validate task status fields
	for _, taskStatus := range taskStatuses.Data {
		if taskStatus.Type != "task_status" {
			t.Fatalf("expected task_status type, got %s", taskStatus.Type)
		}
		if taskStatus.ID == "" {
			t.Fatal("expected task status to have ID")
		}
		if taskStatus.Attributes.Step == "" {
			t.Fatal("expected task status to have step")
		}
		if taskStatus.Attributes.State == "" {
			t.Fatal("expected task status to have state")
		}
		// Verify state is valid
		validStates := map[string]bool{"pending": true, "running": true, "started": true, "in_progress": true, "completed": true, "failed": true, "skipped": true}
		if !validStates[taskStatus.Attributes.State] {
			t.Fatalf("invalid task status state: %s", taskStatus.Attributes.State)
		}
	}
	t.Logf("validated %d task statuses with steps like: %s", len(taskStatuses.Data), taskStatuses.Data[0].Attributes.Step)

	// Test tracking config
	t.Logf("testing tracking config: repo_id=%s", repoID)
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/tracking-config", nil)
	assertStatus(t, resp, http.StatusOK)
	trackingConfig := decodeJSON[trackingConfigResponse](t, resp.Body)
	t.Logf("tracking config: branch=%s", trackingConfig.Data.Attributes.Branch)

	// Test tracking config update
	trackingUpdatePayload := trackingConfigUpdateRequest{
		Data: trackingConfigData{
			Type: "tracking_config",
			Attributes: trackingConfigAttributes{
				Branch: trackingConfig.Data.Attributes.Branch,
			},
		},
	}
	resp = doRequest(t, client, "PUT", baseURL+"/api/v1/repositories/"+repoID+"/tracking-config", trackingUpdatePayload)
	assertStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	t.Log("tracking config update successful")

	// Test tags
	t.Logf("testing tags: repo_id=%s", repoID)
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/tags", nil)
	assertStatus(t, resp, http.StatusOK)
	tags := decodeJSON[tagListResponse](t, resp.Body)
	t.Logf("retrieved tags: count=%d", len(tags.Data))
	if len(tags.Data) > 0 {
		tagID := tags.Data[0].ID
		resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/tags/"+tagID, nil)
		assertStatus(t, resp, http.StatusOK)
		_ = resp.Body.Close()
		t.Logf("retrieved tag details: tag_id=%s", tagID)
	}

	// Test 404 for non-existent commit
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/commits/nonexistent123", nil)
	assertStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
	t.Log("correctly returns 404 for non-existent commit")

	// Test commits
	t.Logf("testing commits: repo_id=%s", repoID)
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/commits", nil)
	assertStatus(t, resp, http.StatusOK)
	commits := decodeJSON[commitListResponse](t, resp.Body)
	t.Logf("retrieved commits: count=%d", len(commits.Data))
	if len(commits.Data) == 0 {
		t.Fatal("expected at least one commit")
	}

	// Validate commit data
	commit := commits.Data[0]
	if commit.Type != "commit" {
		t.Fatalf("expected commit type 'commit', got %s", commit.Type)
	}
	commitSHA := commit.Attributes.CommitSHA
	if commitSHA == "" {
		t.Fatal("expected commit SHA to be set")
	}
	if len(commitSHA) != 40 {
		t.Fatalf("expected 40-character commit SHA, got %d characters: %s", len(commitSHA), commitSHA)
	}
	if commit.Attributes.Author == "" {
		t.Fatal("expected commit author to be set")
	}
	if commit.Attributes.Date == "" {
		t.Fatal("expected commit date to be set")
	}
	t.Logf("commit data: sha=%s, author=%s, message=%s",
		commitSHA, commit.Attributes.Author, commit.Attributes.Message)

	commitURL := baseURL + "/api/v1/repositories/" + repoID + "/commits/" + commitSHA

	// Get commit by SHA
	resp = doRequest(t, client, "GET", commitURL, nil)
	assertStatus(t, resp, http.StatusOK)
	commitDetail := decodeJSON[commitResponse](t, resp.Body)
	if commitDetail.Data.Attributes.CommitSHA != commitSHA {
		t.Fatalf("expected commit SHA %s, got %s", commitSHA, commitDetail.Data.Attributes.CommitSHA)
	}
	t.Logf("retrieved commit details: sha=%s", commitSHA)

	// Test 404 for non-existent file
	resp = doRequest(t, client, "GET", commitURL+"/files/nonexistent123", nil)
	assertStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
	t.Log("correctly returns 404 for non-existent file")

	// Test files - verify we have files in the repository
	resp = doRequest(t, client, "GET", commitURL+"/files", nil)
	assertStatus(t, resp, http.StatusOK)
	files := decodeJSON[fileListResponse](t, resp.Body)
	t.Logf("retrieved commit files: count=%d", len(files.Data))
	if len(files.Data) == 0 {
		t.Fatal("expected at least one file in repository")
	}

	// Validate file data - test repo has main.go
	file := files.Data[0]
	blobSHA := file.Attributes.BlobSHA
	filePath := file.Attributes.Path
	if blobSHA == "" {
		t.Fatal("expected file to have blob SHA")
	}
	if len(blobSHA) != 40 {
		t.Fatalf("expected 40-character blob SHA, got %d characters: %s", len(blobSHA), blobSHA)
	}
	if filePath == "" {
		t.Fatal("expected file to have path")
	}
	if !strings.HasSuffix(filePath, "main.go") {
		t.Fatalf("expected file path to end with main.go, got %s", filePath)
	}
	if file.Attributes.Extension != "go" && file.Attributes.Extension != ".go" {
		t.Fatalf("expected extension go or .go, got %s", file.Attributes.Extension)
	}
	if file.Attributes.Size <= 0 {
		t.Fatalf("expected file size > 0, got %d", file.Attributes.Size)
	}
	t.Logf("file data: path=%s, extension=%s, size=%d, mime=%s",
		filePath, file.Attributes.Extension, file.Attributes.Size, file.Attributes.MimeType)

	// Get file by blob SHA
	resp = doRequest(t, client, "GET", commitURL+"/files/"+blobSHA, nil)
	assertStatus(t, resp, http.StatusOK)
	fileResp := decodeJSON[fileResponse](t, resp.Body)
	if fileResp.Data.Attributes.BlobSHA != blobSHA {
		t.Fatalf("expected blob SHA %s, got %s", blobSHA, fileResp.Data.Attributes.BlobSHA)
	}
	if fileResp.Data.Attributes.Size != file.Attributes.Size {
		t.Fatalf("expected size %d, got %d", file.Attributes.Size, fileResp.Data.Attributes.Size)
	}
	t.Logf("retrieved file metadata: path=%s, blob_sha=%s, size=%d",
		filePath, blobSHA, fileResp.Data.Attributes.Size)

	// Test snippets - should return actual data, not redirect
	resp = doRequest(t, client, "GET", commitURL+"/snippets", nil)
	assertStatus(t, resp, http.StatusOK)
	snippets := decodeJSON[snippetListResponse](t, resp.Body)
	t.Logf("retrieved snippets: count=%d", len(snippets.Data))
	if len(snippets.Data) == 0 {
		t.Fatal("expected at least one snippet (code was sliced)")
	}

	// Validate snippet data
	snippet := snippets.Data[0]
	if snippet.ID == "" {
		t.Fatal("expected snippet to have ID")
	}
	if snippet.Attributes.Content.Value == "" {
		t.Fatal("expected snippet to have content value")
	}
	if snippet.Attributes.Content.Language == "" {
		t.Fatal("expected snippet to have language")
	}
	// The test repo is Go code, so snippets should be Go
	if snippet.Attributes.Content.Language != "go" {
		t.Fatalf("expected snippet language 'go', got %s", snippet.Attributes.Content.Language)
	}
	// Snippet should contain Go code
	if !strings.Contains(snippet.Attributes.Content.Value, "package") &&
		!strings.Contains(snippet.Attributes.Content.Value, "func") &&
		!strings.Contains(snippet.Attributes.Content.Value, "import") {
		t.Fatalf("expected snippet to contain Go code, got: %s", snippet.Attributes.Content.Value[:min(100, len(snippet.Attributes.Content.Value))])
	}
	t.Logf("snippet data: id=%s, language=%s, content_length=%d",
		snippet.ID, snippet.Attributes.Content.Language, len(snippet.Attributes.Content.Value))

	// Test commit enrichments endpoint
	resp = doRequest(t, client, "GET", commitURL+"/enrichments", nil)
	assertStatus(t, resp, http.StatusOK)
	commitEnrichments := decodeJSON[enrichmentListResponse](t, resp.Body)
	t.Logf("retrieved commit enrichments: count=%d", len(commitEnrichments.Data))

	// Enrichments are required - fail if none found
	if len(commitEnrichments.Data) == 0 {
		t.Fatal("expected at least one enrichment - ensure LLM provider is configured")
	}

	// Verify enrichment has expected fields
	firstEnrichment := commitEnrichments.Data[0]
	enrichmentID := firstEnrichment.ID
	if firstEnrichment.Attributes.Type == "" {
		t.Fatal("enrichment should have a type")
	}
	t.Logf("first enrichment: id=%s, type=%s, subtype=%s",
		enrichmentID, firstEnrichment.Attributes.Type, firstEnrichment.Attributes.Subtype)

	// Test get specific enrichment by ID within commit context
	resp = doRequest(t, client, "GET", commitURL+"/enrichments/"+enrichmentID, nil)
	assertStatus(t, resp, http.StatusOK)
	singleEnrichment := decodeJSON[enrichmentResponse](t, resp.Body)
	if singleEnrichment.Data.ID != enrichmentID {
		t.Fatalf("expected enrichment ID %s, got %s", enrichmentID, singleEnrichment.Data.ID)
	}
	t.Logf("retrieved single enrichment: id=%s", enrichmentID)

	// Test repository-level enrichments endpoint
	t.Logf("testing repository enrichments: repo_id=%s", repoID)
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/enrichments", nil)
	assertStatus(t, resp, http.StatusOK)
	repoEnrichments := decodeJSON[enrichmentListResponse](t, resp.Body)
	t.Logf("retrieved repository enrichments: count=%d", len(repoEnrichments.Data))
	if len(repoEnrichments.Data) == 0 {
		t.Fatal("expected at least one repository enrichment")
	}

	// Test global enrichments endpoint
	// Note: global endpoint may return empty if all enrichments are repo-scoped
	t.Log("testing global enrichments endpoint...")
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/enrichments", nil)
	assertStatus(t, resp, http.StatusOK)
	globalEnrichments := decodeJSON[enrichmentListResponse](t, resp.Body)
	t.Logf("retrieved global enrichments: count=%d", len(globalEnrichments.Data))

	// Test get global enrichment by ID if any exist
	if len(globalEnrichments.Data) > 0 {
		globalEnrichmentID := globalEnrichments.Data[0].ID
		resp = doRequest(t, client, "GET", baseURL+"/api/v1/enrichments/"+globalEnrichmentID, nil)
		assertStatus(t, resp, http.StatusOK)
		_ = resp.Body.Close()
		t.Logf("retrieved global enrichment by ID: %s", globalEnrichmentID)
	}

	// Test embeddings endpoint is deprecated
	resp = doRequest(t, client, "GET", commitURL+"/embeddings", nil)
	assertStatus(t, resp, http.StatusGone)
	_ = resp.Body.Close()
	t.Log("embeddings endpoint correctly returns 410 Gone")

	// Test search API - Keywords mode
	t.Log("testing search API - keywords mode...")
	keywordsPayload := searchRequest{
		Data: searchData{
			Type: "search",
			Attributes: searchRequestAttributes{
				Keywords: []string{"main", "func", "package"},
				Limit:    intPtr(10),
			},
		},
	}
	resp = doRequest(t, client, "POST", baseURL+"/api/v1/search", keywordsPayload)
	assertStatus(t, resp, http.StatusOK)
	keywordsResp := decodeJSON[searchResponse](t, resp.Body)
	t.Logf("keywords search completed: results=%d", len(keywordsResp.Data))
	if len(keywordsResp.Data) == 0 {
		t.Fatal("keywords search: expected at least one result")
	}
	validateSearchResults(t, keywordsResp.Data, "keywords")

	// Test search API - Text mode (for searching enrichments/descriptions)
	// Note: text search uses vector embeddings on enrichment summaries
	t.Log("testing search API - text mode...")
	textPayload := searchRequest{
		Data: searchData{
			Type: "search",
			Attributes: searchRequestAttributes{
				Text:  strPtr("hello world greeting function"),
				Limit: intPtr(10),
			},
		},
	}
	resp = doRequest(t, client, "POST", baseURL+"/api/v1/search", textPayload)
	assertStatus(t, resp, http.StatusOK)
	textResp := decodeJSON[searchResponse](t, resp.Body)
	t.Logf("text search completed: results=%d", len(textResp.Data))
	if len(textResp.Data) == 0 {
		t.Log("text search returned no results (enrichment summaries may not be indexed yet)")
	} else {
		validateSearchResults(t, textResp.Data, "text")
	}

	// Test search API - Code mode (for searching code snippets)
	t.Log("testing search API - code mode...")
	codePayload := searchRequest{
		Data: searchData{
			Type: "search",
			Attributes: searchRequestAttributes{
				Code:  strPtr("func main() { fmt.Println }"),
				Limit: intPtr(10),
			},
		},
	}
	resp = doRequest(t, client, "POST", baseURL+"/api/v1/search", codePayload)
	assertStatus(t, resp, http.StatusOK)
	codeResp := decodeJSON[searchResponse](t, resp.Body)
	t.Logf("code search completed: results=%d", len(codeResp.Data))
	if len(codeResp.Data) == 0 {
		t.Fatal("code search: expected at least one result")
	}
	validateSearchResults(t, codeResp.Data, "code")

	// Test search API - Combined mode (keywords + code + text)
	t.Log("testing search API - combined mode...")
	combinedPayload := searchRequest{
		Data: searchData{
			Type: "search",
			Attributes: searchRequestAttributes{
				Keywords: []string{"main", "hello"},
				Text:     strPtr("greeting program"),
				Code:     strPtr("package main"),
				Limit:    intPtr(10),
			},
		},
	}
	resp = doRequest(t, client, "POST", baseURL+"/api/v1/search", combinedPayload)
	assertStatus(t, resp, http.StatusOK)
	combinedResp := decodeJSON[searchResponse](t, resp.Body)
	t.Logf("combined search completed: results=%d", len(combinedResp.Data))
	if len(combinedResp.Data) == 0 {
		t.Fatal("combined search: expected at least one result")
	}
	validateSearchResults(t, combinedResp.Data, "combined")

	// Test 404 for non-existent queue task
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/queue/99999", nil)
	assertStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
	t.Log("correctly returns 404 for non-existent queue task")

	// Test queue API
	t.Log("testing queue API...")
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/queue", nil)
	assertStatus(t, resp, http.StatusOK)
	queueResp := decodeJSON[queueListResponse](t, resp.Body)
	t.Logf("queue contains %d tasks", len(queueResp.Data))

	// Validate queue task data
	if len(queueResp.Data) > 0 {
		task := queueResp.Data[0]
		if task.Type != "task" {
			t.Fatalf("expected task type 'task', got %s", task.Type)
		}
		if task.ID == "" {
			t.Fatal("expected task to have ID")
		}
		if task.Attributes.Type == "" {
			t.Fatal("expected task to have type attribute")
		}
		// Verify task type starts with known prefix
		if !strings.HasPrefix(task.Attributes.Type, "kodit.") {
			t.Fatalf("expected task type to start with 'kodit.', got %s", task.Attributes.Type)
		}
		t.Logf("task data: id=%s, type=%s, priority=%d",
			task.ID, task.Attributes.Type, task.Attributes.Priority)

		// Get specific task by ID
		resp = doRequest(t, client, "GET", baseURL+"/api/v1/queue/"+task.ID, nil)
		assertStatus(t, resp, http.StatusOK)
		taskDetail := decodeJSON[queueTaskResponse](t, resp.Body)
		if taskDetail.Data.ID != task.ID {
			t.Fatalf("expected task ID %s, got %s", task.ID, taskDetail.Data.ID)
		}
		if taskDetail.Data.Attributes.Type != task.Attributes.Type {
			t.Fatalf("expected type %s, got %s", task.Attributes.Type, taskDetail.Data.Attributes.Type)
		}
		t.Logf("retrieved queue task detail: id=%s", task.ID)
	}

	// Test rescan endpoint
	t.Logf("testing rescan: repo_id=%s, commit_sha=%s", repoID, commitSHA)
	resp = doRequest(t, client, "POST", commitURL+"/rescan", nil)
	assertStatus(t, resp, http.StatusAccepted)
	_ = resp.Body.Close()
	t.Log("rescan triggered successfully")

	// Wait for rescan to create tasks
	time.Sleep(2 * time.Second)

	// Check that rescan created new queue tasks
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/queue", nil)
	assertStatus(t, resp, http.StatusOK)
	queueAfterRescan := decodeJSON[queueListResponse](t, resp.Body)
	t.Logf("queue after rescan: %d tasks", len(queueAfterRescan.Data))

	// Wait for rescan tasks to complete
	t.Log("waiting for rescan to complete...")
	rescanDone := waitForCondition(t, 5*time.Minute, time.Second, func() bool {
		resp := doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/status", nil)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return false
		}

		status := decodeJSON[taskStatusListResponse](t, resp.Body)
		terminalStates := map[string]bool{"completed": true, "skipped": true, "failed": true}
		for _, task := range status.Data {
			if !terminalStates[task.Attributes.State] {
				return false
			}
		}
		return true
	})
	if !rescanDone {
		t.Fatal("rescan did not complete within timeout")
	}
	t.Log("rescan completed")

	// Test repository deletion
	t.Logf("testing repository deletion: repo_id=%s", repoID)
	resp = doRequest(t, client, "DELETE", baseURL+"/api/v1/repositories/"+repoID, nil)
	assertStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
	t.Logf("repository delete requested: repo_id=%s", repoID)

	// Wait for repository deletion to complete (deletion is async)
	// Allow up to 2 minutes for background tasks to complete before deletion finishes
	t.Log("waiting for repository deletion to complete...")
	deletionDone := waitForCondition(t, 2*time.Minute, 500*time.Millisecond, func() bool {
		resp := doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID, nil)
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode == http.StatusNotFound
	})
	if !deletionDone {
		t.Fatal("repository deletion did not complete within timeout")
	}
	t.Logf("repository deleted successfully: repo_id=%s", repoID)

	t.Log("all smoke tests passed successfully")
}

// findCmdDir locates the cmd/kodit directory.
func findCmdDir(t *testing.T) string {
	t.Helper()

	// Try relative paths from current working directory
	candidates := []string{
		"./cmd/kodit",
		"../../cmd/kodit",
		"../../../cmd/kodit",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Try to find from module root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Walk up looking for go.mod
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			cmdDir := filepath.Join(dir, "cmd", "kodit")
			if _, err := os.Stat(cmdDir); err == nil {
				return cmdDir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatal("could not find cmd/kodit directory")
	return ""
}

// doRequest performs an HTTP request and returns the response.
func doRequest(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	return resp
}

// assertStatus checks that the response has the expected status code.
func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d: %s", expected, resp.StatusCode, string(body))
	}
}

// decodeJSON decodes a JSON response body.
func decodeJSON[T any](t *testing.T, body io.ReadCloser) T {
	t.Helper()
	defer func() { _ = body.Close() }()

	var result T
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	return result
}

// Helper functions
func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// validateSearchResults validates the structure of search results.
func validateSearchResults(t *testing.T, results []searchResultData, mode string) {
	t.Helper()
	for i, result := range results {
		if result.ID == "" {
			t.Fatalf("%s search result %d: expected ID to be set", mode, i)
		}
		if result.Type != "snippet" {
			t.Fatalf("%s search result %d: expected type 'snippet', got %s", mode, i, result.Type)
		}
		// Validate derives_from field is present and has entries
		if result.Attributes.DerivesFrom == nil {
			t.Fatalf("%s search result %d: expected derives_from field to be present", mode, i)
		}
		if len(result.Attributes.DerivesFrom) == 0 {
			t.Fatalf("%s search result %d: expected derives_from to have at least one entry", mode, i)
		}
		// Validate derives_from entries have required fields
		for j, derivesFrom := range result.Attributes.DerivesFrom {
			if derivesFrom.BlobSHA == "" {
				t.Fatalf("%s search result %d derives_from %d: expected blob_sha to be set", mode, i, j)
			}
			if derivesFrom.Path == "" {
				t.Fatalf("%s search result %d derives_from %d: expected path to be set", mode, i, j)
			}
		}
		// Validate enrichments field is present and has entries
		if result.Attributes.Enrichments == nil {
			t.Fatalf("%s search result %d: expected enrichments field to be present", mode, i)
		}
		if len(result.Attributes.Enrichments) == 0 {
			t.Fatalf("%s search result %d: expected enrichments to have at least one entry", mode, i)
		}
		// Validate enrichment entries have required fields
		for j, enrichment := range result.Attributes.Enrichments {
			if enrichment.Type == "" {
				t.Fatalf("%s search result %d enrichment %d: expected type to be set", mode, i, j)
			}
			if enrichment.Content == "" {
				t.Fatalf("%s search result %d enrichment %d: expected content to be set", mode, i, j)
			}
		}
		// Validate content is set
		if result.Attributes.Content.Value == "" {
			t.Fatalf("%s search result %d: expected content value to be set", mode, i)
		}
		if result.Attributes.Content.Language == "" {
			t.Fatalf("%s search result %d: expected content language to be set", mode, i)
		}
		// Validate links
		if result.Links == nil {
			t.Fatalf("%s search result %d: expected links to be present", mode, i)
		}
		if !strings.HasPrefix(result.Links.Repository, "/api/v1/repositories/") {
			t.Fatalf("%s search result %d: expected repository link to start with /api/v1/repositories/, got %s", mode, i, result.Links.Repository)
		}
		if !strings.Contains(result.Links.Commit, "/commits/") {
			t.Fatalf("%s search result %d: expected commit link to contain /commits/, got %s", mode, i, result.Links.Commit)
		}
		if !strings.Contains(result.Links.File, "/files/") {
			t.Fatalf("%s search result %d: expected file link to contain /files/, got %s", mode, i, result.Links.File)
		}
		t.Logf("%s search result %d: id=%s, derives_from=%d files, enrichments=%d, language=%s, links=%+v",
			mode, i, result.ID, len(result.Attributes.DerivesFrom), len(result.Attributes.Enrichments), result.Attributes.Content.Language, result.Links)
	}
}

// Request/Response types (minimal subset needed for smoke tests)

type healthResponse struct {
	Status string `json:"status"`
}

type repositoryCreateAttributes struct {
	RemoteURI string `json:"remote_uri"`
}

type repositoryCreateData struct {
	Type       string                     `json:"type"`
	Attributes repositoryCreateAttributes `json:"attributes"`
}

type repositoryCreateRequest struct {
	Data repositoryCreateData `json:"data"`
}

type repositoryAttributes struct {
	RemoteURI   string `json:"remote_uri"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	ClonedPath  string `json:"cloned_path"`
	NumCommits  int    `json:"num_commits"`
	NumBranches int    `json:"num_branches"`
	NumTags     int    `json:"num_tags"`
}

type repositoryData struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Attributes repositoryAttributes `json:"attributes"`
}

type repositoryResponse struct {
	Data repositoryData `json:"data"`
}

type repositoryListResponse struct {
	Data []repositoryData `json:"data"`
}

type taskStatusAttributes struct {
	Step      string  `json:"step"`
	State     string  `json:"state"`
	Progress  float64 `json:"progress"`
	Total     int     `json:"total"`
	Current   int     `json:"current"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	Error     string  `json:"error"`
	Message   string  `json:"message"`
}

type taskStatusData struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Attributes taskStatusAttributes `json:"attributes"`
}

type taskStatusListResponse struct {
	Data []taskStatusData `json:"data"`
}

type statusSummaryAttributes struct {
	TotalTasks     int `json:"total_tasks"`
	CompletedTasks int `json:"completed_tasks"`
	FailedTasks    int `json:"failed_tasks"`
	PendingTasks   int `json:"pending_tasks"`
	RunningTasks   int `json:"running_tasks"`
}

type statusSummaryData struct {
	Type       string                  `json:"type"`
	Attributes statusSummaryAttributes `json:"attributes"`
}

type statusSummaryResponse struct {
	Data statusSummaryData `json:"data"`
}

type trackingConfigAttributes struct {
	Branch string `json:"branch"`
}

type trackingConfigData struct {
	Type       string                   `json:"type"`
	Attributes trackingConfigAttributes `json:"attributes"`
}

type trackingConfigResponse struct {
	Data trackingConfigData `json:"data"`
}

type trackingConfigUpdateRequest struct {
	Data trackingConfigData `json:"data"`
}

type tagData struct {
	ID string `json:"id"`
}

type tagListResponse struct {
	Data []tagData `json:"data"`
}

type commitAttributes struct {
	CommitSHA       string `json:"commit_sha"`
	Author          string `json:"author"`
	Date            string `json:"date"`
	Message         string `json:"message"`
	ParentCommitSHA string `json:"parent_commit_sha"`
}

type commitData struct {
	Type       string           `json:"type"`
	ID         string           `json:"id"`
	Attributes commitAttributes `json:"attributes"`
}

type commitResponse struct {
	Data commitData `json:"data"`
}

type commitListResponse struct {
	Data []commitData `json:"data"`
}

type fileAttributes struct {
	BlobSHA   string `json:"blob_sha"`
	Path      string `json:"path"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
}

type fileData struct {
	Attributes fileAttributes `json:"attributes"`
}

type fileListResponse struct {
	Data []fileData `json:"data"`
}

type fileResponse struct {
	Data fileData `json:"data"`
}

type snippetContent struct {
	Value    string `json:"value"`
	Language string `json:"language"`
}

type snippetAttributes struct {
	Content snippetContent `json:"content"`
}

type snippetData struct {
	ID         string            `json:"id"`
	Attributes snippetAttributes `json:"attributes"`
}

type snippetListResponse struct {
	Data []snippetData `json:"data"`
}

type enrichmentAttributes struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Content string `json:"content"`
}

type enrichmentData struct {
	ID         string               `json:"id"`
	Attributes enrichmentAttributes `json:"attributes"`
}

type enrichmentListResponse struct {
	Data []enrichmentData `json:"data"`
}

type enrichmentResponse struct {
	Data enrichmentData `json:"data"`
}

type searchRequestAttributes struct {
	Keywords []string `json:"keywords,omitempty"`
	Text     *string  `json:"text,omitempty"`
	Code     *string  `json:"code,omitempty"`
	Limit    *int     `json:"limit,omitempty"`
}

type searchData struct {
	Type       string                  `json:"type"`
	Attributes searchRequestAttributes `json:"attributes"`
}

type searchRequest struct {
	Data searchData `json:"data"`
}

type searchResultGitFile struct {
	BlobSHA  string `json:"blob_sha"`
	Path     string `json:"path"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

type searchResultContent struct {
	Value    string `json:"value"`
	Language string `json:"language"`
}

type searchResultEnrichment struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type searchResultAttributes struct {
	DerivesFrom    []searchResultGitFile    `json:"derives_from"`
	Content        searchResultContent      `json:"content"`
	Enrichments    []searchResultEnrichment `json:"enrichments"`
	OriginalScores []float64                `json:"original_scores"`
}

type searchResultLinks struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	File       string `json:"file"`
}

type searchResultData struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Attributes searchResultAttributes `json:"attributes"`
	Links      *searchResultLinks     `json:"links,omitempty"`
}

type searchResponse struct {
	Data []searchResultData `json:"data"`
}

type queueTaskAttributes struct {
	Type      string `json:"type"`
	Priority  int    `json:"priority"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type queueTaskData struct {
	Type       string              `json:"type"`
	ID         string              `json:"id"`
	Attributes queueTaskAttributes `json:"attributes"`
}

type queueTaskResponse struct {
	Data queueTaskData `json:"data"`
}

type queueListResponse struct {
	Data []queueTaskData `json:"data"`
}

// TestSmoke_MigrationFromDump verifies that kodit can start against a
// Python-era PostgreSQL database dump. This proves AutoMigrate handles
// the schema transition without errors.
//
// Requires SMOKE_DB_URL pointing to a VectorChord instance pre-loaded with
// testdata/kodit_dump.sql (see: make smoke-postgres).
func TestSmoke_MigrationFromDump(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}

	smokeDBURL := os.Getenv("SMOKE_DB_URL")
	if smokeDBURL == "" {
		t.Skip("SMOKE_DB_URL not set — run 'make smoke-postgres' to execute this test")
	}

	if !portAvailable(baseHost, basePort) {
		t.Fatalf("port %d is already in use", basePort)
	}

	// Create temp env file
	tmpEnv, err := os.CreateTemp("", "smoke-migration-env-*")
	if err != nil {
		t.Fatalf("create temp env: %v", err)
	}
	tmpEnvPath := tmpEnv.Name()
	_ = tmpEnv.Close()
	defer func() { _ = os.Remove(tmpEnvPath) }()

	cmdDir := findCmdDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	buildTags := os.Getenv("SMOKE_BUILD_TAGS")
	if buildTags == "" {
		buildTags = "fts5 ORT embed_model"
	}
	cmd := exec.CommandContext(ctx, "go", "run", "-tags="+buildTags, cmdDir,
		"serve",
		"--host", baseHost,
		"--port", strconv.Itoa(basePort),
		"--env-file", tmpEnvPath,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append(os.Environ(),
		"DISABLE_TELEMETRY=true",
		"DB_URL="+smokeDBURL,
		"SKIP_PROVIDER_VALIDATION=true",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		cancel()
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("server stdout:\n%s", stdout.String())
			t.Logf("server stderr:\n%s", stderr.String())
		}
	}()

	client := &http.Client{Timeout: 30 * time.Second}

	// Wait for server — if migration fails the process exits and the port
	// never opens, so this correctly times out.
	t.Log("waiting for server to start (migration + startup)...")
	started := waitForCondition(t, 60*time.Second, time.Second, func() bool {
		return !portAvailable(baseHost, basePort)
	})
	if !started {
		t.Fatal("server failed to start — migration likely failed (check stderr above)")
	}

	// Health check proves migration succeeded and server is running.
	t.Log("verifying health endpoint...")
	resp := doRequest(t, client, "GET", baseURL+"/healthz", nil)
	assertStatus(t, resp, http.StatusOK)
	health := decodeJSON[healthResponse](t, resp.Body)
	if health.Status != "healthy" {
		t.Fatalf("expected healthy, got %s", health.Status)
	}

	// Verify migrated data is accessible.
	t.Log("verifying repositories from dump are accessible...")
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories", nil)
	assertStatus(t, resp, http.StatusOK)
	repos := decodeJSON[repositoryListResponse](t, resp.Body)
	if len(repos.Data) == 0 {
		t.Fatal("expected at least one repository from the dump")
	}
	t.Logf("migration smoke test passed: %d repositories accessible", len(repos.Data))
}
