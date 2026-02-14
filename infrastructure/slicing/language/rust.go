package language

import (
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/helixml/kodit/infrastructure/slicing"
)

// Rust implements Analyzer for Rust code.
type Rust struct {
	Base
}

// NewRust creates a new Rust analyzer.
func NewRust(language slicing.Language) *Rust {
	return &Rust{
		Base: NewBase(language),
	}
}

// FunctionName extracts the function name from a function_item node.
func (r *Rust) FunctionName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return r.NodeText(nameNode, source)
	}

	return ""
}

// IsPublic always returns true for Rust (we index all functions).
func (r *Rust) IsPublic(_ *sitter.Node, _ string, _ []byte) bool {
	return true
}

// IsMethod returns true if the node is inside an impl block.
func (r *Rust) IsMethod(node *sitter.Node) bool {
	if node == nil {
		return false
	}

	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "impl_item" {
			return true
		}
		parent = parent.Parent()
	}

	return false
}

// Docstring extracts doc comments (/// or //!) preceding a function.
func (r *Rust) Docstring(node *sitter.Node, source []byte) string {
	return r.ExtractPrecedingComment(node, source)
}

// ModulePath builds the module path from file information.
func (r *Rust) ModulePath(file slicing.ParsedFile) string {
	return r.BuildModulePathFromPath(file.Path(), ".rs")
}

// Classes extracts struct and enum definitions.
func (r *Rust) Classes(tree *sitter.Tree, source []byte) []slicing.ClassDefinition {
	if tree == nil {
		return nil
	}

	structNodes := r.Walker().CollectNodes(tree.RootNode(), []string{"struct_item", "enum_item"})
	classes := make([]slicing.ClassDefinition, 0, len(structNodes))

	for _, node := range structNodes {
		class := r.extractStruct(node, source)
		classes = append(classes, class)
	}

	return classes
}

// Types extracts type aliases and trait definitions.
func (r *Rust) Types(tree *sitter.Tree, source []byte) []slicing.TypeDefinition {
	if tree == nil {
		return nil
	}

	typeNodes := r.Walker().CollectNodes(tree.RootNode(), []string{"type_item", "trait_item"})
	types := make([]slicing.TypeDefinition, 0, len(typeNodes))

	for _, node := range typeNodes {
		typeDef := r.extractType(node, source)
		types = append(types, typeDef)
	}

	return types
}

func (r *Rust) extractStruct(node *sitter.Node, source []byte) slicing.ClassDefinition {
	nameNode := node.ChildByFieldName("name")
	name := ""
	if nameNode != nil {
		name = r.NodeText(nameNode, source)
	}

	docstring := r.Docstring(node, source)
	fields := r.extractStructFields(node, source)

	methods := r.extractImplMethods(node, source, name)

	return slicing.NewClassDefinition(
		"",
		node,
		node.StartByte(),
		node.EndByte(),
		name,
		name,
		true,
		docstring,
		nil,
		methods,
		fields,
	)
}

func (r *Rust) extractStructFields(node *sitter.Node, source []byte) []string {
	body := node.ChildByFieldName("body")
	if body == nil {
		return nil
	}

	fieldNodes := r.Walker().CollectDescendants(body, "field_declaration")
	var fields []string

	for _, fieldNode := range fieldNodes {
		nameNode := fieldNode.ChildByFieldName("name")
		typeNode := fieldNode.ChildByFieldName("type")

		if nameNode != nil && typeNode != nil {
			field := r.NodeText(nameNode, source) + ": " + r.NodeText(typeNode, source)
			fields = append(fields, field)
		}
	}

	return fields
}

func (r *Rust) extractImplMethods(structNode *sitter.Node, source []byte, structName string) []slicing.FunctionDefinition {
	tree := structNode
	for tree.Parent() != nil {
		tree = tree.Parent()
	}

	implNodes := r.Walker().CollectNodes(tree, []string{"impl_item"})
	var methods []slicing.FunctionDefinition

	for _, implNode := range implNodes {
		typeNode := implNode.ChildByFieldName("type")
		if typeNode == nil {
			continue
		}

		var typeName string
		r.Walker().Walk(typeNode, func(n *sitter.Node) bool {
			if n.Type() == "type_identifier" {
				typeName = r.NodeText(n, source)
				return false
			}
			return true
		})

		if typeName != structName {
			continue
		}

		body := implNode.ChildByFieldName("body")
		if body == nil {
			continue
		}

		funcNodes := r.Walker().CollectNodes(body, []string{"function_item"})
		for _, funcNode := range funcNodes {
			name := r.FunctionName(funcNode, source)
			if name == "" {
				continue
			}

			qualifiedName := structName + "::" + name
			docstring := r.Docstring(funcNode, source)
			params := r.extractParameters(funcNode, source)
			returnType := r.extractReturnType(funcNode, source)

			method := slicing.NewFunctionDefinition(
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
	}

	return methods
}

func (r *Rust) extractType(node *sitter.Node, source []byte) slicing.TypeDefinition {
	nameNode := node.ChildByFieldName("name")
	name := ""
	if nameNode != nil {
		name = r.NodeText(nameNode, source)
	}

	kind := "alias"
	if node.Type() == "trait_item" {
		kind = "trait"
	}

	docstring := r.Docstring(node, source)

	return slicing.NewTypeDefinition(
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

func (r *Rust) extractParameters(node *sitter.Node, source []byte) []string {
	params := node.ChildByFieldName("parameters")
	if params == nil {
		return nil
	}

	var result []string
	paramNodes := r.Walker().CollectNodes(params, []string{"parameter", "self_parameter"})

	for _, paramNode := range paramNodes {
		result = append(result, r.NodeText(paramNode, source))
	}

	return result
}

func (r *Rust) extractReturnType(node *sitter.Node, source []byte) string {
	returnType := node.ChildByFieldName("return_type")
	if returnType == nil {
		return ""
	}

	return r.NodeText(returnType, source)
}
