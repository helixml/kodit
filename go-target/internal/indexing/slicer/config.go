// Package slicer provides AST-based code snippet extraction using tree-sitter.
package slicer

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Language represents a supported programming language.
type Language struct {
	name      string
	extension string
	language  *sitter.Language
	nodes     NodeTypes
}

// NewLanguage creates a new Language configuration.
func NewLanguage(name, extension string, lang *sitter.Language, nodes NodeTypes) Language {
	return Language{
		name:      name,
		extension: extension,
		language:  lang,
		nodes:     nodes,
	}
}

// Name returns the language name.
func (l Language) Name() string { return l.name }

// Extension returns the file extension (e.g., ".py").
func (l Language) Extension() string { return l.extension }

// SitterLanguage returns the tree-sitter language.
func (l Language) SitterLanguage() *sitter.Language { return l.language }

// Nodes returns the node type configuration.
func (l Language) Nodes() NodeTypes { return l.nodes }

// NodeTypes defines AST node type names for a language.
type NodeTypes struct {
	functionNodes []string
	methodNodes   []string
	classNodes    []string
	typeNodes     []string
	callNode      string
	importNodes   []string
	nameField     string
}

// NewNodeTypes creates a new NodeTypes configuration.
func NewNodeTypes(
	functionNodes, methodNodes, classNodes, typeNodes []string,
	callNode string,
	importNodes []string,
	nameField string,
) NodeTypes {
	return NodeTypes{
		functionNodes: functionNodes,
		methodNodes:   methodNodes,
		classNodes:    classNodes,
		typeNodes:     typeNodes,
		callNode:      callNode,
		importNodes:   importNodes,
		nameField:     nameField,
	}
}

// FunctionNodes returns function definition node types.
func (n NodeTypes) FunctionNodes() []string { return n.functionNodes }

// MethodNodes returns method definition node types.
func (n NodeTypes) MethodNodes() []string { return n.methodNodes }

// ClassNodes returns class/struct definition node types.
func (n NodeTypes) ClassNodes() []string { return n.classNodes }

// TypeNodes returns type definition node types.
func (n NodeTypes) TypeNodes() []string { return n.typeNodes }

// CallNode returns the function call node type.
func (n NodeTypes) CallNode() string { return n.callNode }

// ImportNodes returns import statement node types.
func (n NodeTypes) ImportNodes() []string { return n.importNodes }

// NameField returns the tree-sitter field name for extracting identifiers.
func (n NodeTypes) NameField() string { return n.nameField }

// IsFunctionNode returns true if the node type is a function definition.
func (n NodeTypes) IsFunctionNode(nodeType string) bool {
	for _, t := range n.functionNodes {
		if t == nodeType {
			return true
		}
	}
	return false
}

// IsMethodNode returns true if the node type is a method definition.
func (n NodeTypes) IsMethodNode(nodeType string) bool {
	for _, t := range n.methodNodes {
		if t == nodeType {
			return true
		}
	}
	return false
}

// IsClassNode returns true if the node type is a class definition.
func (n NodeTypes) IsClassNode(nodeType string) bool {
	for _, t := range n.classNodes {
		if t == nodeType {
			return true
		}
	}
	return false
}

// IsTypeNode returns true if the node type is a type definition.
func (n NodeTypes) IsTypeNode(nodeType string) bool {
	for _, t := range n.typeNodes {
		if t == nodeType {
			return true
		}
	}
	return false
}

// LanguageConfig holds all supported language configurations.
type LanguageConfig struct {
	languages map[string]Language
	byExt     map[string]Language
}

// NewLanguageConfig creates a LanguageConfig with all supported languages.
func NewLanguageConfig() LanguageConfig {
	languages := make(map[string]Language)
	byExt := make(map[string]Language)

	configs := []Language{
		pythonConfig(),
		goConfig(),
		javaConfig(),
		cConfig(),
		cppConfig(),
		rustConfig(),
		javascriptConfig(),
		typescriptConfig(),
		tsxConfig(),
		csharpConfig(),
	}

	for _, cfg := range configs {
		languages[cfg.name] = cfg
		byExt[cfg.extension] = cfg
	}

	return LanguageConfig{
		languages: languages,
		byExt:     byExt,
	}
}

// ByName returns the language configuration by name.
func (c LanguageConfig) ByName(name string) (Language, bool) {
	lang, ok := c.languages[name]
	return lang, ok
}

// ByExtension returns the language configuration by file extension.
func (c LanguageConfig) ByExtension(ext string) (Language, bool) {
	lang, ok := c.byExt[ext]
	return lang, ok
}

// SupportedExtensions returns all supported file extensions.
func (c LanguageConfig) SupportedExtensions() []string {
	extensions := make([]string, 0, len(c.byExt))
	for ext := range c.byExt {
		extensions = append(extensions, ext)
	}
	return extensions
}

// SupportedLanguages returns all supported language names.
func (c LanguageConfig) SupportedLanguages() []string {
	names := make([]string, 0, len(c.languages))
	for name := range c.languages {
		names = append(names, name)
	}
	return names
}

func pythonConfig() Language {
	return NewLanguage(
		"python",
		".py",
		python.GetLanguage(),
		NewNodeTypes(
			[]string{"function_definition"},
			[]string{},
			[]string{"class_definition"},
			[]string{},
			"call",
			[]string{"import_statement", "import_from_statement"},
			"name",
		),
	)
}

func goConfig() Language {
	return NewLanguage(
		"go",
		".go",
		golang.GetLanguage(),
		NewNodeTypes(
			[]string{"function_declaration"},
			[]string{"method_declaration"},
			[]string{},
			[]string{"type_declaration", "type_spec"},
			"call_expression",
			[]string{"import_declaration", "import_spec"},
			"name",
		),
	)
}

func javaConfig() Language {
	return NewLanguage(
		"java",
		".java",
		java.GetLanguage(),
		NewNodeTypes(
			[]string{"method_declaration", "constructor_declaration"},
			[]string{},
			[]string{"class_declaration", "interface_declaration", "enum_declaration"},
			[]string{},
			"method_invocation",
			[]string{"import_declaration"},
			"name",
		),
	)
}

func cConfig() Language {
	return NewLanguage(
		"c",
		".c",
		c.GetLanguage(),
		NewNodeTypes(
			[]string{"function_definition"},
			[]string{},
			[]string{"struct_specifier", "union_specifier", "enum_specifier"},
			[]string{"type_definition"},
			"call_expression",
			[]string{"preproc_include"},
			"declarator",
		),
	)
}

func cppConfig() Language {
	return NewLanguage(
		"cpp",
		".cpp",
		cpp.GetLanguage(),
		NewNodeTypes(
			[]string{"function_definition"},
			[]string{},
			[]string{"class_specifier", "struct_specifier"},
			[]string{"type_definition", "alias_declaration"},
			"call_expression",
			[]string{"preproc_include", "using_declaration"},
			"declarator",
		),
	)
}

func rustConfig() Language {
	return NewLanguage(
		"rust",
		".rs",
		rust.GetLanguage(),
		NewNodeTypes(
			[]string{"function_item"},
			[]string{"impl_item"},
			[]string{"struct_item", "enum_item"},
			[]string{"type_item", "trait_item"},
			"call_expression",
			[]string{"use_declaration"},
			"name",
		),
	)
}

func javascriptConfig() Language {
	return NewLanguage(
		"javascript",
		".js",
		javascript.GetLanguage(),
		NewNodeTypes(
			[]string{"function_declaration", "arrow_function", "function_expression"},
			[]string{"method_definition"},
			[]string{"class_declaration"},
			[]string{},
			"call_expression",
			[]string{"import_statement"},
			"name",
		),
	)
}

func typescriptConfig() Language {
	return NewLanguage(
		"typescript",
		".ts",
		typescript.GetLanguage(),
		NewNodeTypes(
			[]string{"function_declaration", "arrow_function", "function_expression"},
			[]string{"method_definition"},
			[]string{"class_declaration"},
			[]string{"type_alias_declaration", "interface_declaration"},
			"call_expression",
			[]string{"import_statement"},
			"name",
		),
	)
}

func tsxConfig() Language {
	return NewLanguage(
		"tsx",
		".tsx",
		tsx.GetLanguage(),
		NewNodeTypes(
			[]string{"function_declaration", "arrow_function", "function_expression"},
			[]string{"method_definition"},
			[]string{"class_declaration"},
			[]string{"type_alias_declaration", "interface_declaration"},
			"call_expression",
			[]string{"import_statement"},
			"name",
		),
	)
}

func csharpConfig() Language {
	return NewLanguage(
		"csharp",
		".cs",
		csharp.GetLanguage(),
		NewNodeTypes(
			[]string{"method_declaration", "local_function_statement"},
			[]string{"constructor_declaration"},
			[]string{"class_declaration", "struct_declaration", "interface_declaration", "enum_declaration"},
			[]string{},
			"invocation_expression",
			[]string{"using_directive"},
			"name",
		),
	)
}
