// Package example provides extraction of code examples from documentation.
package example

// CodeBlock represents a code block extracted from documentation.
type CodeBlock struct {
	content   string
	language  string
	lineStart int
	lineEnd   int
	context   string
}

// NewCodeBlock creates a new CodeBlock.
func NewCodeBlock(content, language string, lineStart, lineEnd int, context string) CodeBlock {
	return CodeBlock{
		content:   content,
		language:  language,
		lineStart: lineStart,
		lineEnd:   lineEnd,
		context:   context,
	}
}

// Content returns the code content.
func (b CodeBlock) Content() string { return b.content }

// Language returns the programming language.
func (b CodeBlock) Language() string { return b.language }

// LineStart returns the starting line number.
func (b CodeBlock) LineStart() int { return b.lineStart }

// LineEnd returns the ending line number.
func (b CodeBlock) LineEnd() int { return b.lineEnd }

// Context returns the surrounding context (heading or paragraph).
func (b CodeBlock) Context() string { return b.context }

// HasLanguage returns true if a language is specified.
func (b CodeBlock) HasLanguage() bool { return b.language != "" }

// HasContext returns true if context is available.
func (b CodeBlock) HasContext() bool { return b.context != "" }
