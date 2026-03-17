package extraction

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tsawler/tabula"
)

// maxDocumentSize is the largest file (100 MB) we will attempt to extract text from.
const maxDocumentSize = 100 * 1024 * 1024

// documentExtensions lists binary document formats that tabula can extract text from.
var documentExtensions = map[string]bool{
	".pdf":  true,
	".docx": true,
	".odt":  true,
	".xlsx": true,
	".pptx": true,
	".epub": true,
}

// IsDocument returns true if the extension is a supported document format.
func IsDocument(ext string) bool {
	return documentExtensions[strings.ToLower(ext)]
}

// Extensions returns the supported document extensions (e.g. ".pdf", ".docx").
func Extensions() []string {
	out := make([]string, 0, len(documentExtensions))
	for ext := range documentExtensions {
		out = append(out, ext)
	}
	return out
}

// DocumentText extracts plain text from binary document files using tabula.
type DocumentText struct{}

// NewDocumentText creates a new DocumentText.
func NewDocumentText() *DocumentText {
	return &DocumentText{}
}

// Text extracts readable text from the file at the given path.
// It validates that the file exists, has a supported extension, and is
// within the maximum size limit before passing it to tabula.
func (d *DocumentText) Text(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if !IsDocument(ext) {
		return "", fmt.Errorf("unsupported document type %q for %s", ext, filepath.Base(path))
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", filepath.Base(path), err)
	}
	if info.Size() > maxDocumentSize {
		return "", fmt.Errorf("%s exceeds maximum document size (%d bytes)", filepath.Base(path), maxDocumentSize)
	}

	text, _, err := tabula.Open(path).
		ExcludeHeadersAndFooters().
		JoinParagraphs().
		Text()
	if err != nil {
		return "", fmt.Errorf("extract text from %s: %w", filepath.Base(path), err)
	}
	return text, nil
}
