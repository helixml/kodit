package extraction

import "bytes"

const binaryProbeSize = 8192

// SourceText treats file content as plain text.
// Binary files (containing null bytes in the first 8 KB) produce empty text.
type SourceText struct{}

// NewSourceText creates a SourceText.
func NewSourceText() *SourceText {
	return &SourceText{}
}

// Text returns the content as a string, or empty if the content appears binary.
func (s *SourceText) Text(content []byte) (string, error) {
	probe := content
	if len(probe) > binaryProbeSize {
		probe = probe[:binaryProbeSize]
	}
	if bytes.ContainsRune(probe, 0) {
		return "", nil
	}
	return string(content), nil
}
