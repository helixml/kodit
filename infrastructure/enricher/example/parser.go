package example

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Parser extracts code blocks from documentation content.
type Parser interface {
	Parse(content string) []CodeBlock
}

// MarkdownParser parses Markdown documentation.
type MarkdownParser struct {
	codeBlockPattern *regexp.Regexp
}

// NewMarkdownParser creates a new MarkdownParser.
func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{
		codeBlockPattern: regexp.MustCompile(`^` + "```" + `(\w+)?`),
	}
}

// Parse extracts code blocks from Markdown content.
func (p *MarkdownParser) Parse(content string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(content, "\n")
	i := 0

	for i < len(lines) {
		line := lines[i]
		matches := p.codeBlockPattern.FindStringSubmatch(line)

		if matches != nil {
			language := ""
			if len(matches) > 1 {
				language = matches[1]
			}
			lineStart := i + 1
			var codeLines []string
			i++

			for i < len(lines) && !strings.HasPrefix(lines[i], "```") {
				codeLines = append(codeLines, lines[i])
				i++
			}

			if len(codeLines) > 0 {
				context := p.findContext(lines, lineStart-1)
				blocks = append(blocks, NewCodeBlock(
					strings.Join(codeLines, "\n"),
					language,
					lineStart,
					i-1,
					context,
				))
			}
		}
		i++
	}

	return blocks
}

func (p *MarkdownParser) findContext(lines []string, blockLine int) string {
	var heading string

	start := blockLine - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < blockLine; i++ {
		if strings.HasPrefix(lines[i], "#") {
			heading = strings.TrimSpace(strings.TrimLeft(lines[i], "#"))
		}
	}

	if heading != "" {
		return heading
	}

	var paragraphLines []string
	start = blockLine - 3
	if start < 0 {
		start = 0
	}

	for i := start; i < blockLine; i++ {
		stripped := strings.TrimSpace(lines[i])
		if stripped != "" && !strings.HasPrefix(stripped, "#") {
			paragraphLines = append(paragraphLines, stripped)
		}
	}

	if len(paragraphLines) > 0 {
		return strings.Join(paragraphLines, " ")
	}

	return ""
}

// RstParser parses reStructuredText documentation.
type RstParser struct {
	directivePattern *regexp.Regexp
}

// NewRstParser creates a new RstParser.
func NewRstParser() *RstParser {
	return &RstParser{
		directivePattern: regexp.MustCompile(`^\.\.\s+(code-block|code)::\s*(\w+)?`),
	}
}

// Parse extracts code blocks from RST content.
func (p *RstParser) Parse(content string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(content, "\n")
	i := 0

	for i < len(lines) {
		line := lines[i]
		matches := p.directivePattern.FindStringSubmatch(line)

		if matches != nil {
			language := ""
			if len(matches) > 2 {
				language = matches[2]
			}
			i++

			// Skip blank lines
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}

			if i >= len(lines) {
				break
			}

			baseIndent := len(lines[i]) - len(strings.TrimLeft(lines[i], " \t"))
			lineStart := i
			var codeLines []string

			for i < len(lines) {
				currentLine := lines[i]
				if strings.TrimSpace(currentLine) == "" {
					i++
					continue
				}

				currentIndent := len(currentLine) - len(strings.TrimLeft(currentLine, " \t"))
				if currentIndent < baseIndent {
					break
				}

				if len(currentLine) >= baseIndent {
					codeLines = append(codeLines, currentLine[baseIndent:])
				}
				i++
			}

			if len(codeLines) > 0 {
				blocks = append(blocks, NewCodeBlock(
					strings.Join(codeLines, "\n"),
					language,
					lineStart,
					i-1,
					"",
				))
			}
		} else {
			i++
		}
	}

	return blocks
}

// ParserForExtension returns the appropriate parser for a file extension.
func ParserForExtension(extension string) Parser {
	ext := strings.ToLower(extension)
	switch ext {
	case ".md", ".markdown":
		return NewMarkdownParser()
	case ".rst":
		return NewRstParser()
	default:
		return nil
	}
}

// ParserForFile returns the appropriate parser for a file path.
func ParserForFile(filePath string) Parser {
	return ParserForExtension(filepath.Ext(filePath))
}
