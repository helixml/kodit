package e2e_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestLs_MatchesPattern(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/ls-repo.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repository_id=%d&pattern=**/*.go", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	var result struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Path string `json:"path"`
				Size int64  `json:"size"`
			} `json:"attributes"`
			Links struct {
				Self string `json:"self"`
			} `json:"links"`
		} `json:"data"`
	}
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 1 {
		t.Fatalf("expected 1 Go file, got %d", len(result.Data))
	}
	if result.Data[0].Attributes.Path != "src/main.go" {
		t.Errorf("expected src/main.go, got %s", result.Data[0].Attributes.Path)
	}

	// ID should be the blob SHA (a hex string), not the path
	id := result.Data[0].ID
	if id == "src/main.go" {
		t.Error("ID should be blob SHA, not file path")
	}
	if len(id) < 7 {
		t.Errorf("ID should be a blob SHA (got %q)", id)
	}

	// Links should use HTTP blob API format, not file:// URIs
	expectedLink := fmt.Sprintf("/api/v1/repositories/%d/blob/%s/src/main.go", repo.ID(), commitSHA)
	if result.Data[0].Links.Self != expectedLink {
		t.Errorf("expected link %s, got %s", expectedLink, result.Data[0].Links.Self)
	}
}

func TestLs_MissingRepositoryID(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/ls?pattern=*.go")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestLs_MissingPattern(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/ls?repository_id=1")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestLs_RepoNotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/ls?repository_id=999999&pattern=*.go")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestLs_NoMatches(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/ls-empty-repo.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	_ = repo
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repository_id=%d&pattern=**/*.rs", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	var result struct {
		Data []json.RawMessage `json:"data"`
	}
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 0 {
		t.Errorf("expected 0 results, got %d", len(result.Data))
	}
}

func TestLs_PageZero_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/ls-page-zero.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repository_id=%d&pattern=*&page=0",
		repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestLs_PageNegative_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/ls-page-negative.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repository_id=%d&pattern=*&page=-1",
		repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestLs_PageSizeZero_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/ls-pagesize-zero.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repository_id=%d&pattern=*&page_size=0",
		repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestLs_PageSizeNegative_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/ls-pagesize-negative.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repository_id=%d&pattern=*&page_size=-1",
		repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestLs_AllFiles(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/ls-all-repo.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repository_id=%d&pattern=*", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
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
	ts.DecodeJSON(resp, &result)

	// initGitRepo creates README.md and src/main.go
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result.Data))
	}

	expectedPrefix := fmt.Sprintf("/api/v1/repositories/%d/blob/", repo.ID())
	for _, f := range result.Data {
		if !strings.HasPrefix(f.Links.Self, expectedPrefix) {
			t.Errorf("expected link prefix %s, got %s", expectedPrefix, f.Links.Self)
		}
	}
}
