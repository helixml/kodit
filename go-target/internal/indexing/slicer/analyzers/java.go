package analyzers

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/helixml/kodit/internal/indexing/slicer"
)

// Java implements Analyzer for Java code.
type Java struct {
	Base
}

// NewJava creates a new Java analyzer.
func NewJava(language slicer.Language) *Java {
	return &Java{
		Base: NewBase(language),
	}
}

// FunctionName extracts the method name from a method_declaration node.
func (j *Java) FunctionName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return j.NodeText(nameNode, source)
	}

	return ""
}

// IsPublic always returns true for Java (we treat all methods as public for indexing).
func (j *Java) IsPublic(_ *sitter.Node, _ string, _ []byte) bool {
	return true
}

// IsMethod returns false for Java (methods are handled within class extraction).
func (j *Java) IsMethod(_ *sitter.Node) bool {
	return false
}

// Docstring extracts Javadoc comments preceding a method.
func (j *Java) Docstring(node *sitter.Node, source []byte) string {
	return j.ExtractPrecedingComment(node, source)
}

// ModulePath builds the module path from package declaration.
func (j *Java) ModulePath(file slicer.ParsedFile) string {
	tree := file.Tree()
	if tree == nil {
		return ""
	}

	packageNodes := j.Walker().CollectNodes(tree.RootNode(), []string{"package_declaration"})
	if len(packageNodes) == 0 {
		return j.BuildModulePathFromPath(file.Path(), ".java")
	}

	packageNode := packageNodes[0]
	scopedId := j.Walker().FindDescendant(packageNode, "scoped_identifier")
	if scopedId != nil {
		return j.NodeText(scopedId, file.SourceCode())
	}

	idNode := j.Walker().FindDescendant(packageNode, "identifier")
	if idNode != nil {
		return j.NodeText(idNode, file.SourceCode())
	}

	return j.BuildModulePathFromPath(file.Path(), ".java")
}

// Classes extracts class definitions from the AST.
func (j *Java) Classes(tree *sitter.Tree, source []byte) []slicer.ClassDefinition {
	if tree == nil {
		return nil
	}

	classNodes := j.Walker().CollectNodes(tree.RootNode(), []string{"class_declaration", "interface_declaration", "enum_declaration"})
	classes := make([]slicer.ClassDefinition, 0, len(classNodes))

	for _, node := range classNodes {
		class := j.extractClass(node, source)
		classes = append(classes, class)
	}

	return classes
}

// Types returns nil for Java (types are classes).
func (j *Java) Types(_ *sitter.Tree, _ []byte) []slicer.TypeDefinition {
	return nil
}

func (j *Java) extractClass(node *sitter.Node, source []byte) slicer.ClassDefinition {
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

func (j *Java) extractBases(node *sitter.Node, source []byte) []string {
	var bases []string

	superclass := node.ChildByFieldName("superclass")
	if superclass != nil {
		j.Walker().Walk(superclass, func(n *sitter.Node) bool {
			if n.Type() == "type_identifier" {
				bases = append(bases, j.NodeText(n, source))
			}
			return true
		})
	}

	interfaces := node.ChildByFieldName("interfaces")
	if interfaces != nil {
		j.Walker().Walk(interfaces, func(n *sitter.Node) bool {
			if n.Type() == "type_identifier" {
				bases = append(bases, j.NodeText(n, source))
			}
			return true
		})
	}

	return bases
}

func (j *Java) extractMethods(classNode *sitter.Node, source []byte, className string) []slicer.FunctionDefinition {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return nil
	}

	methodNodes := j.Walker().CollectNodes(body, []string{"method_declaration", "constructor_declaration"})
	methods := make([]slicer.FunctionDefinition, 0, len(methodNodes))

	for _, methodNode := range methodNodes {
		name := j.FunctionName(methodNode, source)
		if name == "" {
			if methodNode.Type() == "constructor_declaration" {
				name = className
			} else {
				continue
			}
		}

		qualifiedName := className + "." + name
		docstring := j.Docstring(methodNode, source)
		params := j.extractParameters(methodNode, source)
		returnType := j.extractReturnType(methodNode, source)

		method := slicer.NewFunctionDefinition(
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

func (j *Java) extractParameters(node *sitter.Node, source []byte) []string {
	params := node.ChildByFieldName("parameters")
	if params == nil {
		return nil
	}

	var result []string
	paramNodes := j.Walker().CollectNodes(params, []string{"formal_parameter", "spread_parameter"})

	for _, paramNode := range paramNodes {
		typeNode := paramNode.ChildByFieldName("type")
		nameNode := paramNode.ChildByFieldName("name")

		if typeNode != nil && nameNode != nil {
			param := j.NodeText(typeNode, source) + " " + j.NodeText(nameNode, source)
			result = append(result, param)
		}
	}

	return result
}

func (j *Java) extractReturnType(node *sitter.Node, source []byte) string {
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil {
		return ""
	}

	return j.NodeText(typeNode, source)
}
