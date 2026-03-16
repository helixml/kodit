package extraction

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tsawler/tabula"
)

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

// DocumentText extracts plain text from binary document files using tabula.
type DocumentText struct{}

// NewDocumentText creates a new DocumentText.
func NewDocumentText() *DocumentText {
	return &DocumentText{}
}

// Text extracts readable text from the file at the given path.
func (d *DocumentText) Text(path string) (string, error) {
	text, _, err := tabula.Open(path).
		ExcludeHeadersAndFooters().
		JoinParagraphs().
		Text()
	if err != nil {
		return "", fmt.Errorf("extract text from %s: %w", filepath.Base(path), err)
	}
	return text, nil
}
