package service

import (
	"path/filepath"
	"strings"
)

// matchGlob matches a file path against a glob pattern supporting **
// (matches zero or more path segments) and delegates to filepath.Match
// for single-segment patterns.
func matchGlob(pattern, path string) bool {
	// Fast path: no ** in pattern, use filepath.Match directly.
	if !strings.Contains(pattern, "**") {
		ok, _ := filepath.Match(pattern, path)
		return ok
	}

	// Split pattern on ** and match segments recursively.
	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0]
	suffix := strings.TrimLeft(parts[1], "/")

	// The prefix must match the beginning of path.
	if prefix != "" {
		prefix = strings.TrimRight(prefix, "/")
		if !strings.HasPrefix(path, prefix+"/") && path != prefix {
			return false
		}
		path = strings.TrimPrefix(path, prefix+"/")
	}

	// If suffix is empty, ** matches everything remaining.
	if suffix == "" {
		return true
	}

	// Try matching suffix against every possible tail of path.
	segments := strings.Split(path, "/")
	for i := range segments {
		tail := strings.Join(segments[i:], "/")
		if matchGlob(suffix, tail) {
			return true
		}
	}
	return false
}
