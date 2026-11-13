package workingcopy

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// newTree builds a working copy with regular files plus symbolic links that
// point inside the root, at another directory, and at host files.
func newTree(t *testing.T) (root, outside string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "repo")
	outside = filepath.Join(base, "other-clone")
	for _, dir := range []string{root, outside, filepath.Join(root, "docs"), filepath.Join(outside, "docs")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, content string) {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "README.md"), "own readme")
	write(filepath.Join(root, "docs", "guide.md"), "own guide")
	write(filepath.Join(outside, "secret.txt"), "other tenant")
	write(filepath.Join(outside, "docs", "secret.md"), "other tenant doc")

	link := func(target, name string) {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	link(filepath.Join(outside, "secret.txt"), "abs-link.txt")
	link("../other-clone/secret.txt", "rel-link.txt")
	link("README.md", "inside-link.md")
	link(filepath.Join(outside, "docs"), "linked-dir")
	link("/etc/hostname", "hostname.txt")
	link("/proc/self/environ", "environ.txt")
	return root, outside
}

func TestReadFile(t *testing.T) {
	root, _ := newTree(t)
	tests := []struct {
		rel     string
		want    string
		wantErr error
	}{
		{"README.md", "own readme", nil},
		{"docs/guide.md", "own guide", nil},
		{"abs-link.txt", "", ErrSymlink},
		{"rel-link.txt", "", ErrSymlink},
		{"inside-link.md", "", ErrSymlink},
		{"linked-dir/secret.md", "", ErrSymlink},
		{"hostname.txt", "", ErrSymlink},
		{"environ.txt", "", ErrSymlink},
		{"../other-clone/secret.txt", "", ErrOutsideRoot},
		{"/etc/hostname", "", ErrOutsideRoot},
		{".", "", ErrOutsideRoot},
		{"docs", "", ErrNotRegular},
	}
	for _, tt := range tests {
		t.Run(tt.rel, func(t *testing.T) {
			got, err := ReadFile(root, tt.rel)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v (content %q)", err, tt.wantErr, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("content = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFileAndStatRefuseSymlinks(t *testing.T) {
	root, _ := newTree(t)
	for _, rel := range []string{"abs-link.txt", "linked-dir/secret.md", "hostname.txt", "environ.txt"} {
		if _, err := File(root, rel); !errors.Is(err, ErrSymlink) {
			t.Errorf("File(%s) err = %v, want ErrSymlink", rel, err)
		}
		if _, err := Stat(root, rel); !errors.Is(err, ErrSymlink) {
			t.Errorf("Stat(%s) err = %v, want ErrSymlink", rel, err)
		}
	}
	p, err := File(root, "docs/guide.md")
	if err != nil || p != filepath.Join(root, "docs", "guide.md") {
		t.Errorf("File(docs/guide.md) = %q, %v", p, err)
	}
}

func TestWalkDirSkipsSymlinks(t *testing.T) {
	root, _ := newTree(t)
	var seen []string
	err := WalkDir(root, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			seen = append(seen, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(seen)
	want := []string{"README.md", "docs/guide.md"}
	if len(seen) != len(want) || seen[0] != want[0] || seen[1] != want[1] {
		t.Errorf("walked %v, want %v", seen, want)
	}
	if err := WalkDir(root, "linked-dir", func(string, fs.DirEntry, error) error { return nil }); !errors.Is(err, ErrSymlink) {
		t.Errorf("walking a symlinked directory: err = %v, want ErrSymlink", err)
	}
}

func TestReadDirAndGlobSkipSymlinks(t *testing.T) {
	root, _ := newTree(t)
	entries, err := ReadDir(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "README.md" || names[1] != "docs" {
		t.Errorf("ReadDir = %v, want [README.md docs]", names)
	}

	matches, err := Glob(root, "*")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(matches)
	if len(matches) != 2 || matches[0] != "README.md" || matches[1] != "docs" {
		t.Errorf("Glob(*) = %v, want [README.md docs]", matches)
	}
	if matches, _ := Glob(root, "*/secret.md"); len(matches) != 0 {
		t.Errorf("Glob followed a symlinked directory: %v", matches)
	}
}
