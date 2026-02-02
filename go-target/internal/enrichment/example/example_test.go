package example

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeBlock(t *testing.T) {
	t.Run("creates code block with all fields", func(t *testing.T) {
		block := NewCodeBlock("func main() {}", "go", 10, 15, "Example usage")

		assert.Equal(t, "func main() {}", block.Content())
		assert.Equal(t, "go", block.Language())
		assert.Equal(t, 10, block.LineStart())
		assert.Equal(t, 15, block.LineEnd())
		assert.Equal(t, "Example usage", block.Context())
		assert.True(t, block.HasLanguage())
		assert.True(t, block.HasContext())
	})

	t.Run("handles empty language and context", func(t *testing.T) {
		block := NewCodeBlock("some code", "", 1, 1, "")

		assert.False(t, block.HasLanguage())
		assert.False(t, block.HasContext())
	})
}

func TestDiscovery(t *testing.T) {
	discovery := NewDiscovery()

	t.Run("detects example directory files", func(t *testing.T) {
		testCases := []struct {
			path     string
			expected bool
		}{
			{"examples/main.go", true},
			{"example/test.py", true},
			{"samples/demo.js", true},
			{"sample/app.rs", true},
			{"demos/hello.go", true},
			{"demo/world.py", true},
			{"tutorials/guide.md", true},
			{"tutorial/intro.rst", true},
			{"src/main.go", false},
			{"lib/utils.py", false},
			{"docs/api.md", false},
		}

		for _, tc := range testCases {
			result := discovery.IsExampleDirectoryFile(tc.path)
			assert.Equal(t, tc.expected, result, "path: %s", tc.path)
		}
	})

	t.Run("detects documentation files", func(t *testing.T) {
		testCases := []struct {
			path     string
			expected bool
		}{
			{"README.md", true},
			{"docs/guide.markdown", true},
			{"docs/api.rst", true},
			{"docs/book.adoc", true},
			{"docs/manual.asciidoc", true},
			{"main.go", false},
			{"test.py", false},
			{"config.json", false},
		}

		for _, tc := range testCases {
			result := discovery.IsDocumentationFile(tc.path)
			assert.Equal(t, tc.expected, result, "path: %s", tc.path)
		}
	})

	t.Run("identifies example candidates", func(t *testing.T) {
		testCases := []struct {
			path     string
			expected bool
		}{
			{"examples/main.go", true},
			{"README.md", true},
			{"docs/guide.rst", true},
			{"src/main.go", false},
			{"lib/utils.py", false},
		}

		for _, tc := range testCases {
			result := discovery.IsExampleCandidate(tc.path)
			assert.Equal(t, tc.expected, result, "path: %s", tc.path)
		}
	})
}

func TestMarkdownParser(t *testing.T) {
	parser := NewMarkdownParser()

	t.Run("parses simple code block", func(t *testing.T) {
		content := "# Example\n\n```go\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```"

		blocks := parser.Parse(content)

		assert.Len(t, blocks, 1)
		assert.Equal(t, "go", blocks[0].Language())
		assert.Contains(t, blocks[0].Content(), "func main()")
		assert.Equal(t, "Example", blocks[0].Context())
	})

	t.Run("parses multiple code blocks", func(t *testing.T) {
		content := "```python\nprint('hello')\n```\n\n```javascript\nconsole.log('world')\n```"

		blocks := parser.Parse(content)

		assert.Len(t, blocks, 2)
		assert.Equal(t, "python", blocks[0].Language())
		assert.Equal(t, "javascript", blocks[1].Language())
	})

	t.Run("parses code block without language", func(t *testing.T) {
		content := "```\nsome code\n```"

		blocks := parser.Parse(content)

		assert.Len(t, blocks, 1)
		assert.Equal(t, "", blocks[0].Language())
		assert.Equal(t, "some code", blocks[0].Content())
	})

	t.Run("uses preceding paragraph as context when no heading", func(t *testing.T) {
		content := "Here is an example of how to use the library:\n\n```go\nfoo.Bar()\n```"

		blocks := parser.Parse(content)

		assert.Len(t, blocks, 1)
		assert.Contains(t, blocks[0].Context(), "Here is an example")
	})

	t.Run("handles empty content", func(t *testing.T) {
		blocks := parser.Parse("")
		assert.Len(t, blocks, 0)
	})

	t.Run("handles content without code blocks", func(t *testing.T) {
		content := "# Just a Heading\n\nSome regular text."
		blocks := parser.Parse(content)
		assert.Len(t, blocks, 0)
	})
}

func TestRstParser(t *testing.T) {
	parser := NewRstParser()

	t.Run("parses code-block directive", func(t *testing.T) {
		content := ".. code-block:: python\n\n    def hello():\n        print('world')"

		blocks := parser.Parse(content)

		assert.Len(t, blocks, 1)
		assert.Equal(t, "python", blocks[0].Language())
		assert.Contains(t, blocks[0].Content(), "def hello()")
	})

	t.Run("parses code directive", func(t *testing.T) {
		content := ".. code:: go\n\n    func main() {}"

		blocks := parser.Parse(content)

		assert.Len(t, blocks, 1)
		assert.Equal(t, "go", blocks[0].Language())
	})

	t.Run("parses directive without language", func(t *testing.T) {
		content := ".. code-block::\n\n    some code"

		blocks := parser.Parse(content)

		assert.Len(t, blocks, 1)
		assert.Equal(t, "", blocks[0].Language())
	})

	t.Run("handles multiple code blocks", func(t *testing.T) {
		content := ".. code-block:: python\n\n    first\n\n.. code-block:: go\n\n    second"

		blocks := parser.Parse(content)

		assert.Len(t, blocks, 2)
	})

	t.Run("handles empty content", func(t *testing.T) {
		blocks := parser.Parse("")
		assert.Len(t, blocks, 0)
	})
}

func TestParserFactory(t *testing.T) {
	t.Run("returns markdown parser for md files", func(t *testing.T) {
		parser := ParserForExtension(".md")
		assert.NotNil(t, parser)
		_, ok := parser.(*MarkdownParser)
		assert.True(t, ok)
	})

	t.Run("returns markdown parser for markdown files", func(t *testing.T) {
		parser := ParserForExtension(".markdown")
		assert.NotNil(t, parser)
		_, ok := parser.(*MarkdownParser)
		assert.True(t, ok)
	})

	t.Run("returns rst parser for rst files", func(t *testing.T) {
		parser := ParserForExtension(".rst")
		assert.NotNil(t, parser)
		_, ok := parser.(*RstParser)
		assert.True(t, ok)
	})

	t.Run("returns nil for unsupported extensions", func(t *testing.T) {
		parser := ParserForExtension(".go")
		assert.Nil(t, parser)
	})

	t.Run("ParserForFile works with full paths", func(t *testing.T) {
		parser := ParserForFile("/docs/README.md")
		assert.NotNil(t, parser)
	})
}
