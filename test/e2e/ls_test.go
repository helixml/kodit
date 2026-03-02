package e2e_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestLs_MatchesPattern(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/ls-repo.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repo_url=%s&pattern=**/*.go", url.QueryEscape(repoURL)))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	var result struct {
		Data []struct {
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

	expectedURI := fmt.Sprintf("file://%d/%s/src/main.go", repo.ID(), commitSHA)
	if result.Data[0].Links.Self != expectedURI {
		t.Errorf("expected link %s, got %s", expectedURI, result.Data[0].Links.Self)
	}
}

func TestLs_MissingRepoURL(t *testing.T) {
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

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repo_url=%s", url.QueryEscape("https://github.com/test/x.git")))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestLs_RepoNotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repo_url=%s&pattern=*.go", url.QueryEscape("https://github.com/test/nonexistent.git")))
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

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repo_url=%s&pattern=**/*.rs", url.QueryEscape(repoURL)))
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

func TestLs_AllFiles(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/ls-all-repo.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/ls?repo_url=%s&pattern=*", url.QueryEscape(repoURL)))
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

	for _, f := range result.Data {
		if !strings.HasPrefix(f.Links.Self, "file://") {
			t.Errorf("expected file:// URI, got %s", f.Links.Self)
		}
	}
}
