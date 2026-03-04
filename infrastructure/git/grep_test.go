package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

// initTestRepo creates a temporary git repository with a single committed file.
// Returns the repo path and the commit SHA.
func initTestRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return string(out)
	}

	run("init", "-b", "main")
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", name)
	}
	run("commit", "-m", "initial")

	return dir
}

func TestGiteaAdapter_Grep(t *testing.T) {
	adapter, err := NewGiteaAdapter(zerolog.New(os.Stderr).With().Timestamp().Logger())
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}

	repoPath := initTestRepo(t, map[string]string{
		"main.go":     "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n",
		"lib/util.go": "package lib\n\nfunc Parse() error {\n\treturn nil\n}\n",
		"README.md":   "# Project\nThis is a project.\n",
	})

	ctx := context.Background()
	sha, err := adapter.LatestCommitSHA(ctx, repoPath, "main")
	if err != nil {
		t.Fatalf("get commit SHA: %v", err)
	}

	t.Run("matches across files", func(t *testing.T) {
		matches, err := adapter.Grep(ctx, repoPath, sha, "func", "", 100)
		if err != nil {
			t.Fatalf("grep: %v", err)
		}
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
		}
	})

	t.Run("pathspec filters to Go files", func(t *testing.T) {
		matches, err := adapter.Grep(ctx, repoPath, sha, "func", "*.go", 100)
		if err != nil {
			t.Fatalf("grep: %v", err)
		}
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(matches))
		}
		for _, m := range matches {
			if filepath.Ext(m.Path) != ".go" {
				t.Errorf("expected .go file, got %s", m.Path)
			}
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		matches, err := adapter.Grep(ctx, repoPath, sha, "nonexistent_string_xyz", "", 100)
		if err != nil {
			t.Fatalf("grep: %v", err)
		}
		if len(matches) != 0 {
			t.Fatalf("expected 0 matches, got %d", len(matches))
		}
	})

	t.Run("regex pattern", func(t *testing.T) {
		matches, err := adapter.Grep(ctx, repoPath, sha, "func.*\\(\\)", "", 100)
		if err != nil {
			t.Fatalf("grep: %v", err)
		}
		// "func main()" and "func Parse()" both match
		if len(matches) != 2 {
			t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
		}
	})

	t.Run("line numbers are correct", func(t *testing.T) {
		matches, err := adapter.Grep(ctx, repoPath, sha, "func main", "", 100)
		if err != nil {
			t.Fatalf("grep: %v", err)
		}
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].Path != "main.go" {
			t.Errorf("expected main.go, got %s", matches[0].Path)
		}
		if matches[0].Line != 3 {
			t.Errorf("expected line 3, got %d", matches[0].Line)
		}
		if matches[0].Content != "func main() {" {
			t.Errorf("expected 'func main() {', got %q", matches[0].Content)
		}
	})
}
