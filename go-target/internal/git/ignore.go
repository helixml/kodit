package git

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// IgnorePattern provides file ignore pattern matching for git repositories.
// It combines gitignore rules from the repository with custom .noindex patterns.
type IgnorePattern struct {
	base          string
	repo          *git.Repository
	noIndexRules  []gitignore.Pattern
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

	// Try to open as git repository
	repo, err := git.PlainOpen(base)
	if err == nil {
		pattern.repo = repo
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
	if p.repo != nil {
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

// matchGitIgnore checks if the path matches gitignore rules.
func (p IgnorePattern) matchGitIgnore(relPath string) bool {
	wt, err := p.repo.Worktree()
	if err != nil {
		return false
	}

	patterns, err := gitignore.ReadPatterns(wt.Filesystem, nil)
	if err != nil {
		return false
	}

	// Split path into parts for matching
	parts := strings.Split(relPath, "/")
	matcher := gitignore.NewMatcher(patterns)

	return matcher.Match(parts, false)
}

// matchNoIndex checks if the path matches .noindex patterns.
func (p IgnorePattern) matchNoIndex(relPath string) bool {
	if len(p.noIndexRules) == 0 {
		return false
	}

	parts := strings.Split(relPath, "/")
	matcher := gitignore.NewMatcher(p.noIndexRules)

	return matcher.Match(parts, false)
}

// loadNoIndexPatterns reads patterns from a .noindex file.
func loadNoIndexPatterns(path string) ([]gitignore.Pattern, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var patterns []gitignore.Pattern
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, nil))
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
