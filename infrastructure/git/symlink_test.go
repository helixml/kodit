package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

// nonGitDirWithSymlinks returns a plain (non-git) directory holding one
// regular file and symbolic links to a file in another directory and to host
// files, plus the path of the other directory's file.
func nonGitDirWithSymlinks(t *testing.T) (dir, outsideFile string) {
	t.Helper()
	base := t.TempDir()
	dir = filepath.Join(base, "dir")
	other := filepath.Join(base, "other")
	for _, d := range []string{dir, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	outsideFile = filepath.Join(other, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "own.txt"), []byte("own"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{
		"link.txt":     outsideFile,
		"hostname.txt": "/etc/hostname",
		"environ.txt":  "/proc/self/environ",
		"linked-dir":   other,
	} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dir, outsideFile
}

func TestFileContent_NonGitDir_DoesNotFollowSymlinks(t *testing.T) {
	dir, _ := nonGitDirWithSymlinks(t)
	adapter := &GiteaAdapter{layout: NewLayout("", false), logger: zerolog.Nop()}
	ctx := context.Background()

	got, err := adapter.FileContent(ctx, dir, "", "own.txt")
	if err != nil || string(got) != "own" {
		t.Fatalf("own.txt = %q, %v", got, err)
	}
	for _, rel := range []string{"link.txt", "hostname.txt", "environ.txt", "linked-dir/secret.txt"} {
		got, err := adapter.FileContent(ctx, dir, "", rel)
		if !errors.Is(err, ErrFileNotFound) {
			t.Errorf("%s: err = %v, content %q; want ErrFileNotFound", rel, err, got)
		}
	}
}

func TestScanCommit_NonGitDir_SkipsSymlinks(t *testing.T) {
	dir, outsideFile := nonGitDirWithSymlinks(t)
	scanner := NewRepositoryScanner(&GiteaAdapter{logger: zerolog.Nop()}, NewLayout("", false), zerolog.Nop())

	result, err := scanner.ScanCommit(context.Background(), dir, "sha", 1)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range result.Files() {
		paths = append(paths, f.Path())
	}
	if len(paths) != 1 || paths[0] != "own.txt" {
		t.Errorf("scanned files = %v, want [own.txt]", paths)
	}

	// The directory hash must not depend on a symlink target's content.
	before, err := dirHash(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideFile, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := dirHash(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Error("directory hash changed when a symlink target outside the directory changed")
	}
}
