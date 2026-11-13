package enricher

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/helixml/kodit/internal/workingcopy"
)

// DatabaseSchemaService detects and extracts database schemas from a repository.
type DatabaseSchemaService struct{}

// NewDatabaseSchemaService creates a new DatabaseSchemaService.
func NewDatabaseSchemaService() *DatabaseSchemaService {
	return &DatabaseSchemaService{}
}

// Discover scans a repository for database schema definitions.
func (s *DatabaseSchemaService) Discover(ctx context.Context, repoPath string) (string, error) {
	var schemaContent []string

	// Look for common schema locations
	schemaPaths := []string{
		// SQL migrations
		"migrations",
		"db/migrations",
		"database/migrations",
		"sql",
		"schema",
		"db/schema",
		// Django/Python
		"models.py",
		"*/models.py",
		// Rails
		"db/schema.rb",
		// Prisma
		"prisma/schema.prisma",
		// SQLAlchemy
		"alembic/versions",
	}

	for _, pattern := range schemaPaths {
		matches, err := workingcopy.Glob(repoPath, pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			content := s.extractSchemaContent(repoPath, match)
			if content != "" {
				schemaContent = append(schemaContent, content)
			}
		}
	}

	// Also look for common schema file patterns
	schemaExtensions := []string{".sql", ".prisma"}
	schemaFileNames := []string{"schema", "models", "migration", "create_"}

	err := workingcopy.WalkDir(repoPath, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip common non-schema directories
		if strings.Contains(path, "node_modules") || strings.Contains(path, ".git") || strings.Contains(path, "vendor") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		base := strings.ToLower(filepath.Base(path))

		for _, schemaExt := range schemaExtensions {
			if ext == schemaExt {
				content := s.extractSchemaContent(repoPath, path)
				if content != "" {
					schemaContent = append(schemaContent, content)
				}
				return nil
			}
		}

		for _, namePattern := range schemaFileNames {
			if strings.Contains(base, namePattern) && (ext == ".py" || ext == ".rb" || ext == ".ts" || ext == ".go") {
				content := s.extractSchemaContent(repoPath, path)
				if content != "" {
					schemaContent = append(schemaContent, content)
				}
				return nil
			}
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	if len(schemaContent) == 0 {
		return "No database schemas detected in the repository.", nil
	}

	// Limit total output size
	result := strings.Join(schemaContent, "\n\n---\n\n")
	if len(result) > 10000 {
		result = result[:10000] + "\n\n...[truncated]"
	}

	return result, nil
}

// extractSchemaContent reads the schema file, or the schema files directly
// inside the directory, at path relative to repoPath.
func (s *DatabaseSchemaService) extractSchemaContent(repoPath, path string) string {
	info, err := workingcopy.Stat(repoPath, path)
	if err != nil {
		return ""
	}

	if info.IsDir() {
		return s.extractFromDirectory(repoPath, path)
	}

	data, err := workingcopy.ReadFile(repoPath, path)
	if err != nil {
		return ""
	}

	content := string(data)

	// Truncate large files
	if len(content) > 2000 {
		content = content[:2000] + "\n...[truncated]"
	}

	return "### " + filepath.Base(path) + "\n```\n" + content + "\n```"
}

func (s *DatabaseSchemaService) extractFromDirectory(repoPath, dirPath string) string {
	var contents []string

	entries, err := workingcopy.ReadDir(repoPath, dirPath)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)

		// Only process schema-like files
		if ext != ".sql" && ext != ".prisma" && ext != ".rb" && ext != ".py" {
			continue
		}

		data, err := workingcopy.ReadFile(repoPath, filepath.Join(dirPath, name))
		if err != nil {
			continue
		}

		content := string(data)
		if len(content) > 500 {
			content = content[:500] + "\n...[truncated]"
		}

		contents = append(contents, "### "+name+"\n```\n"+content+"\n```")

		// Limit number of files processed
		if len(contents) >= 5 {
			break
		}
	}

	if len(contents) == 0 {
		return ""
	}

	return strings.Join(contents, "\n\n")
}
