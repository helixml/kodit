package e2e_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestGlobFiles_MatchesPattern(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, _ := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/glob-repo.git", repoDir)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/files?glob=**/*.go", repo.ID()))
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
		} `json:"data"`
	}
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 1 {
		t.Fatalf("expected 1 Go file, got %d", len(result.Data))
	}
	if result.Data[0].Attributes.Path != "src/main.go" {
		t.Errorf("expected src/main.go, got %s", result.Data[0].Attributes.Path)
	}
}

func TestGlobFiles_WithFilter(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, _ := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/glob-filter-repo.git", repoDir)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/files?glob=*&filter=src", repo.ID()))
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
		} `json:"data"`
	}
	ts.DecodeJSON(resp, &result)

	if len(result.Data) != 1 {
		t.Fatalf("expected 1 result with 'src' filter, got %d", len(result.Data))
	}
	if result.Data[0].Attributes.Path != "src/main.go" {
		t.Errorf("expected src/main.go, got %s", result.Data[0].Attributes.Path)
	}
}

func TestGlobFiles_MissingGlob(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, _ := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/glob-missing-repo.git", repoDir)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/files", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusBadRequest, body)
	}
}

func TestGlobFiles_RepoNotFound(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/repositories/99999/files?glob=*.go")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGlobFiles_NoMatches(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, _ := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/glob-empty-repo.git", repoDir)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/files?glob=**/*.rs", repo.ID()))
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

func TestGlobFiles_AllFiles(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, _ := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/glob-all-repo.git", repoDir)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/files?glob=*", repo.ID()))
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
		} `json:"data"`
	}
	ts.DecodeJSON(resp, &result)

	// initGitRepo creates README.md and src/main.go
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result.Data))
	}
}
