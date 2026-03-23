package extraction

import "strings"

// TextExtractor converts raw file bytes into indexable plain text.
// Returns empty string when the content should be skipped.
type TextExtractor interface {
	Text(content []byte) (string, error)
}

// Extractors maps file extensions to text extractors.
type Extractors struct {
	registered map[string]TextExtractor
	fallback   TextExtractor
}

// NewExtractors creates an Extractors with CSV and plain-text extractors.
func NewExtractors() *Extractors {
	return &Extractors{
		registered: map[string]TextExtractor{
			".csv": NewCSVText(),
		},
		fallback: NewSourceText(),
	}
}

// For returns the text extractor for the given file extension.
func (e *Extractors) For(ext string) TextExtractor {
	if t, ok := e.registered[strings.ToLower(ext)]; ok {
		return t
	}
	return e.fallback
}
