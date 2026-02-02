package analyzers

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/helixml/kodit/internal/indexing/slicer"
)

// CPP implements Analyzer for C++ code.
type CPP struct {
	C
}

// NewCPP creates a new C++ analyzer.
func NewCPP(language slicer.Language) *CPP {
	return &CPP{
		C: C{
			Base: NewBase(language),
		},
	}
}

// ModulePath builds the module path from file information.
func (c *CPP) ModulePath(file slicer.ParsedFile) string {
	return c.BuildModulePathFromPath(file.Path(), ".cpp")
}

// Classes extracts class/struct definitions including methods.
func (c *CPP) Classes(tree *sitter.Tree, source []byte) []slicer.ClassDefinition {
	if tree == nil {
		return nil
	}

	classNodes := c.Walker().CollectNodes(tree.RootNode(), []string{"class_specifier", "struct_specifier"})
	classes := make([]slicer.ClassDefinition, 0, len(classNodes))

	for _, node := range classNodes {
		class := c.extractClass(node, source)
		classes = append(classes, class)
	}

	return classes
}

func (c *CPP) extractClass(node *sitter.Node, source []byte) slicer.ClassDefinition {
	nameNode := node.ChildByFieldName("name")
	name := ""
	if nameNode != nil {
		name = c.NodeText(nameNode, source)
	}

	docstring := c.Docstring(node, source)
	bases := c.extractBases(node, source)
	methods := c.extractMethods(node, source, name)

	return slicer.NewClassDefinition(
		"",
		node,
		node.StartByte(),
		node.EndByte(),
		name,
		name,
		true,
		docstring,
		bases,
		methods,
		nil,
	)
}

func (c *CPP) extractBases(node *sitter.Node, source []byte) []string {
	baseClause := node.ChildByFieldName("base_clause")
	if baseClause == nil {
		return nil
	}

	var bases []string
	c.Walker().Walk(baseClause, func(n *sitter.Node) bool {
		if n.Type() == "type_identifier" {
			bases = append(bases, c.NodeText(n, source))
		}
		return true
	})

	return bases
}

func (c *CPP) extractMethods(classNode *sitter.Node, source []byte, className string) []slicer.FunctionDefinition {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return nil
	}

	funcNodes := c.Walker().CollectNodes(body, []string{"function_definition", "declaration"})
	methods := make([]slicer.FunctionDefinition, 0, len(funcNodes))

	for _, funcNode := range funcNodes {
		name := c.FunctionName(funcNode, source)
		if name == "" {
			continue
		}

		qualifiedName := className + "::" + name
		docstring := c.Docstring(funcNode, source)
		params := c.ExtractParameters(funcNode, source)
		returnType := c.ExtractReturnType(funcNode, source)

		method := slicer.NewFunctionDefinition(
			"",
			funcNode,
			funcNode.StartByte(),
			funcNode.EndByte(),
			qualifiedName,
			name,
			true,
			true,
			docstring,
			params,
			returnType,
		)
		methods = append(methods, method)
	}

	return methods
}

// Types extracts type alias declarations.
func (c *CPP) Types(tree *sitter.Tree, source []byte) []slicer.TypeDefinition {
	if tree == nil {
		return nil
	}

	aliasNodes := c.Walker().CollectNodes(tree.RootNode(), []string{"type_definition", "alias_declaration"})
	types := make([]slicer.TypeDefinition, 0, len(aliasNodes))

	for _, node := range aliasNodes {
		var name string

		if node.Type() == "alias_declaration" {
			nameNode := node.ChildByFieldName("name")
			if nameNode != nil {
				name = c.NodeText(nameNode, source)
			}
		} else {
			declarator := node.ChildByFieldName("declarator")
			if declarator != nil {
				c.Walker().Walk(declarator, func(n *sitter.Node) bool {
					if n.Type() == "type_identifier" || n.Type() == "identifier" {
						name = c.NodeText(n, source)
						return false
					}
					return true
				})
			}
		}

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
			"alias",
			docstring,
			nil,
		)
		types = append(types, typeDef)
	}

	return types
}
