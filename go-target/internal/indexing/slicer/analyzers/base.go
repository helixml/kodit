// Package analyzers provides language-specific AST analyzers.
package analyzers

import (
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/helixml/kodit/internal/indexing/slicer"
)

// Base provides common analyzer functionality.
type Base struct {
	language slicer.Language
	walker   slicer.Walker
}

// NewBase creates a new Base analyzer.
func NewBase(language slicer.Language) Base {
	return Base{
		language: language,
		walker:   slicer.NewWalker(),
	}
}

// Language returns the language configuration.
func (b Base) Language() slicer.Language { return b.language }

// Walker returns the AST walker.
func (b Base) Walker() slicer.Walker { return b.walker }

// NodeText extracts text from a node.
func (b Base) NodeText(node *sitter.Node, source []byte) string {
	return b.walker.NodeText(node, source)
}

// ExtractIdentifier extracts an identifier from a node using the language's name field.
func (b Base) ExtractIdentifier(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	nameField := b.language.Nodes().NameField()
	if nameField == "" {
		nameField = "name"
	}

	nameNode := node.ChildByFieldName(nameField)
	if nameNode != nil {
		return b.NodeText(nameNode, source)
	}

	if b.walker.IsIdentifier(node) {
		return b.NodeText(node, source)
	}

	return ""
}

// ExtractPrecedingComment extracts comment text from nodes preceding the given node.
func (b Base) ExtractPrecedingComment(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	var comments []string
	prev := node.PrevSibling()

	for prev != nil && b.walker.IsComment(prev) {
		text := b.NodeText(prev, source)
		text = cleanComment(text)
		if text != "" {
			comments = append([]string{text}, comments...)
		}
		prev = prev.PrevSibling()
	}

	return strings.Join(comments, "\n")
}

// ExtractFirstChildComment extracts the first child comment/string (for Python docstrings).
func (b Base) ExtractFirstChildComment(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	body := node.ChildByFieldName("body")
	if body == nil {
		return ""
	}

	for i := uint32(0); i < body.ChildCount(); i++ {
		child := body.Child(int(i))
		if child == nil {
			continue
		}

		if child.Type() == "expression_statement" {
			if child.ChildCount() > 0 {
				expr := child.Child(0)
				if expr != nil && b.walker.IsString(expr) {
					text := b.NodeText(expr, source)
					return cleanDocstring(text)
				}
			}
		}

		if !b.walker.IsComment(child) {
			break
		}
	}

	return ""
}

// BuildQualifiedName builds a qualified name from module path and simple name.
func (b Base) BuildQualifiedName(modulePath, simpleName string) string {
	if modulePath == "" {
		return simpleName
	}
	return modulePath + "." + simpleName
}

// BuildModulePathFromPath builds a module path from a file path.
func (b Base) BuildModulePathFromPath(filePath, extension string) string {
	base := filepath.Base(filePath)
	name := strings.TrimSuffix(base, extension)

	dir := filepath.Dir(filePath)
	parts := strings.Split(dir, string(filepath.Separator))

	var moduleParts []string
	for _, part := range parts {
		if part != "" && part != "." && part != ".." {
			moduleParts = append(moduleParts, part)
		}
	}
	moduleParts = append(moduleParts, name)

	return strings.Join(moduleParts, ".")
}

func cleanComment(text string) string {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "//") {
		text = strings.TrimPrefix(text, "//")
	} else if strings.HasPrefix(text, "#") {
		text = strings.TrimPrefix(text, "#")
	} else if strings.HasPrefix(text, "/*") && strings.HasSuffix(text, "*/") {
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
	}

	return strings.TrimSpace(text)
}

func cleanDocstring(text string) string {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`) {
		text = strings.TrimPrefix(text, `"""`)
		text = strings.TrimSuffix(text, `"""`)
	} else if strings.HasPrefix(text, "'''") && strings.HasSuffix(text, "'''") {
		text = strings.TrimPrefix(text, "'''")
		text = strings.TrimSuffix(text, "'''")
	} else if strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) {
		text = strings.TrimPrefix(text, `"`)
		text = strings.TrimSuffix(text, `"`)
	} else if strings.HasPrefix(text, "'") && strings.HasSuffix(text, "'") {
		text = strings.TrimPrefix(text, "'")
		text = strings.TrimSuffix(text, "'")
	}

	return strings.TrimSpace(text)
}
