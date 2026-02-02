package analyzers

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/helixml/kodit/internal/indexing/slicer"
)

// TypeScript implements Analyzer for TypeScript code.
type TypeScript struct {
	JavaScript
	extension string
}

// NewTypeScript creates a new TypeScript analyzer.
func NewTypeScript(language slicer.Language) *TypeScript {
	return &TypeScript{
		JavaScript: JavaScript{
			Base: NewBase(language),
		},
		extension: ".ts",
	}
}

// NewTSX creates a new TSX analyzer.
func NewTSX(language slicer.Language) *TypeScript {
	return &TypeScript{
		JavaScript: JavaScript{
			Base: NewBase(language),
		},
		extension: ".tsx",
	}
}

// ModulePath builds the module path from file information.
func (t *TypeScript) ModulePath(file slicer.ParsedFile) string {
	return t.BuildModulePathFromPath(file.Path(), t.extension)
}

// Types extracts type definitions from the AST.
func (t *TypeScript) Types(tree *sitter.Tree, source []byte) []slicer.TypeDefinition {
	if tree == nil {
		return nil
	}

	typeNodes := t.Walker().CollectNodes(tree.RootNode(), []string{"type_alias_declaration", "interface_declaration"})
	types := make([]slicer.TypeDefinition, 0, len(typeNodes))

	for _, node := range typeNodes {
		typeDef := t.extractTypeDef(node, source)
		types = append(types, typeDef)
	}

	return types
}

func (t *TypeScript) extractTypeDef(node *sitter.Node, source []byte) slicer.TypeDefinition {
	nameNode := node.ChildByFieldName("name")
	name := ""
	if nameNode != nil {
		name = t.NodeText(nameNode, source)
	}

	kind := "alias"
	if node.Type() == "interface_declaration" {
		kind = "interface"
	}

	docstring := t.Docstring(node, source)

	return slicer.NewTypeDefinition(
		"",
		node,
		node.StartByte(),
		node.EndByte(),
		name,
		name,
		kind,
		docstring,
		nil,
	)
}

// ExtractTypeReferences extracts type names referenced in a node.
func (t *TypeScript) ExtractTypeReferences(node *sitter.Node, source []byte) []string {
	if node == nil {
		return nil
	}

	typeRefSet := make(map[string]struct{})
	typeNodes := t.Walker().CollectDescendants(node, "type_identifier")

	for _, typeNode := range typeNodes {
		typeName := t.NodeText(typeNode, source)
		if typeName != "" && !isBuiltinType(typeName) {
			typeRefSet[typeName] = struct{}{}
		}
	}

	result := make([]string, 0, len(typeRefSet))
	for name := range typeRefSet {
		result = append(result, name)
	}

	return result
}

func isBuiltinType(name string) bool {
	builtins := map[string]struct{}{
		"string":    {},
		"number":    {},
		"boolean":   {},
		"void":      {},
		"null":      {},
		"undefined": {},
		"any":       {},
		"never":     {},
		"unknown":   {},
		"object":    {},
		"symbol":    {},
		"bigint":    {},
		"Array":     {},
		"Promise":   {},
		"Map":       {},
		"Set":       {},
		"Function":  {},
		"Object":    {},
		"String":    {},
		"Number":    {},
		"Boolean":   {},
	}

	_, ok := builtins[name]
	return ok
}

// ExtractJSXReturns extracts JSX return statements (for TSX).
func (t *TypeScript) ExtractJSXReturns(node *sitter.Node, source []byte) []string {
	if node == nil {
		return nil
	}

	returnNodes := t.Walker().CollectDescendants(node, "return_statement")
	var jsxReturns []string

	for _, returnNode := range returnNodes {
		for i := uint32(0); i < returnNode.ChildCount(); i++ {
			child := returnNode.Child(int(i))
			if child != nil && strings.HasPrefix(child.Type(), "jsx") {
				jsxReturns = append(jsxReturns, t.NodeText(child, source))
			}
		}
	}

	return jsxReturns
}
