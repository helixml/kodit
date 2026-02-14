package example

import (
	"path/filepath"
	"strings"
)

// Discovery detects example files in repositories.
type Discovery struct {
	exampleDirectories      map[string]bool
	documentationExtensions map[string]bool
}

// NewDiscovery creates a new Discovery service.
func NewDiscovery() *Discovery {
	return &Discovery{
		exampleDirectories: map[string]bool{
			"examples":  true,
			"example":   true,
			"samples":   true,
			"sample":    true,
			"demos":     true,
			"demo":      true,
			"tutorials": true,
			"tutorial":  true,
		},
		documentationExtensions: map[string]bool{
			".md":       true,
			".markdown": true,
			".rst":      true,
			".adoc":     true,
			".asciidoc": true,
		},
	}
}

// IsExampleDirectoryFile checks if file is in an example directory.
func (d *Discovery) IsExampleDirectoryFile(filePath string) bool {
	parts := strings.Split(filePath, string(filepath.Separator))
	for _, part := range parts {
		if d.exampleDirectories[strings.ToLower(part)] {
			return true
		}
	}
	return false
}

// IsDocumentationFile checks if file is a documentation file.
func (d *Discovery) IsDocumentationFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return d.documentationExtensions[ext]
}

// IsExampleCandidate checks if file should be processed for examples.
func (d *Discovery) IsExampleCandidate(filePath string) bool {
	return d.IsExampleDirectoryFile(filePath) || d.IsDocumentationFile(filePath)
}
