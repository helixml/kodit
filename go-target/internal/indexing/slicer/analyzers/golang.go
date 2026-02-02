package analyzers

import (
	"path/filepath"
	"strings"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/helixml/kodit/internal/indexing/slicer"
)

// Go implements Analyzer for Go code.
type Go struct {
	Base
}

// NewGo creates a new Go analyzer.
func NewGo(language slicer.Language) *Go {
	return &Go{
		Base: NewBase(language),
	}
}

// FunctionName extracts the function name from a function or method declaration.
func (g *Go) FunctionName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return g.NodeText(nameNode, source)
	}

	return ""
}

// IsPublic returns true if the function name starts with an uppercase letter.
func (g *Go) IsPublic(_ *sitter.Node, name string, _ []byte) bool {
	if name == "" {
		return false
	}

	firstRune := []rune(name)[0]
	return unicode.IsUpper(firstRune)
}

// IsMethod returns true if the node is a method_declaration.
func (g *Go) IsMethod(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	return node.Type() == "method_declaration"
}

// Docstring extracts the documentation comment preceding a function.
func (g *Go) Docstring(node *sitter.Node, source []byte) string {
	return g.ExtractPrecedingComment(node, source)
}

// ModulePath builds the module path from file information (package name).
func (g *Go) ModulePath(file slicer.ParsedFile) string {
	tree := file.Tree()
	if tree == nil {
		return ""
	}

	packageNodes := g.Walker().CollectNodes(tree.RootNode(), []string{"package_clause"})
	if len(packageNodes) == 0 {
		return ""
	}

	packageNode := packageNodes[0]
	nameNode := packageNode.ChildByFieldName("name")
	if nameNode != nil {
		return g.NodeText(nameNode, file.SourceCode())
	}

	return filepath.Base(filepath.Dir(file.Path()))
}

// Classes returns nil for Go (Go uses type definitions instead).
func (g *Go) Classes(_ *sitter.Tree, _ []byte) []slicer.ClassDefinition {
	return nil
}

// Types extracts type definitions from the AST.
func (g *Go) Types(tree *sitter.Tree, source []byte) []slicer.TypeDefinition {
	if tree == nil {
		return nil
	}

	typeNodes := g.Walker().CollectNodes(tree.RootNode(), []string{"type_declaration", "type_spec"})
	types := make([]slicer.TypeDefinition, 0, len(typeNodes))

	for _, node := range typeNodes {
		if node.Type() == "type_declaration" {
			for i := uint32(0); i < node.ChildCount(); i++ {
				child := node.Child(int(i))
				if child != nil && child.Type() == "type_spec" {
					typeDef := g.extractTypeSpec(child, source)
					types = append(types, typeDef)
				}
			}
		} else {
			typeDef := g.extractTypeSpec(node, source)
			types = append(types, typeDef)
		}
	}

	return types
}

func (g *Go) extractTypeSpec(node *sitter.Node, source []byte) slicer.TypeDefinition {
	nameNode := node.ChildByFieldName("name")
	name := ""
	if nameNode != nil {
		name = g.NodeText(nameNode, source)
	}

	kind := g.determineTypeKind(node, source)
	docstring := g.Docstring(node, source)
	constructorParams := g.extractStructFields(node, source)

	return slicer.NewTypeDefinition(
		"",
		node,
		node.StartByte(),
		node.EndByte(),
		name,
		name,
		kind,
		docstring,
		constructorParams,
	)
}

func (g *Go) determineTypeKind(node *sitter.Node, _ []byte) string {
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil {
		return "alias"
	}

	switch typeNode.Type() {
	case "struct_type":
		return "struct"
	case "interface_type":
		return "interface"
	case "map_type":
		return "map"
	case "channel_type":
		return "channel"
	case "function_type":
		return "func"
	case "slice_type", "array_type":
		return "slice"
	case "pointer_type":
		return "pointer"
	default:
		return "alias"
	}
}

func (g *Go) extractStructFields(node *sitter.Node, source []byte) []string {
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil || typeNode.Type() != "struct_type" {
		return nil
	}

	fieldNodes := g.Walker().CollectDescendants(typeNode, "field_declaration")
	var fields []string

	for _, fieldNode := range fieldNodes {
		nameNode := fieldNode.ChildByFieldName("name")
		typeFieldNode := fieldNode.ChildByFieldName("type")

		if nameNode != nil && typeFieldNode != nil {
			fieldName := g.NodeText(nameNode, source)
			fieldType := g.NodeText(typeFieldNode, source)
			fields = append(fields, fieldName+" "+fieldType)
		}
	}

	return fields
}

// ExtractReceiver extracts the receiver type from a method declaration.
func (g *Go) ExtractReceiver(node *sitter.Node, source []byte) string {
	if node == nil || node.Type() != "method_declaration" {
		return ""
	}

	receiver := node.ChildByFieldName("receiver")
	if receiver == nil {
		return ""
	}

	var typeName string
	g.Walker().Walk(receiver, func(n *sitter.Node) bool {
		if n.Type() == "type_identifier" {
			typeName = g.NodeText(n, source)
			return false
		}
		return true
	})

	return typeName
}

// ExtractParameters extracts function parameters.
func (g *Go) ExtractParameters(node *sitter.Node, source []byte) []string {
	params := node.ChildByFieldName("parameters")
	if params == nil {
		return nil
	}

	var result []string
	for i := uint32(0); i < params.ChildCount(); i++ {
		child := params.Child(int(i))
		if child == nil || child.Type() != "parameter_declaration" {
			continue
		}

		nameNode := child.ChildByFieldName("name")
		typeNode := child.ChildByFieldName("type")

		if nameNode != nil && typeNode != nil {
			param := g.NodeText(nameNode, source) + " " + g.NodeText(typeNode, source)
			result = append(result, strings.TrimSpace(param))
		} else if typeNode != nil {
			result = append(result, g.NodeText(typeNode, source))
		}
	}

	return result
}

// ExtractReturnType extracts the return type from a function.
func (g *Go) ExtractReturnType(node *sitter.Node, source []byte) string {
	result := node.ChildByFieldName("result")
	if result == nil {
		return ""
	}

	return g.NodeText(result, source)
}
