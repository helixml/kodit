package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

func TestRepositories_List_Empty(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/repositories")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.RepositoryListResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestRepositories_List_WithData(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository directly in the database
	ts.CreateRepository("https://github.com/test/repo.git")

	resp := ts.GET("/api/v1/repositories")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.RepositoryListResponse
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 1 {
		t.Errorf("len(data) = %d, want 1", len(result.Data))
	}
	if result.Data[0].Type != "repository" {
		t.Errorf("type = %q, want %q", result.Data[0].Type, "repository")
	}
	if result.Data[0].Attributes.RemoteURI != "https://github.com/test/repo.git" {
		t.Errorf("remote_uri = %q, want %q", result.Data[0].Attributes.RemoteURI, "https://github.com/test/repo.git")
	}
}

func TestRepositories_Create(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.RepositoryRequest{
		RemoteURL: "https://github.com/test/new-repo.git",
	}

	resp := ts.POST("/api/v1/repositories", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var result dto.RepositoryResponse
	ts.DecodeJSON(resp, &result)

	if result.Data.ID == "" {
		t.Error("ID should not be empty")
	}
	if result.Data.Attributes.RemoteURI != "https://github.com/test/new-repo.git" {
		t.Errorf("remote_uri = %q, want %q", result.Data.Attributes.RemoteURI, "https://github.com/test/new-repo.git")
	}
}

func TestRepositories_Create_WithTracking(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.RepositoryRequest{
		RemoteURL: "https://github.com/test/tracked-repo.git",
		Branch:    "main",
	}

	resp := ts.POST("/api/v1/repositories", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var result dto.RepositoryResponse
	ts.DecodeJSON(resp, &result)

	if result.Data.Attributes.TrackingBranch == nil || *result.Data.Attributes.TrackingBranch != "main" {
		var actual string
		if result.Data.Attributes.TrackingBranch != nil {
			actual = *result.Data.Attributes.TrackingBranch
		}
		t.Errorf("tracking_branch = %q, want %q", actual, "main")
	}
}

func TestRepositories_Create_MissingURL(t *testing.T) {
	ts := NewTestServer(t)

	body := dto.RepositoryRequest{
		RemoteURL: "",
	}

	resp := ts.POST("/api/v1/repositories", body)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRepositories_Get(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository
	repo := ts.CreateRepository("https://github.com/test/get-repo.git")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d", repo.ID()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.RepositoryResponse
	ts.DecodeJSON(resp, &result)

	if result.Data.ID != fmt.Sprintf("%d", repo.ID()) {
		t.Errorf("ID = %s, want %d", result.Data.ID, repo.ID())
	}
	if result.Data.Attributes.RemoteURI != "https://github.com/test/get-repo.git" {
		t.Errorf("remote_uri = %q, want %q", result.Data.Attributes.RemoteURI, "https://github.com/test/get-repo.git")
	}
}

func TestRepositories_Get_NotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/repositories/99999")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRepositories_Delete(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository
	repo := ts.CreateRepository("https://github.com/test/delete-repo.git")

	resp := ts.DELETE(fmt.Sprintf("/api/v1/repositories/%d", repo.ID()))
	defer func() {
		_ = resp.Body.Close()
	}()

	// Delete returns 204 No Content and queues a delete task
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestRepositories_Status(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository
	repo := ts.CreateRepository("https://github.com/test/status-repo.git")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/status", repo.ID()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.TaskStatusListResponse
	ts.DecodeJSON(resp, &result)

	// Should return empty list (no tracking service configured in test)
	if result.Data == nil {
		t.Error("data should not be nil")
	}
}

func TestRepositories_Status_NotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/repositories/99999/status")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRepositories_StatusSummary(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository
	repo := ts.CreateRepository("https://github.com/test/status-summary-repo.git")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/status/summary", repo.ID()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.RepositoryStatusSummaryResponse
	ts.DecodeJSON(resp, &result)

	// Should return pending status (no tracking service configured in test)
	if result.Data.Type != "repository_status_summary" {
		t.Errorf("type = %q, want %q", result.Data.Type, "repository_status_summary")
	}
	if result.Data.Attributes.Status != "pending" {
		t.Errorf("status = %q, want %q", result.Data.Attributes.Status, "pending")
	}
}

func TestRepositories_StatusSummary_NotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/repositories/99999/status/summary")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRepositories_ListCommits_Empty(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository
	repo := ts.CreateRepository("https://github.com/test/commits-repo.git")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/commits", repo.ID()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.CommitJSONAPIListResponse
	ts.DecodeJSON(resp, &result)

	if result.Data == nil {
		t.Error("data should not be nil")
	}
	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestRepositories_ListCommits_NotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/repositories/99999/commits")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRepositories_GetCommit_NotFound(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository
	repo := ts.CreateRepository("https://github.com/test/get-commit-repo.git")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/commits/abc123", repo.ID()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRepositories_GetCommit_RepoNotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/repositories/99999/commits/abc123")
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRepositories_GetCommitFile(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository, commit, and file
	repo := ts.CreateRepository("https://github.com/test/file-repo.git")
	commit := ts.CreateCommit(repo, "abc123def", "Test commit")
	file := ts.CreateFile(commit.SHA(), "src/main.go", "blob123abc", "text/x-go", ".go", 1024)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/commits/%s/files/%s", repo.ID(), commit.SHA(), file.BlobSHA()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.FileJSONAPIResponse
	ts.DecodeJSON(resp, &result)

	if result.Data.Type != "file" {
		t.Errorf("type = %q, want %q", result.Data.Type, "file")
	}
	if result.Data.ID != "blob123abc" {
		t.Errorf("ID = %q, want %q", result.Data.ID, "blob123abc")
	}
	if result.Data.Attributes.BlobSHA != "blob123abc" {
		t.Errorf("blob_sha = %q, want %q", result.Data.Attributes.BlobSHA, "blob123abc")
	}
	if result.Data.Attributes.Path != "src/main.go" {
		t.Errorf("path = %q, want %q", result.Data.Attributes.Path, "src/main.go")
	}
	if result.Data.Attributes.MimeType != "text/x-go" {
		t.Errorf("mime_type = %q, want %q", result.Data.Attributes.MimeType, "text/x-go")
	}
	if result.Data.Attributes.Size != 1024 {
		t.Errorf("size = %d, want %d", result.Data.Attributes.Size, 1024)
	}
	if result.Data.Attributes.Extension != ".go" {
		t.Errorf("extension = %q, want %q", result.Data.Attributes.Extension, ".go")
	}
}

func TestRepositories_GetCommitFile_NotFound(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository and commit, but no file
	repo := ts.CreateRepository("https://github.com/test/file-not-found-repo.git")
	commit := ts.CreateCommit(repo, "def456abc", "Test commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/commits/%s/files/nonexistent", repo.ID(), commit.SHA()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRepositories_GetCommitFile_CommitNotFound(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository but no commit
	repo := ts.CreateRepository("https://github.com/test/commit-not-found-repo.git")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/commits/nonexistent/files/blob123", repo.ID()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRepositories_ListCommitEnrichments_Empty(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository and commit
	repo := ts.CreateRepository("https://github.com/test/enrichments-repo.git")
	commit := ts.CreateCommit(repo, "enrichment123", "Test commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/commits/%s/enrichments", repo.ID(), commit.SHA()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result dto.EnrichmentJSONAPIListResponse
	ts.DecodeJSON(resp, &result)

	if result.Data == nil {
		t.Error("data should not be nil")
	}
	if len(result.Data) != 0 {
		t.Errorf("len(data) = %d, want 0", len(result.Data))
	}
}

func TestRepositories_ListCommitEnrichments_NotFound(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository but no commit
	repo := ts.CreateRepository("https://github.com/test/enrichments-not-found-repo.git")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/commits/nonexistent/enrichments", repo.ID()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRepositories_ListCommitSnippets(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository and commit
	repo := ts.CreateRepository("https://github.com/test/snippets-repo.git")
	commit := ts.CreateCommit(repo, "snippet123", "Test commit")

	// Create a snippet with content
	snippetContent := `func Hello() string {
	return "Hello, World!"
}`
	snippetSHA := "abc123def456"
	ts.CreateSnippet(snippetSHA, snippetContent, ".go")
	ts.CreateSnippetAssociation(snippetSHA, commit.SHA())

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/commits/%s/snippets", repo.ID(), commit.SHA()))
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result struct {
		Data []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Content struct {
					Value    string `json:"value"`
					Language string `json:"language"`
				} `json:"content"`
			} `json:"attributes"`
		} `json:"data"`
	}
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 1 {
		t.Errorf("expected 1 snippet, got %d", len(result.Data))
		return
	}

	// Verify snippet has content (the bug was snippets having empty content)
	snippet := result.Data[0]
	if snippet.Attributes.Content.Value == "" {
		t.Error("snippet content.value should not be empty")
	}
	if snippet.Attributes.Content.Value != snippetContent {
		t.Errorf("snippet content.value = %q, want %q", snippet.Attributes.Content.Value, snippetContent)
	}
	if snippet.ID != snippetSHA {
		t.Errorf("snippet ID = %q, want %q", snippet.ID, snippetSHA)
	}
}
