package language

import (
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/helixml/kodit/infrastructure/slicing"
)

// CSharp implements Analyzer for C# code.
type CSharp struct {
	Base
}

// NewCSharp creates a new C# analyzer.
func NewCSharp(language slicing.Language) *CSharp {
	return &CSharp{
		Base: NewBase(language),
	}
}

// FunctionName extracts the method name from a method_declaration node.
func (cs *CSharp) FunctionName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return cs.NodeText(nameNode, source)
	}

	return ""
}

// IsPublic always returns true for C# (we index all methods).
func (cs *CSharp) IsPublic(_ *sitter.Node, _ string, _ []byte) bool {
	return true
}

// IsMethod returns true if node is a constructor_declaration.
func (cs *CSharp) IsMethod(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	return node.Type() == "constructor_declaration"
}

// Docstring extracts XML doc comments (///) preceding a method.
func (cs *CSharp) Docstring(node *sitter.Node, source []byte) string {
	return cs.ExtractPrecedingComment(node, source)
}

// ModulePath builds the module path from namespace declaration.
func (cs *CSharp) ModulePath(file slicing.ParsedFile) string {
	tree := file.Tree()
	if tree == nil {
		return ""
	}

	namespaceNodes := cs.Walker().CollectNodes(tree.RootNode(), []string{"namespace_declaration", "file_scoped_namespace_declaration"})
	if len(namespaceNodes) == 0 {
		return cs.BuildModulePathFromPath(file.Path(), ".cs")
	}

	namespaceNode := namespaceNodes[0]
	nameNode := namespaceNode.ChildByFieldName("name")
	if nameNode != nil {
		return cs.NodeText(nameNode, file.SourceCode())
	}

	return cs.BuildModulePathFromPath(file.Path(), ".cs")
}

// Classes extracts class, struct, interface, and enum definitions.
func (cs *CSharp) Classes(tree *sitter.Tree, source []byte) []slicing.ClassDefinition {
	if tree == nil {
		return nil
	}

	classNodes := cs.Walker().CollectNodes(tree.RootNode(), []string{"class_declaration", "struct_declaration", "interface_declaration", "enum_declaration"})
	classes := make([]slicing.ClassDefinition, 0, len(classNodes))

	for _, node := range classNodes {
		class := cs.extractClass(node, source)
		classes = append(classes, class)
	}

	return classes
}

// Types returns nil for C# (types are classes).
func (cs *CSharp) Types(_ *sitter.Tree, _ []byte) []slicing.TypeDefinition {
	return nil
}

func (cs *CSharp) extractClass(node *sitter.Node, source []byte) slicing.ClassDefinition {
	nameNode := node.ChildByFieldName("name")
	name := ""
	if nameNode != nil {
		name = cs.NodeText(nameNode, source)
	}

	docstring := cs.Docstring(node, source)
	bases := cs.extractBases(node, source)
	methods := cs.extractMethods(node, source, name)

	return slicing.NewClassDefinition(
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

func (cs *CSharp) extractBases(node *sitter.Node, source []byte) []string {
	baseList := node.ChildByFieldName("bases")
	if baseList == nil {
		return nil
	}

	var bases []string
	cs.Walker().Walk(baseList, func(n *sitter.Node) bool {
		if n.Type() == "identifier" || n.Type() == "generic_name" {
			bases = append(bases, cs.NodeText(n, source))
		}
		return true
	})

	return bases
}

func (cs *CSharp) extractMethods(classNode *sitter.Node, source []byte, className string) []slicing.FunctionDefinition {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return nil
	}

	methodNodes := cs.Walker().CollectNodes(body, []string{"method_declaration", "constructor_declaration"})
	methods := make([]slicing.FunctionDefinition, 0, len(methodNodes))

	for _, methodNode := range methodNodes {
		name := cs.FunctionName(methodNode, source)
		if name == "" {
			if methodNode.Type() == "constructor_declaration" {
				name = className
			} else {
				continue
			}
		}

		qualifiedName := className + "." + name
		docstring := cs.Docstring(methodNode, source)
		params := cs.extractParameters(methodNode, source)
		returnType := cs.extractReturnType(methodNode, source)

		method := slicing.NewFunctionDefinition(
			"",
			methodNode,
			methodNode.StartByte(),
			methodNode.EndByte(),
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

func (cs *CSharp) extractParameters(node *sitter.Node, source []byte) []string {
	params := node.ChildByFieldName("parameters")
	if params == nil {
		return nil
	}

	var result []string
	paramNodes := cs.Walker().CollectNodes(params, []string{"parameter"})

	for _, paramNode := range paramNodes {
		typeNode := paramNode.ChildByFieldName("type")
		nameNode := paramNode.ChildByFieldName("name")

		if typeNode != nil && nameNode != nil {
			param := cs.NodeText(typeNode, source) + " " + cs.NodeText(nameNode, source)
			result = append(result, param)
		}
	}

	return result
}

func (cs *CSharp) extractReturnType(node *sitter.Node, source []byte) string {
	returnType := node.ChildByFieldName("type")
	if returnType == nil {
		return ""
	}

	return cs.NodeText(returnType, source)
}
