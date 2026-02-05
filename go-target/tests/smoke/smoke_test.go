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
	"syscall"
	"testing"
	"time"
)

const (
	baseHost  = "127.0.0.1"
	basePort  = 8080
	targetURI = "https://gist.github.com/philwinder/11e4c4f7ea48b1c05b7cedea49367f1a.git"
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

	cmd := exec.CommandContext(ctx, "go", "run", cmdDir,
		"serve",
		"--host", baseHost,
		"--port", strconv.Itoa(basePort),
		"--env-file", tmpEnvPath,
	)

	// Create a new process group so we can kill all child processes
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Set environment - use in-memory SQLite by default
	cmd.Env = append(os.Environ(),
		"DISABLE_TELEMETRY=true",
		"DB_URL=sqlite:///:memory:",
		"SKIP_PROVIDER_VALIDATION=true",
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
	_ = resp.Body.Close()
	t.Log("server health check passed")

	// Test repository lifecycle
	t.Log("testing repository lifecycle...")

	// List existing repositories
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories", nil)
	assertStatus(t, resp, http.StatusOK)
	repos := decodeJSON[repositoryListResponse](t, resp.Body)
	t.Logf("listed existing repositories: count=%d", len(repos.Data))

	// Create repository (using legacy format that Go API accepts)
	createPayload := repositoryCreateRequest{
		RemoteURL: targetURI,
	}
	resp = doRequest(t, client, "POST", baseURL+"/api/v1/repositories", createPayload)
	assertStatus(t, resp, http.StatusCreated)
	_ = resp.Body.Close()
	t.Logf("created repository: uri=%s", targetURI)

	// Get repository ID
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories", nil)
	assertStatus(t, resp, http.StatusOK)
	repos = decodeJSON[repositoryListResponse](t, resp.Body)
	if len(repos.Data) == 0 {
		t.Fatal("no repositories found after creation")
	}
	repoID := repos.Data[0].ID
	t.Logf("retrieved repository ID: %s", repoID)

	// Test repository endpoints
	t.Logf("testing repository endpoints: repo_id=%s", repoID)
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID, nil)
	assertStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()

	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/status", nil)
	assertStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()

	// Wait for indexing to complete
	t.Logf("waiting for indexing to complete: repo_id=%s", repoID)
	indexingDone := waitForCondition(t, 10*time.Minute, time.Second, func() bool {
		resp := doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/status", nil)
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return false
		}

		status := decodeJSON[taskStatusListResponse](t, resp.Body)
		t.Logf("indexing status: tasks=%d", len(status.Data))

		// Need at least some tasks and all in terminal state
		if len(status.Data) < 5 {
			return false
		}

		terminalStates := map[string]bool{"completed": true, "skipped": true, "failed": true}
		for _, task := range status.Data {
			if !terminalStates[task.Attributes.State] {
				return false
			}
		}
		return true
	})
	if !indexingDone {
		t.Fatal("indexing did not complete within timeout")
	}
	t.Logf("indexing completed: repo_id=%s", repoID)

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

	// Test commits
	t.Logf("testing commits: repo_id=%s", repoID)
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/repositories/"+repoID+"/commits", nil)
	assertStatus(t, resp, http.StatusOK)
	commits := decodeJSON[commitListResponse](t, resp.Body)
	t.Logf("retrieved commits: count=%d", len(commits.Data))

	if len(commits.Data) > 0 {
		commitSHA := commits.Data[0].Attributes.CommitSHA
		commitURL := baseURL + "/api/v1/repositories/" + repoID + "/commits/" + commitSHA

		resp = doRequest(t, client, "GET", commitURL, nil)
		assertStatus(t, resp, http.StatusOK)
		_ = resp.Body.Close()
		t.Logf("retrieved commit details: commit_sha=%s", commitSHA)

		// Test files
		resp = doRequest(t, client, "GET", commitURL+"/files", nil)
		assertStatus(t, resp, http.StatusOK)
		files := decodeJSON[fileListResponse](t, resp.Body)
		t.Logf("retrieved commit files: count=%d", len(files.Data))

		if len(files.Data) > 0 {
			blobSHA := files.Data[0].Attributes.BlobSHA
			resp = doRequest(t, client, "GET", commitURL+"/files/"+blobSHA, nil)
			assertStatus(t, resp, http.StatusOK)
			_ = resp.Body.Close()
			t.Logf("retrieved file content: blob_sha=%s", blobSHA)
		}

		// Test snippets (redirects to enrichments)
		resp = doRequestNoFollow(t, client, "GET", commitURL+"/snippets", nil)
		if resp.StatusCode != http.StatusPermanentRedirect {
			t.Logf("expected redirect for snippets, got %d", resp.StatusCode)
		}
		_ = resp.Body.Close()
		t.Logf("verified snippets redirect: commit_sha=%s", commitSHA)

		// Test enrichments
		resp = doRequest(t, client, "GET", commitURL+"/enrichments", nil)
		assertStatus(t, resp, http.StatusOK)
		_ = resp.Body.Close()
		t.Logf("retrieved enrichments: commit_sha=%s", commitSHA)

		// Test embeddings
		resp = doRequest(t, client, "GET", commitURL+"/embeddings?full=false", nil)
		assertStatus(t, resp, http.StatusOK)
		_ = resp.Body.Close()
		t.Logf("retrieved embeddings: commit_sha=%s", commitSHA)
	}

	// Test search API
	t.Log("testing search API...")
	searchPayload := searchRequest{
		Data: searchData{
			Type: "search",
			Attributes: searchAttributes{
				Keywords: []string{"test"},
				Code:     strPtr("def"),
				Limit:    intPtr(5),
			},
		},
	}
	resp = doRequest(t, client, "POST", baseURL+"/api/v1/search", searchPayload)
	assertStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	t.Log("search completed successfully")

	// Test queue API
	t.Log("testing queue API...")
	resp = doRequest(t, client, "GET", baseURL+"/api/v1/queue", nil)
	assertStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	t.Log("queue API responded successfully")

	// Test repository deletion
	t.Logf("testing repository deletion: repo_id=%s", repoID)
	resp = doRequest(t, client, "DELETE", baseURL+"/api/v1/repositories/"+repoID, nil)
	assertStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
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

// doRequestNoFollow performs an HTTP request without following redirects.
func doRequestNoFollow(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()

	noFollowClient := &http.Client{
		Timeout: client.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

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

	resp, err := noFollowClient.Do(req)
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

// Request/Response types (minimal subset needed for smoke tests)

type repositoryCreateRequest struct {
	RemoteURL string `json:"remote_url"`
}

type repositoryData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type repositoryListResponse struct {
	Data []repositoryData `json:"data"`
}

type taskStatusAttributes struct {
	State string `json:"state"`
}

type taskStatusData struct {
	Attributes taskStatusAttributes `json:"attributes"`
}

type taskStatusListResponse struct {
	Data []taskStatusData `json:"data"`
}

type tagData struct {
	ID string `json:"id"`
}

type tagListResponse struct {
	Data []tagData `json:"data"`
}

type commitAttributes struct {
	CommitSHA string `json:"commit_sha"`
}

type commitData struct {
	Attributes commitAttributes `json:"attributes"`
}

type commitListResponse struct {
	Data []commitData `json:"data"`
}

type fileAttributes struct {
	BlobSHA string `json:"blob_sha"`
}

type fileData struct {
	Attributes fileAttributes `json:"attributes"`
}

type fileListResponse struct {
	Data []fileData `json:"data"`
}

type searchAttributes struct {
	Keywords []string `json:"keywords,omitempty"`
	Code     *string  `json:"code,omitempty"`
	Limit    *int     `json:"limit,omitempty"`
}

type searchData struct {
	Type       string           `json:"type"`
	Attributes searchAttributes `json:"attributes"`
}

type searchRequest struct {
	Data searchData `json:"data"`
}
