package e2e_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestSearchGrep_MatchesPattern(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/grep-repo.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/grep?repository_id=%d&pattern=func", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	var result struct {
		Data []struct {
			Path  string `json:"path"`
			Links *struct {
				File string `json:"file"`
			} `json:"links"`
			Matches []struct {
				Line    int    `json:"line"`
				Content string `json:"content"`
			} `json:"matches"`
		} `json:"data"`
	}
	ts.DecodeJSON(resp, &result)

	if len(result.Data) == 0 {
		t.Fatal("expected at least one grep result")
	}

	found := false
	for _, d := range result.Data {
		if d.Path == "src/main.go" {
			found = true
			// Check that file link uses blob API format
			expectedLink := fmt.Sprintf("/api/v1/repositories/%d/blob/%s/src/main.go", repo.ID(), commitSHA)
			if d.Links == nil || d.Links.File != expectedLink {
				var got string
				if d.Links != nil {
					got = d.Links.File
				}
				t.Errorf("links.file = %s, want %s", got, expectedLink)
			}
		}
	}

	if !found {
		t.Error("expected grep result for src/main.go")
	}
}

func TestSearchGrep_MissingRepositoryID(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/grep?pattern=func")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSearchGrep_MissingPattern(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/search/grep?repository_id=1")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSearchGrep_InvalidRegex_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/grep-invalid-regex.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	// "[invalid" is a malformed regex (unclosed bracket).
	resp := ts.GET(fmt.Sprintf("/api/v1/search/grep?repository_id=%d&pattern=%s",
		repo.ID(), url.QueryEscape("[invalid")))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSearchGrep_PathTraversalGlob_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/grep-path-traversal.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/grep?repository_id=%d&pattern=test&glob=%s",
		repo.ID(), url.QueryEscape("../../etc/*")))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestSearchGrep_LimitZero_Returns400(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/grep-limit-zero.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/search/grep?repository_id=%d&pattern=func&limit=0",
		repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestRepositoriesGrep_DeprecatedHeader(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repoURL := "https://github.com/test/grep-deprecated-repo.git"
	repo := ts.CreateRepositoryWithRealWorkingCopy(repoURL, repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/grep?pattern=func", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	deprecated := resp.Header.Get("Deprecated")
	if deprecated == "" {
		t.Error("expected Deprecated header on old grep endpoint")
	}
}
