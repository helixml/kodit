package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/helixml/kodit/internal/api/v1/dto"
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

	if result.TotalCount != 0 {
		t.Errorf("total_count = %d, want 0", result.TotalCount)
	}
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

	if result.TotalCount != 1 {
		t.Errorf("total_count = %d, want 1", result.TotalCount)
	}
	if len(result.Data) != 1 {
		t.Errorf("len(data) = %d, want 1", len(result.Data))
	}
	if result.Data[0].RemoteURL != "https://github.com/test/repo.git" {
		t.Errorf("remote_url = %q, want %q", result.Data[0].RemoteURL, "https://github.com/test/repo.git")
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

	if result.ID == 0 {
		t.Error("ID should not be 0")
	}
	if result.RemoteURL != "https://github.com/test/new-repo.git" {
		t.Errorf("remote_url = %q, want %q", result.RemoteURL, "https://github.com/test/new-repo.git")
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

	if result.TrackingType != "branch" {
		t.Errorf("tracking_type = %q, want %q", result.TrackingType, "branch")
	}
	if result.TrackingValue != "main" {
		t.Errorf("tracking_value = %q, want %q", result.TrackingValue, "main")
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

	if result.ID != repo.ID() {
		t.Errorf("ID = %d, want %d", result.ID, repo.ID())
	}
	if result.RemoteURL != "https://github.com/test/get-repo.git" {
		t.Errorf("remote_url = %q, want %q", result.RemoteURL, "https://github.com/test/get-repo.git")
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

func TestRepositories_Sync_NotCloned(t *testing.T) {
	ts := NewTestServer(t)

	// Create a repository without a working copy (not cloned)
	repo := ts.CreateRepository("https://github.com/test/sync-repo.git")

	resp := ts.POST(fmt.Sprintf("/api/v1/repositories/%d/sync", repo.ID()), nil)
	defer func() {
		_ = resp.Body.Close()
	}()

	// Sync fails for uncloned repos with 500 (or could be 400)
	// The error is "repository has not been cloned"
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestRepositories_Sync_NotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.POST("/api/v1/repositories/99999/sync", nil)
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
