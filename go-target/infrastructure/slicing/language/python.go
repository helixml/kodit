package language

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/helixml/kodit/infrastructure/slicing"
)

// Python implements Analyzer for Python code.
type Python struct {
	Base
}

// NewPython creates a new Python analyzer.
func NewPython(language slicing.Language) *Python {
	return &Python{
		Base: NewBase(language),
	}
}

// FunctionName extracts the function name from a function_definition node.
func (p *Python) FunctionName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return p.NodeText(nameNode, source)
	}

	return ""
}

// IsPublic returns true if the function name does not start with underscore.
func (p *Python) IsPublic(_ *sitter.Node, name string, _ []byte) bool {
	return !strings.HasPrefix(name, "_")
}

// IsMethod returns false for Python (methods are extracted within class walk).
func (p *Python) IsMethod(_ *sitter.Node) bool {
	return false
}

// Docstring extracts the docstring from a function or class.
func (p *Python) Docstring(node *sitter.Node, source []byte) string {
	return p.ExtractFirstChildComment(node, source)
}

// ModulePath builds the module path from file information.
func (p *Python) ModulePath(file slicing.ParsedFile) string {
	return p.BuildModulePathFromPath(file.Path(), ".py")
}

// Classes extracts class definitions from the AST.
func (p *Python) Classes(tree *sitter.Tree, source []byte) []slicing.ClassDefinition {
	if tree == nil {
		return nil
	}

	classNodes := p.Walker().CollectNodes(tree.RootNode(), []string{"class_definition"})
	classes := make([]slicing.ClassDefinition, 0, len(classNodes))

	for _, node := range classNodes {
		class := p.extractClass(node, source, "")
		classes = append(classes, class)
	}

	return classes
}

// Types returns nil for Python (no type definitions).
func (p *Python) Types(_ *sitter.Tree, _ []byte) []slicing.TypeDefinition {
	return nil
}

func (p *Python) extractClass(node *sitter.Node, source []byte, modulePath string) slicing.ClassDefinition {
	nameNode := node.ChildByFieldName("name")
	name := ""
	if nameNode != nil {
		name = p.NodeText(nameNode, source)
	}

	qualifiedName := p.BuildQualifiedName(modulePath, name)
	docstring := p.Docstring(node, source)
	bases := p.extractBases(node, source)
	methods := p.extractMethods(node, source, qualifiedName)
	constructorParams := p.extractConstructorParams(node, source)

	return slicing.NewClassDefinition(
		"",
		node,
		node.StartByte(),
		node.EndByte(),
		qualifiedName,
		name,
		!strings.HasPrefix(name, "_"),
		docstring,
		bases,
		methods,
		constructorParams,
	)
}

func (p *Python) extractBases(node *sitter.Node, source []byte) []string {
	superclass := node.ChildByFieldName("superclasses")
	if superclass == nil {
		return nil
	}

	var bases []string
	for i := uint32(0); i < superclass.ChildCount(); i++ {
		child := superclass.Child(int(i))
		if child != nil && p.Walker().IsIdentifier(child) {
			bases = append(bases, p.NodeText(child, source))
		}
	}

	return bases
}

func (p *Python) extractMethods(classNode *sitter.Node, source []byte, classQualifiedName string) []slicing.FunctionDefinition {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return nil
	}

	funcNodes := p.Walker().CollectNodes(body, []string{"function_definition"})
	methods := make([]slicing.FunctionDefinition, 0, len(funcNodes))

	for _, funcNode := range funcNodes {
		if funcNode.Parent() != body {
			continue
		}

		name := p.FunctionName(funcNode, source)
		if name == "" {
			continue
		}

		qualifiedName := classQualifiedName + "." + name
		docstring := p.Docstring(funcNode, source)
		params := p.extractParameters(funcNode, source)
		returnType := p.extractReturnType(funcNode, source)

		method := slicing.NewFunctionDefinition(
			"",
			funcNode,
			funcNode.StartByte(),
			funcNode.EndByte(),
			qualifiedName,
			name,
			!strings.HasPrefix(name, "_"),
			true,
			docstring,
			params,
			returnType,
		)
		methods = append(methods, method)
	}

	return methods
}

func (p *Python) extractConstructorParams(classNode *sitter.Node, source []byte) []string {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return nil
	}

	funcNodes := p.Walker().CollectNodes(body, []string{"function_definition"})
	for _, funcNode := range funcNodes {
		if funcNode.Parent() != body {
			continue
		}

		name := p.FunctionName(funcNode, source)
		if name == "__init__" {
			return p.extractParameters(funcNode, source)
		}
	}

	return nil
}

func (p *Python) extractParameters(funcNode *sitter.Node, source []byte) []string {
	params := funcNode.ChildByFieldName("parameters")
	if params == nil {
		return nil
	}

	var result []string
	for i := uint32(0); i < params.ChildCount(); i++ {
		child := params.Child(int(i))
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			result = append(result, p.NodeText(child, source))
		case "typed_parameter":
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				name := p.NodeText(nameNode, source)
				typeNode := child.ChildByFieldName("type")
				if typeNode != nil {
					name += ": " + p.NodeText(typeNode, source)
				}
				result = append(result, name)
			}
		case "default_parameter":
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				name := p.NodeText(nameNode, source)
				valueNode := child.ChildByFieldName("value")
				if valueNode != nil {
					name += "=" + p.NodeText(valueNode, source)
				}
				result = append(result, name)
			}
		case "typed_default_parameter":
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				name := p.NodeText(nameNode, source)
				typeNode := child.ChildByFieldName("type")
				if typeNode != nil {
					name += ": " + p.NodeText(typeNode, source)
				}
				valueNode := child.ChildByFieldName("value")
				if valueNode != nil {
					name += " = " + p.NodeText(valueNode, source)
				}
				result = append(result, name)
			}
		case "list_splat_pattern":
			result = append(result, "*"+p.NodeText(child, source))
		case "dictionary_splat_pattern":
			result = append(result, "**"+p.NodeText(child, source))
		}
	}

	return result
}

func (p *Python) extractReturnType(funcNode *sitter.Node, source []byte) string {
	returnType := funcNode.ChildByFieldName("return_type")
	if returnType == nil {
		return ""
	}

	return p.NodeText(returnType, source)
}
