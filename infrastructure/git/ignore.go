package git

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IgnorePattern provides file ignore pattern matching for git repositories.
// It combines gitignore rules from the repository with custom .noindex patterns.
type IgnorePattern struct {
	base         string
	isGitRepo    bool
	noIndexRules []string
}

// NewIgnorePattern creates an IgnorePattern for the given base directory.
// Returns an error if the base directory does not exist or is not a directory.
func NewIgnorePattern(base string) (IgnorePattern, error) {
	info, err := os.Stat(base)
	if err != nil {
		return IgnorePattern{}, err
	}
	if !info.IsDir() {
		return IgnorePattern{}, &NotDirectoryError{Path: base}
	}

	pattern := IgnorePattern{
		base: base,
	}

	// Check if this is a git repository by looking for .git directory
	_, err = os.Stat(filepath.Join(base, ".git"))
	if err == nil {
		pattern.isGitRepo = true
	}

	// Load .noindex patterns if present
	noindexPath := filepath.Join(base, ".noindex")
	if rules, err := loadNoIndexPatterns(noindexPath); err == nil {
		pattern.noIndexRules = rules
	}

	return pattern, nil
}

// ShouldIgnore checks if a path should be ignored.
// Directories are never ignored. Files are ignored if they match gitignore
// rules or .noindex patterns.
func (p IgnorePattern) ShouldIgnore(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Directories are never ignored
	if info.IsDir() {
		return false
	}

	// Get relative path
	relPath, err := filepath.Rel(p.base, path)
	if err != nil {
		return false
	}

	// Normalize to forward slashes for pattern matching
	relPath = filepath.ToSlash(relPath)

	// Files in .git directory are always ignored
	if strings.HasPrefix(relPath, ".git") {
		return true
	}

	// Check git ignore rules if repo is available
	if p.isGitRepo {
		if p.matchGitIgnore(relPath) {
			return true
		}
	}

	// Check .noindex rules
	if p.matchNoIndex(relPath) {
		return true
	}

	return false
}

// matchGitIgnore checks if the path matches gitignore rules using git check-ignore.
func (p IgnorePattern) matchGitIgnore(relPath string) bool {
	cmd := exec.CommandContext(context.Background(), "git", "check-ignore", "-q", relPath)
	cmd.Dir = p.base
	err := cmd.Run()
	// Exit 0 means the path is ignored, exit 1 means it's not
	return err == nil
}

// matchNoIndex checks if the path matches .noindex patterns.
func (p IgnorePattern) matchNoIndex(relPath string) bool {
	if len(p.noIndexRules) == 0 {
		return false
	}

	for _, pattern := range p.noIndexRules {
		// Try matching against the full relative path
		matched, err := filepath.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}

		// Try matching against each path component
		parts := strings.Split(relPath, "/")
		for _, part := range parts {
			matched, err = filepath.Match(pattern, part)
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}

// loadNoIndexPatterns reads patterns from a .noindex file.
func loadNoIndexPatterns(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var patterns []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}

// NotDirectoryError indicates the path is not a directory.
type NotDirectoryError struct {
	Path string
}

func (e *NotDirectoryError) Error() string {
	return "path is not a directory: " + e.Path
}
