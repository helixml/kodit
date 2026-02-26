package e2e_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo creates a real git repository in a temp directory with sample files.
// Returns the repo path and the HEAD commit SHA.
func initGitRepo(t *testing.T) (string, string) {
	t.Helper()

	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	run("init", "-b", "main")

	// Create some files
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Test Repo\n\nHello world.\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	srcDir := filepath.Join(repoDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	run("add", ".")
	run("commit", "-m", "initial commit")

	sha := run("rev-parse", "HEAD")

	return repoDir, sha
}

func TestBlob_ReadFileByBranch(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/blob-repo.git", repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")
	ts.CreateBranch(repo, "main", commitSHA, true)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/blob/main/README.md", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	body := ts.ReadBody(resp)
	if !strings.Contains(body, "# Test Repo") {
		t.Errorf("expected README content, got: %s", body)
	}
	if sha := resp.Header.Get("X-Commit-SHA"); sha != commitSHA {
		t.Errorf("X-Commit-SHA = %q, want %q", sha, commitSHA)
	}
}

func TestBlob_ReadFileByCommitSHA(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/blob-sha-repo.git", repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/blob/%s/src/main.go", repo.ID(), commitSHA))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	body := ts.ReadBody(resp)
	if !strings.Contains(body, "package main") {
		t.Errorf("expected Go source, got: %s", body)
	}
}

func TestBlob_ReadFileByTag(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/blob-tag-repo.git", repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")
	ts.CreateTag(repo, "v1.0.0", commitSHA)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/blob/v1.0.0/README.md", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	body := ts.ReadBody(resp)
	if !strings.Contains(body, "# Test Repo") {
		t.Errorf("expected README content, got: %s", body)
	}
}

func TestBlob_FileNotFound_Returns404(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/blob-404-repo.git", repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")
	ts.CreateBranch(repo, "main", commitSHA, true)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/blob/main/nonexistent.txt", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusNotFound, body)
	}
}

func TestBlob_RepoNotFound_Returns404(t *testing.T) {
	ts := NewTestServer(t)

	resp := ts.GET("/api/v1/repositories/99999/blob/main/README.md")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestBlob_BranchNotFound_Returns500(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/blob-no-branch-repo.git", repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")
	// Deliberately don't create the "develop" branch

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/blob/develop/README.md", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	// "develop" doesn't match a commit SHA, tag, or branch → unresolvable
	if resp.StatusCode != http.StatusInternalServerError {
		body := ts.ReadBody(resp)
		t.Errorf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusInternalServerError, body)
	}
}

func TestBlob_ReadNestedFilePath(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/blob-nested-repo.git", repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")
	ts.CreateBranch(repo, "main", commitSHA, true)

	// Request a file in a subdirectory — this mirrors the production bug where
	// chi.URLParam(req, "*") returns URL-encoded paths (e.g. src%2Fmain.go).
	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/blob/main/src/main.go", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	body := ts.ReadBody(resp)
	if !strings.Contains(body, "package main") {
		t.Errorf("expected Go source, got: %s", body)
	}
}

// TestBlob_ReadFileWithEncodedSlashes reproduces the production bug where a client
// (or reverse proxy) sends percent-encoded forward slashes (%2F) in the file path.
// chi.URLParam(req, "*") returns the raw (encoded) value, so the handler must decode it.
func TestBlob_ReadFileWithEncodedSlashes(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/blob-encoded-repo.git", repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")
	ts.CreateBranch(repo, "main", commitSHA, true)

	// Build a raw URL with %2F instead of / in the file path portion.
	// This is what happens in production when clients URL-encode the path.
	rawURL := fmt.Sprintf("%s/api/v1/repositories/%d/blob/main/src%%2Fmain.go", ts.URL(), repo.ID())
	resp, err := http.Get(rawURL)
	if err != nil {
		t.Fatalf("GET %s: %v", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	body := ts.ReadBody(resp)
	if !strings.Contains(body, "package main") {
		t.Errorf("expected Go source, got: %s", body)
	}
}

func TestBlob_WithLineFilter(t *testing.T) {
	ts := NewTestServer(t)
	repoDir, commitSHA := initGitRepo(t)

	repo := ts.CreateRepositoryWithRealWorkingCopy("https://github.com/test/blob-lines-repo.git", repoDir)
	ts.CreateCommit(repo, commitSHA, "initial commit")
	ts.CreateBranch(repo, "main", commitSHA, true)

	resp := ts.GET(fmt.Sprintf("/api/v1/repositories/%d/blob/main/README.md?lines=L1", repo.ID()))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body := ts.ReadBody(resp)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, http.StatusOK, body)
	}

	body := ts.ReadBody(resp)
	if body != "# Test Repo" {
		t.Errorf("expected first line only, got: %q", body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}
