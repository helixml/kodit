// Package workingcopy reads files inside a repository working copy without
// following symbolic links.
//
// A working copy holds content chosen by whoever controls the repository. A
// symbolic link committed to it can point anywhere on the host: another
// repository's clone, a configuration file, /proc. Every read of working-copy
// content must therefore go through this package, which refuses any path whose
// components (below the working-copy root) include a symbolic link, and opens
// files through os.Root so a path can never resolve outside the root.
//
// The root itself is trusted: it is a directory kodit created or was
// configured with, and may legitimately sit under a symlinked parent such as
// /tmp on macOS.
package workingcopy

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrSymlink is returned when a path inside a working copy is, or passes
// through, a symbolic link.
var ErrSymlink = errors.New("symbolic links in working copies are not followed")

// ErrNotRegular is returned when a path names something other than a regular
// file where file content was requested.
var ErrNotRegular = errors.New("not a regular file")

// ErrOutsideRoot is returned when a relative path is absolute or leaves the
// working-copy root.
var ErrOutsideRoot = errors.New("path is outside the working copy")

// Path returns the on-disk path of rel inside root after checking that rel
// stays inside root and that neither rel nor any directory leading to it is
// a symbolic link. Use it only to hand a path to code that opens it itself
// (document renderers, rasterizers); use ReadFile to read content.
func Path(root, rel string) (string, error) {
	clean, err := localPath(rel)
	if err != nil {
		return "", err
	}
	current := root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return "", fmt.Errorf("%s: %w", clean, ErrSymlink)
		}
	}
	return current, nil
}

// File returns the on-disk path of the regular file rel inside root, with the
// same checks as Path.
func File(root, rel string) (string, error) {
	p, err := Path(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(p)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s: %w", rel, ErrNotRegular)
	}
	return p, nil
}

// ReadFile returns the content of the regular file rel inside root.
func ReadFile(root, rel string) ([]byte, error) {
	if _, err := File(root, rel); err != nil {
		return nil, err
	}
	clean, err := localPath(rel)
	if err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open working copy: %w", err)
	}
	defer func() { _ = r.Close() }()

	f, err := r.Open(clean)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %w", rel, ErrNotRegular)
	}
	return io.ReadAll(f)
}

// Stat returns file information for rel inside root without following
// symbolic links; a symbolic link is reported as ErrSymlink.
func Stat(root, rel string) (fs.FileInfo, error) {
	p, err := Path(root, rel)
	if err != nil {
		return nil, err
	}
	return os.Lstat(p)
}

// ReadDir returns the entries of directory rel inside root, leaving out
// symbolic links. Use "." for the root itself.
func ReadDir(root, rel string) ([]fs.DirEntry, error) {
	dir := root
	if rel != "." {
		p, err := Path(root, rel)
		if err != nil {
			return nil, err
		}
		dir = p
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := entries[:0]
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink == 0 {
			out = append(out, e)
		}
	}
	return out, nil
}

// WalkDir walks the tree rooted at rel inside root (use "." for the whole
// working copy), calling fn with paths relative to root. Symbolic links are
// skipped: fn never sees one, and they are never descended into.
func WalkDir(root, rel string, fn fs.WalkDirFunc) error {
	start := root
	if rel != "." {
		p, err := Path(root, rel)
		if err != nil {
			return err
		}
		start = p
	}
	return filepath.WalkDir(start, func(p string, d fs.DirEntry, err error) error {
		relPath, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		if err == nil && d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		return fn(relPath, d, err)
	})
}

// Glob returns the paths, relative to root, of entries inside root matching
// pattern (filepath.Match syntax, relative to root) that are not and do not
// pass through a symbolic link.
func Glob(root, pattern string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, m := range matches {
		rel, err := filepath.Rel(root, m)
		if err != nil {
			continue
		}
		if _, err := Path(root, rel); err != nil {
			continue
		}
		out = append(out, rel)
	}
	return out, nil
}

func localPath(rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || !filepath.IsLocal(clean) {
		return "", fmt.Errorf("%q: %w", rel, ErrOutsideRoot)
	}
	return clean, nil
}
