package enricher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
		matches, err := filepath.Glob(filepath.Join(repoPath, pattern))
		if err != nil {
			continue
		}

		for _, match := range matches {
			content := s.extractSchemaContent(match)
			if content != "" {
				schemaContent = append(schemaContent, content)
			}
		}
	}

	// Also look for common schema file patterns
	schemaExtensions := []string{".sql", ".prisma"}
	schemaFileNames := []string{"schema", "models", "migration", "create_"}

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip common non-schema directories
		relPath, _ := filepath.Rel(repoPath, path)
		if strings.Contains(relPath, "node_modules") || strings.Contains(relPath, ".git") || strings.Contains(relPath, "vendor") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		base := strings.ToLower(filepath.Base(path))

		for _, schemaExt := range schemaExtensions {
			if ext == schemaExt {
				content := s.extractSchemaContent(path)
				if content != "" {
					schemaContent = append(schemaContent, content)
				}
				return nil
			}
		}

		for _, namePattern := range schemaFileNames {
			if strings.Contains(base, namePattern) && (ext == ".py" || ext == ".rb" || ext == ".ts" || ext == ".go") {
				content := s.extractSchemaContent(path)
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

func (s *DatabaseSchemaService) extractSchemaContent(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	// Skip directories
	if info.IsDir() {
		return s.extractFromDirectory(path)
	}

	data, err := os.ReadFile(path)
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

func (s *DatabaseSchemaService) extractFromDirectory(dirPath string) string {
	var contents []string

	entries, err := os.ReadDir(dirPath)
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

		filePath := filepath.Join(dirPath, name)
		data, err := os.ReadFile(filePath)
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
