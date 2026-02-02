package analyzers

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/helixml/kodit/internal/indexing/slicer"
)

// JavaScript implements Analyzer for JavaScript code.
type JavaScript struct {
	Base
}

// NewJavaScript creates a new JavaScript analyzer.
func NewJavaScript(language slicer.Language) *JavaScript {
	return &JavaScript{
		Base: NewBase(language),
	}
}

// FunctionName extracts the function name from various function nodes.
func (j *JavaScript) FunctionName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return j.NodeText(nameNode, source)
	}

	if node.Type() == "arrow_function" || node.Type() == "function_expression" {
		parent := node.Parent()
		if parent != nil && parent.Type() == "variable_declarator" {
			nameNode = parent.ChildByFieldName("name")
			if nameNode != nil {
				return j.NodeText(nameNode, source)
			}
		}

		if parent != nil && parent.Type() == "assignment_expression" {
			left := parent.ChildByFieldName("left")
			if left != nil && j.Walker().IsIdentifier(left) {
				return j.NodeText(left, source)
			}
		}
	}

	return ""
}

// IsPublic always returns true for JavaScript (no private convention in standard JS).
func (j *JavaScript) IsPublic(_ *sitter.Node, _ string, _ []byte) bool {
	return true
}

// IsMethod returns true if the node is a method_definition.
func (j *JavaScript) IsMethod(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	return node.Type() == "method_definition"
}

// Docstring extracts JSDoc comments preceding a function.
func (j *JavaScript) Docstring(node *sitter.Node, source []byte) string {
	return j.ExtractPrecedingComment(node, source)
}

// ModulePath builds the module path from file information.
func (j *JavaScript) ModulePath(file slicer.ParsedFile) string {
	return j.BuildModulePathFromPath(file.Path(), ".js")
}

// Classes extracts class definitions from the AST.
func (j *JavaScript) Classes(tree *sitter.Tree, source []byte) []slicer.ClassDefinition {
	if tree == nil {
		return nil
	}

	classNodes := j.Walker().CollectNodes(tree.RootNode(), []string{"class_declaration"})
	classes := make([]slicer.ClassDefinition, 0, len(classNodes))

	for _, node := range classNodes {
		class := j.extractClass(node, source)
		classes = append(classes, class)
	}

	return classes
}

// Types returns nil for JavaScript (no type definitions in vanilla JS).
func (j *JavaScript) Types(_ *sitter.Tree, _ []byte) []slicer.TypeDefinition {
	return nil
}

func (j *JavaScript) extractClass(node *sitter.Node, source []byte) slicer.ClassDefinition {
	nameNode := node.ChildByFieldName("name")
	name := ""
	if nameNode != nil {
		name = j.NodeText(nameNode, source)
	}

	docstring := j.Docstring(node, source)
	bases := j.extractBases(node, source)
	methods := j.extractMethods(node, source, name)

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

func (j *JavaScript) extractBases(node *sitter.Node, source []byte) []string {
	heritage := node.ChildByFieldName("heritage")
	if heritage == nil {
		return nil
	}

	var bases []string
	j.Walker().Walk(heritage, func(n *sitter.Node) bool {
		if j.Walker().IsIdentifier(n) {
			bases = append(bases, j.NodeText(n, source))
		}
		return true
	})

	return bases
}

func (j *JavaScript) extractMethods(classNode *sitter.Node, source []byte, className string) []slicer.FunctionDefinition {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return nil
	}

	methodNodes := j.Walker().CollectNodes(body, []string{"method_definition"})
	methods := make([]slicer.FunctionDefinition, 0, len(methodNodes))

	for _, methodNode := range methodNodes {
		nameNode := methodNode.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}

		name := j.NodeText(nameNode, source)
		qualifiedName := className + "." + name
		docstring := j.Docstring(methodNode, source)
		params := j.extractParameters(methodNode, source)

		method := slicer.NewFunctionDefinition(
			"",
			methodNode,
			methodNode.StartByte(),
			methodNode.EndByte(),
			qualifiedName,
			name,
			!strings.HasPrefix(name, "_"),
			true,
			docstring,
			params,
			"",
		)
		methods = append(methods, method)
	}

	return methods
}

func (j *JavaScript) extractParameters(node *sitter.Node, source []byte) []string {
	params := node.ChildByFieldName("parameters")
	if params == nil {
		return nil
	}

	var result []string
	for i := uint32(0); i < params.ChildCount(); i++ {
		child := params.Child(int(i))
		if child != nil && j.Walker().IsIdentifier(child) {
			result = append(result, j.NodeText(child, source))
		}
	}

	return result
}
