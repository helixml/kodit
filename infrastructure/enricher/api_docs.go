package enricher

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
)

// APIDocService extracts API documentation from code files.
type APIDocService struct{}

// NewAPIDocService creates a new APIDocService.
func NewAPIDocService() *APIDocService {
	return &APIDocService{}
}

// Extract analyzes files to extract public API documentation.
// This is a simplified implementation that looks for common API patterns.
func (s *APIDocService) Extract(ctx context.Context, files []repository.File, language string, includePrivate bool) ([]enrichment.Enrichment, error) {
	var enrichments []enrichment.Enrichment

	for _, file := range files {
		filePath := file.Path()
		if filePath == "" {
			continue
		}

		// Skip test files
		base := filepath.Base(filePath)
		if strings.Contains(base, "test") || strings.Contains(base, "_test") || strings.Contains(base, "spec") {
			continue
		}

		content := s.extractPublicAPI(filePath, language, includePrivate)
		if content == "" {
			continue
		}

		e := enrichment.NewEnrichment(
			enrichment.TypeUsage,
			enrichment.SubtypeAPIDocs,
			enrichment.EntityTypeSnippet,
			content,
		)
		enrichments = append(enrichments, e)
	}

	return enrichments, nil
}

func (s *APIDocService) extractPublicAPI(filePath, language string, includePrivate bool) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	content := string(data)

	// Look for public API indicators based on language
	var apiIndicators []string

	switch language {
	case "python":
		apiIndicators = []string{"def ", "class ", "async def "}
	case "go":
		apiIndicators = []string{"func ", "type ", "var ", "const "}
	case "javascript", "typescript":
		apiIndicators = []string{"export ", "function ", "class ", "const ", "async function "}
	case "java":
		apiIndicators = []string{"public ", "class ", "interface "}
	case "rust":
		apiIndicators = []string{"pub fn ", "pub struct ", "pub enum ", "pub trait "}
	default:
		return ""
	}

	// Simple heuristic: count public API elements
	publicAPIs := 0
	for _, indicator := range apiIndicators {
		publicAPIs += strings.Count(content, indicator)
	}

	// Only include files with substantial public API
	if publicAPIs < 3 {
		return ""
	}

	// Extract first N lines as API overview
	lines := strings.Split(content, "\n")
	maxLines := 100
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	// Filter to only include lines with API indicators
	var apiLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, indicator := range apiIndicators {
			if strings.Contains(trimmed, indicator) {
				apiLines = append(apiLines, line)
				break
			}
		}
	}

	if len(apiLines) == 0 {
		return ""
	}

	result := "### " + filepath.Base(filePath) + " (" + language + ")\n\n"
	result += "```" + language + "\n"
	result += strings.Join(apiLines, "\n")
	result += "\n```"

	return result
}
