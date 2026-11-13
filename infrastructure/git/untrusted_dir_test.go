package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commandConfig returns git config that makes diff, the filesystem monitor
// and hooks run a command creating marker.
func commandConfig(marker string) string {
	touch := fmt.Sprintf("sh -c 'touch %s' --", marker)
	return fmt.Sprintf("[diff \"x\"]\n\ttextconv = %s\n[diff]\n\texternal = %s\n[core]\n\tfsmonitor = %s\n\thooksPath = hooks\n", touch, touch, touch)
}

// craftedDirectory builds what a tenant can upload into a file:// directory:
// a bare repository layout (HEAD, config, objects, refs) whose object store
// borrows another repository's objects through objects/info/alternates and
// whose config and attributes run a command on diff. It returns the
// directory, the other repository's secret text, the commit SHA, and the
// marker file the command would create.
func craftedDirectory(t *testing.T) (dir, secret, sha, marker string) {
	t.Helper()
	base := t.TempDir()
	marker = filepath.Join(base, "command-ran")

	other := filepath.Join(base, "other-clone")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	secret = "other-repository-secret-text"
	runGit(t, other, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(other, "secret.txt"), []byte(secret+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, other, "add", "secret.txt")
	runGit(t, other, "commit", "-q", "-m", "secret")
	sha = runGit(t, other, "rev-parse", "HEAD")

	dir = filepath.Join(base, "uploaded")
	for _, d := range []string{"objects/info", "refs/heads", "info", "hooks"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"HEAD":                    "ref: refs/heads/main\n",
		"config":                  "[core]\n\trepositoryformatversion = 0\n\tbare = true\n" + commandConfig(marker),
		"refs/heads/main":         sha + "\n",
		"objects/info/alternates": filepath.Join(other, ".git", "objects") + "\n",
		"info/attributes":         "* diff=x\n",
		"hooks/post-checkout":     fmt.Sprintf("#!/bin/sh\ntouch %s\n", marker),
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir, secret, sha, marker
}

func assertNoCommandRan(t *testing.T, marker string) {
	t.Helper()
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a command from the directory's git configuration ran")
	}
}

// TestCraftedDirectory_ControlGitHonoursIt proves the fixture is effective:
// plain git, run the way kodit used to (auto-detecting the repository), reads
// the other repository's object through the alternates file and runs the
// configured textconv command.
func TestCraftedDirectory_ControlGitHonoursIt(t *testing.T) {
	dir, secret, sha, marker := craftedDirectory(t)
	out := runGit(t, dir, "show", sha+":secret.txt")
	if !strings.Contains(out, secret) {
		t.Fatalf("control: alternates not honoured: %s", out)
	}
	runGit(t, dir, "show", sha)
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("control: textconv did not run")
	}
}

// TestCraftedDirectory_IsReadAsPlainFiles checks that kodit never reads a
// directory it did not clone with git, whether or not local git repositories
// are allowed (the crafted layout has no real .git directory).
func TestCraftedDirectory_IsReadAsPlainFiles(t *testing.T) {
	for _, allowLocal := range []bool{false, true} {
		t.Run(fmt.Sprintf("allowLocalGit=%v", allowLocal), func(t *testing.T) {
			dir, secret, sha, marker := craftedDirectory(t)
			layout := NewLayout(t.TempDir(), allowLocal)
			adapter, err := NewGiteaAdapter(layout, zerolog.Nop())
			if err != nil {
				t.Fatal(err)
			}
			scanner := NewRepositoryScanner(adapter, layout, zerolog.Nop())
			ctx := context.Background()

			if layout.IsGitRepository(dir) {
				t.Fatal("crafted directory detected as a git repository")
			}

			branches, err := scanner.ScanAllBranches(ctx, dir, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(branches) != 1 || branches[0].HeadCommitSHA() == sha {
				t.Fatalf("expected one synthetic branch, got %+v", branches)
			}
			result, err := scanner.ScanCommit(ctx, dir, branches[0].HeadCommitSHA(), 1)
			if err != nil {
				t.Fatal(err)
			}
			paths := map[string]bool{}
			for _, f := range result.Files() {
				paths[f.Path()] = true
			}
			for _, want := range []string{"HEAD", "config", "objects/info/alternates", "info/attributes"} {
				if !paths[want] {
					t.Errorf("plain-file scan missing %s: %v", want, paths)
				}
			}
			if tags, err := scanner.ScanAllTags(ctx, dir, 1); err != nil || len(tags) != 0 {
				t.Errorf("tags = %v, %v; want none", tags, err)
			}

			if content, err := adapter.FileContent(ctx, dir, sha, "secret.txt"); err == nil {
				t.Errorf("read secret.txt through the crafted object store: %q", content)
			}
			if diff, err := adapter.CommitDiff(ctx, dir, sha); err != nil || diff != "" {
				t.Errorf("CommitDiff = %q, %v; want empty", diff, err)
			}
			for name, call := range map[string]func() error{
				"Grep": func() error {
					_, err := adapter.Grep(ctx, dir, sha, "secret", "", 10)
					return err
				},
				"CommitFiles": func() error {
					_, err := adapter.CommitFiles(ctx, dir, sha)
					return err
				},
				"CommitDetails": func() error {
					_, err := adapter.CommitDetails(ctx, dir, sha)
					return err
				},
				"AllBranches": func() error {
					_, err := adapter.AllBranches(ctx, dir)
					return err
				},
				"CheckoutCommit": func() error { return adapter.CheckoutCommit(ctx, dir, sha) },
			} {
				if err := call(); !errors.Is(err, ErrNotGitRepository) {
					t.Errorf("%s: err = %v, want ErrNotGitRepository", name, err)
				}
			}
			_ = secret
			assertNoCommandRan(t, marker)
		})
	}
}

// TestHardenedCommands_IgnoreRepositoryCommands covers a local repository the
// operator allowed, with a real .git directory whose config defines
// commands: kodit's diff disables textconv and external diff drivers, and
// checkout runs with hooks and the filesystem monitor disabled.
func TestHardenedCommands_IgnoreRepositoryCommands(t *testing.T) {
	base := t.TempDir()
	marker := filepath.Join(base, "command-ran")
	repo := filepath.Join(base, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-q", "-b", "main")
	for i, content := range []string{"one\n", "two\n"} {
		if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, repo, "add", "file.txt")
		runGit(t, repo, "commit", "-q", "-m", fmt.Sprint(i))
	}
	sha := runGit(t, repo, "rev-parse", "HEAD")
	first := runGit(t, repo, "rev-parse", "HEAD~1")

	f, err := os.OpenFile(filepath.Join(repo, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(commandConfig(marker))
	_ = f.Close()
	if err := os.MkdirAll(filepath.Join(repo, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "hooks", "post-checkout"), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "info", "attributes"), []byte("* diff=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	layout := NewLayout("", true)
	adapter, err := NewGiteaAdapter(layout, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	diff, err := adapter.CommitDiff(ctx, repo, sha)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "-one") || !strings.Contains(diff, "+two") {
		t.Errorf("unexpected diff: %s", diff)
	}
	if err := adapter.CheckoutCommit(ctx, repo, first); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.BranchCommits(ctx, repo, "main"); err != nil {
		t.Fatal(err)
	}
	assertNoCommandRan(t, marker)

	// Control: plain git with the same configuration runs the command.
	runGit(t, repo, "diff", first, sha)
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("control: the configured command did not run")
	}
}
