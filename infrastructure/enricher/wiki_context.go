package enricher

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
)

// WikiContextService gathers context for wiki generation.
type WikiContextService struct{}

// NewWikiContextService creates a new WikiContextService.
func NewWikiContextService() *WikiContextService {
	return &WikiContextService{}
}

// Gather collects all relevant context for wiki generation.
// Returns readme content, file tree listing, and existing enrichment summaries.
func (s *WikiContextService) Gather(
	_ context.Context,
	repoPath string,
	files []repository.File,
	existingEnrichments []enrichment.Enrichment,
) (readme, fileTree, enrichments string, err error) {
	readme = s.extractReadme(repoPath)
	fileTree = s.buildFileTree(files)
	enrichments = s.summarizeEnrichments(existingEnrichments)
	return readme, fileTree, enrichments, nil
}

// FileContent reads a file from the repo, truncated to maxLen characters.
func (s *WikiContextService) FileContent(repoPath, filePath string, maxLen int) string {
	fullPath := filepath.Join(repoPath, filePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}
	content := string(data)
	if len(content) > maxLen {
		content = content[:maxLen] + "\n...[truncated]"
	}
	return content
}

func (s *WikiContextService) extractReadme(repoPath string) string {
	names := []string{"README.md", "README.rst", "README.txt", "README"}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(repoPath, name))
		if err != nil {
			continue
		}
		content := string(data)
		if len(content) > 3000 {
			content = content[:3000] + "\n...[truncated]"
		}
		return content
	}
	return ""
}

func (s *WikiContextService) buildFileTree(files []repository.File) string {
	if len(files) == 0 {
		return ""
	}

	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path())
	}

	// Cap at 200 paths to stay within token limits.
	if len(paths) > 200 {
		paths = paths[:200]
		paths = append(paths, "... and more files")
	}

	return strings.Join(paths, "\n")
}

func (s *WikiContextService) summarizeEnrichments(enrichments []enrichment.Enrichment) string {
	var sections []string

	for _, e := range enrichments {
		label := string(e.Type()) + "/" + string(e.Subtype())
		content := e.Content()

		// Truncate long enrichments to keep context manageable.
		if len(content) > 2000 {
			content = content[:2000] + "\n...[truncated]"
		}

		sections = append(sections, "### "+label+"\n"+content)
	}

	if len(sections) == 0 {
		return ""
	}

	return strings.Join(sections, "\n\n")
}
