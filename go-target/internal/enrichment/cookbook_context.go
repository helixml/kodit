package enrichment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// CookbookContextService gathers context for cookbook generation.
type CookbookContextService struct{}

// NewCookbookContextService creates a new CookbookContextService.
func NewCookbookContextService() *CookbookContextService {
	return &CookbookContextService{}
}

// Gather collects all relevant context for cookbook generation.
func (s *CookbookContextService) Gather(ctx context.Context, repoPath, language string) (string, error) {
	var sections []string

	sections = append(sections, "## Primary Language\n"+language)

	readme := s.extractReadmeContent(repoPath)
	if readme != "" {
		sections = append(sections, "## README\n"+readme)
	}

	manifest := s.extractPackageManifest(repoPath)
	if manifest != "" {
		sections = append(sections, "## Package Information\n"+manifest)
	}

	examples := s.findExistingExamples(repoPath)
	if examples != "" {
		sections = append(sections, "## Existing Examples Found\n"+examples)
	}

	if len(sections) == 0 {
		return "No context available", nil
	}

	return strings.Join(sections, "\n\n"), nil
}

func (s *CookbookContextService) extractReadmeContent(repoPath string) string {
	readmeNames := []string{"README.md", "README.rst", "README.txt", "README"}

	for _, name := range readmeNames {
		readmePath := filepath.Join(repoPath, name)
		data, err := os.ReadFile(readmePath)
		if err != nil {
			continue
		}

		content := string(data)
		if len(content) > 3000 {
			content = content[:3000] + "\n...[truncated]"
		}
		return content
	}

	return "No README found"
}

func (s *CookbookContextService) extractPackageManifest(repoPath string) string {
	var manifestInfo []string

	pyproject := filepath.Join(repoPath, "pyproject.toml")
	if data, err := os.ReadFile(pyproject); err == nil {
		content := string(data)
		if len(content) > 500 {
			content = content[:500]
		}
		manifestInfo = append(manifestInfo, "Python project (pyproject.toml):\n"+content)
	}

	setupPy := filepath.Join(repoPath, "setup.py")
	if len(manifestInfo) == 0 {
		if data, err := os.ReadFile(setupPy); err == nil {
			content := string(data)
			if len(content) > 500 {
				content = content[:500]
			}
			manifestInfo = append(manifestInfo, "Python project (setup.py):\n"+content)
		}
	}

	packageJSON := filepath.Join(repoPath, "package.json")
	if data, err := os.ReadFile(packageJSON); err == nil {
		content := string(data)
		if len(content) > 500 {
			content = content[:500]
		}
		manifestInfo = append(manifestInfo, "Node.js project (package.json):\n"+content)
	}

	goMod := filepath.Join(repoPath, "go.mod")
	if data, err := os.ReadFile(goMod); err == nil {
		content := string(data)
		if len(content) > 500 {
			content = content[:500]
		}
		manifestInfo = append(manifestInfo, "Go project (go.mod):\n"+content)
	}

	cargoToml := filepath.Join(repoPath, "Cargo.toml")
	if data, err := os.ReadFile(cargoToml); err == nil {
		content := string(data)
		if len(content) > 500 {
			content = content[:500]
		}
		manifestInfo = append(manifestInfo, "Rust project (Cargo.toml):\n"+content)
	}

	if len(manifestInfo) > 0 {
		return strings.Join(manifestInfo, "\n\n")
	}
	return "No package manifest found"
}

func (s *CookbookContextService) findExistingExamples(repoPath string) string {
	var exampleLocations []string

	exampleDirs := []string{"examples", "example", "docs/examples", "samples"}
	extensions := []string{".py", ".js", ".ts", ".go", ".rs"}

	for _, exampleDir := range exampleDirs {
		examplePath := filepath.Join(repoPath, exampleDir)
		info, err := os.Stat(examplePath)
		if err != nil || !info.IsDir() {
			continue
		}

		var exampleFiles []string
		err = filepath.Walk(examplePath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			for _, validExt := range extensions {
				if ext == validExt {
					exampleFiles = append(exampleFiles, path)
					break
				}
			}
			return nil
		})
		if err != nil {
			continue
		}

		if len(exampleFiles) > 0 {
			exampleLocations = append(exampleLocations,
				"Found "+string(rune('0'+len(exampleFiles)))+" example files in "+exampleDir+"/")

			data, err := os.ReadFile(exampleFiles[0])
			if err == nil {
				content := string(data)
				if len(content) > 500 {
					content = content[:500]
				}
				exampleLocations = append(exampleLocations,
					"Sample from "+filepath.Base(exampleFiles[0])+":\n```\n"+content+"\n```")
			}
		}
	}

	if len(exampleLocations) > 0 {
		return strings.Join(exampleLocations, "\n")
	}
	return "No examples directory found"
}
