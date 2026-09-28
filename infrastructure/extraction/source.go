package extraction

import (
	"bytes"
	"strings"
)

const binaryProbeSize = 8192

// SourceText treats file content as plain text.
// Binary files (containing null bytes in the first 8 KB) produce empty text.
type SourceText struct{}

// NewSourceText creates a SourceText.
func NewSourceText() *SourceText {
	return &SourceText{}
}

// Text returns the content as a string, or empty if the content appears binary.
//
// Non-UTF-8 byte sequences (e.g. a source file saved as Windows-1252/Latin-1,
// which some legacy repos have) are replaced with U+FFFD rather than passed
// through raw -- persistence downstream is a Postgres TEXT column with UTF-8
// client/server encoding, which rejects invalid sequences outright rather
// than storing them, failing the whole commit's remaining chunk extraction.
func (s *SourceText) Text(content []byte) (string, error) {
	probe := content
	if len(probe) > binaryProbeSize {
		probe = probe[:binaryProbeSize]
	}
	if bytes.ContainsRune(probe, 0) {
		return "", nil
	}
	return strings.ToValidUTF8(string(content), "�"), nil
}
