package analyzers

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/helixml/kodit/internal/indexing/slicer"
)

// C implements Analyzer for C code.
type C struct {
	Base
}

// NewC creates a new C analyzer.
func NewC(language slicer.Language) *C {
	return &C{
		Base: NewBase(language),
	}
}

// FunctionName extracts the function name from a function_definition node.
func (c *C) FunctionName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	declarator := node.ChildByFieldName("declarator")
	if declarator == nil {
		return ""
	}

	var name string
	c.Walker().Walk(declarator, func(n *sitter.Node) bool {
		if n.Type() == "identifier" {
			name = c.NodeText(n, source)
			return false
		}
		return true
	})

	return name
}

// IsPublic always returns true for C (no visibility modifiers).
func (c *C) IsPublic(_ *sitter.Node, _ string, _ []byte) bool {
	return true
}

// IsMethod returns false for C (no methods).
func (c *C) IsMethod(_ *sitter.Node) bool {
	return false
}

// Docstring extracts comments preceding a function.
func (c *C) Docstring(node *sitter.Node, source []byte) string {
	return c.ExtractPrecedingComment(node, source)
}

// ModulePath builds the module path from file information.
func (c *C) ModulePath(file slicer.ParsedFile) string {
	return c.BuildModulePathFromPath(file.Path(), ".c")
}

// Classes extracts struct/union/enum definitions.
func (c *C) Classes(tree *sitter.Tree, source []byte) []slicer.ClassDefinition {
	if tree == nil {
		return nil
	}

	structNodes := c.Walker().CollectNodes(tree.RootNode(), []string{"struct_specifier", "union_specifier", "enum_specifier"})
	classes := make([]slicer.ClassDefinition, 0, len(structNodes))

	for _, node := range structNodes {
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}

		name := c.NodeText(nameNode, source)
		docstring := c.Docstring(node, source)

		class := slicer.NewClassDefinition(
			"",
			node,
			node.StartByte(),
			node.EndByte(),
			name,
			name,
			true,
			docstring,
			nil,
			nil,
			nil,
		)
		classes = append(classes, class)
	}

	return classes
}

// Types extracts typedef definitions.
func (c *C) Types(tree *sitter.Tree, source []byte) []slicer.TypeDefinition {
	if tree == nil {
		return nil
	}

	typedefNodes := c.Walker().CollectNodes(tree.RootNode(), []string{"type_definition"})
	types := make([]slicer.TypeDefinition, 0, len(typedefNodes))

	for _, node := range typedefNodes {
		declarator := node.ChildByFieldName("declarator")
		if declarator == nil {
			continue
		}

		var name string
		c.Walker().Walk(declarator, func(n *sitter.Node) bool {
			if n.Type() == "type_identifier" || n.Type() == "identifier" {
				name = c.NodeText(n, source)
				return false
			}
			return true
		})

		if name == "" {
			continue
		}

		docstring := c.Docstring(node, source)

		typeDef := slicer.NewTypeDefinition(
			"",
			node,
			node.StartByte(),
			node.EndByte(),
			name,
			name,
			"typedef",
			docstring,
			nil,
		)
		types = append(types, typeDef)
	}

	return types
}

// ExtractParameters extracts function parameters.
func (c *C) ExtractParameters(node *sitter.Node, source []byte) []string {
	declarator := node.ChildByFieldName("declarator")
	if declarator == nil {
		return nil
	}

	params := declarator.ChildByFieldName("parameters")
	if params == nil {
		return nil
	}

	var result []string
	paramNodes := c.Walker().CollectNodes(params, []string{"parameter_declaration"})

	for _, paramNode := range paramNodes {
		result = append(result, c.NodeText(paramNode, source))
	}

	return result
}

// ExtractReturnType extracts the return type from a function.
func (c *C) ExtractReturnType(node *sitter.Node, source []byte) string {
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil {
		return ""
	}

	return c.NodeText(typeNode, source)
}
