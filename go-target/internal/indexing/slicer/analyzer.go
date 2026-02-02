package slicer

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// Analyzer extracts code elements from parsed AST trees.
type Analyzer interface {
	// Language returns the language configuration.
	Language() Language

	// FunctionName extracts the function name from a function node.
	FunctionName(node *sitter.Node, source []byte) string

	// IsPublic determines if a function is public based on naming conventions.
	IsPublic(node *sitter.Node, name string, source []byte) bool

	// IsMethod determines if a node is a method (receiver-based function).
	IsMethod(node *sitter.Node) bool

	// Docstring extracts documentation comments from a node.
	Docstring(node *sitter.Node, source []byte) string

	// ModulePath builds the module path from file information.
	ModulePath(file ParsedFile) string

	// Classes extracts class definitions from the AST.
	Classes(tree *sitter.Tree, source []byte) []ClassDefinition

	// Types extracts type definitions from the AST.
	Types(tree *sitter.Tree, source []byte) []TypeDefinition
}

// ParsedFile represents a parsed source file.
type ParsedFile struct {
	path       string
	tree       *sitter.Tree
	sourceCode []byte
}

// NewParsedFile creates a new ParsedFile.
func NewParsedFile(path string, tree *sitter.Tree, sourceCode []byte) ParsedFile {
	code := make([]byte, len(sourceCode))
	copy(code, sourceCode)

	return ParsedFile{
		path:       path,
		tree:       tree,
		sourceCode: code,
	}
}

// Path returns the file path.
func (p ParsedFile) Path() string { return p.path }

// Tree returns the AST tree.
func (p ParsedFile) Tree() *sitter.Tree { return p.tree }

// SourceCode returns the source code bytes.
func (p ParsedFile) SourceCode() []byte {
	code := make([]byte, len(p.sourceCode))
	copy(code, p.sourceCode)
	return code
}

// FunctionDefinition represents an extracted function.
type FunctionDefinition struct {
	filePath      string
	node          *sitter.Node
	startByte     uint32
	endByte       uint32
	qualifiedName string
	simpleName    string
	isPublic      bool
	isMethod      bool
	docstring     string
	parameters    []string
	returnType    string
}

// NewFunctionDefinition creates a new FunctionDefinition.
func NewFunctionDefinition(
	filePath string,
	node *sitter.Node,
	startByte, endByte uint32,
	qualifiedName, simpleName string,
	isPublic, isMethod bool,
	docstring string,
	parameters []string,
	returnType string,
) FunctionDefinition {
	params := make([]string, len(parameters))
	copy(params, parameters)

	return FunctionDefinition{
		filePath:      filePath,
		node:          node,
		startByte:     startByte,
		endByte:       endByte,
		qualifiedName: qualifiedName,
		simpleName:    simpleName,
		isPublic:      isPublic,
		isMethod:      isMethod,
		docstring:     docstring,
		parameters:    params,
		returnType:    returnType,
	}
}

// FilePath returns the source file path.
func (f FunctionDefinition) FilePath() string { return f.filePath }

// Node returns the AST node.
func (f FunctionDefinition) Node() *sitter.Node { return f.node }

// StartByte returns the start byte position.
func (f FunctionDefinition) StartByte() uint32 { return f.startByte }

// EndByte returns the end byte position.
func (f FunctionDefinition) EndByte() uint32 { return f.endByte }

// Span returns the byte span (start, end).
func (f FunctionDefinition) Span() (uint32, uint32) { return f.startByte, f.endByte }

// QualifiedName returns the fully qualified name.
func (f FunctionDefinition) QualifiedName() string { return f.qualifiedName }

// SimpleName returns the simple function name.
func (f FunctionDefinition) SimpleName() string { return f.simpleName }

// IsPublic returns true if the function is public.
func (f FunctionDefinition) IsPublic() bool { return f.isPublic }

// IsMethod returns true if the function is a method.
func (f FunctionDefinition) IsMethod() bool { return f.isMethod }

// Docstring returns the function documentation.
func (f FunctionDefinition) Docstring() string { return f.docstring }

// Parameters returns the function parameters.
func (f FunctionDefinition) Parameters() []string {
	params := make([]string, len(f.parameters))
	copy(params, f.parameters)
	return params
}

// ReturnType returns the function return type.
func (f FunctionDefinition) ReturnType() string { return f.returnType }

// ClassDefinition represents an extracted class or struct.
type ClassDefinition struct {
	filePath      string
	node          *sitter.Node
	startByte     uint32
	endByte       uint32
	qualifiedName string
	simpleName    string
	isPublic      bool
	docstring     string
	bases         []string
	methods       []FunctionDefinition
	constructorParams []string
}

// NewClassDefinition creates a new ClassDefinition.
func NewClassDefinition(
	filePath string,
	node *sitter.Node,
	startByte, endByte uint32,
	qualifiedName, simpleName string,
	isPublic bool,
	docstring string,
	bases []string,
	methods []FunctionDefinition,
	constructorParams []string,
) ClassDefinition {
	basesCopy := make([]string, len(bases))
	copy(basesCopy, bases)

	methodsCopy := make([]FunctionDefinition, len(methods))
	copy(methodsCopy, methods)

	paramsCopy := make([]string, len(constructorParams))
	copy(paramsCopy, constructorParams)

	return ClassDefinition{
		filePath:      filePath,
		node:          node,
		startByte:     startByte,
		endByte:       endByte,
		qualifiedName: qualifiedName,
		simpleName:    simpleName,
		isPublic:      isPublic,
		docstring:     docstring,
		bases:         basesCopy,
		methods:       methodsCopy,
		constructorParams: paramsCopy,
	}
}

// FilePath returns the source file path.
func (c ClassDefinition) FilePath() string { return c.filePath }

// Node returns the AST node.
func (c ClassDefinition) Node() *sitter.Node { return c.node }

// StartByte returns the start byte position.
func (c ClassDefinition) StartByte() uint32 { return c.startByte }

// EndByte returns the end byte position.
func (c ClassDefinition) EndByte() uint32 { return c.endByte }

// QualifiedName returns the fully qualified name.
func (c ClassDefinition) QualifiedName() string { return c.qualifiedName }

// SimpleName returns the simple class name.
func (c ClassDefinition) SimpleName() string { return c.simpleName }

// IsPublic returns true if the class is public.
func (c ClassDefinition) IsPublic() bool { return c.isPublic }

// Docstring returns the class documentation.
func (c ClassDefinition) Docstring() string { return c.docstring }

// Bases returns the base class names.
func (c ClassDefinition) Bases() []string {
	bases := make([]string, len(c.bases))
	copy(bases, c.bases)
	return bases
}

// Methods returns the class methods.
func (c ClassDefinition) Methods() []FunctionDefinition {
	methods := make([]FunctionDefinition, len(c.methods))
	copy(methods, c.methods)
	return methods
}

// ConstructorParams returns the constructor parameters.
func (c ClassDefinition) ConstructorParams() []string {
	params := make([]string, len(c.constructorParams))
	copy(params, c.constructorParams)
	return params
}

// TypeDefinition represents an extracted type alias or interface.
type TypeDefinition struct {
	filePath      string
	node          *sitter.Node
	startByte     uint32
	endByte       uint32
	qualifiedName string
	simpleName    string
	kind          string
	docstring     string
	constructorParams []string
}

// NewTypeDefinition creates a new TypeDefinition.
func NewTypeDefinition(
	filePath string,
	node *sitter.Node,
	startByte, endByte uint32,
	qualifiedName, simpleName, kind, docstring string,
	constructorParams []string,
) TypeDefinition {
	paramsCopy := make([]string, len(constructorParams))
	copy(paramsCopy, constructorParams)

	return TypeDefinition{
		filePath:      filePath,
		node:          node,
		startByte:     startByte,
		endByte:       endByte,
		qualifiedName: qualifiedName,
		simpleName:    simpleName,
		kind:          kind,
		docstring:     docstring,
		constructorParams: paramsCopy,
	}
}

// FilePath returns the source file path.
func (t TypeDefinition) FilePath() string { return t.filePath }

// Node returns the AST node.
func (t TypeDefinition) Node() *sitter.Node { return t.node }

// StartByte returns the start byte position.
func (t TypeDefinition) StartByte() uint32 { return t.startByte }

// EndByte returns the end byte position.
func (t TypeDefinition) EndByte() uint32 { return t.endByte }

// QualifiedName returns the fully qualified name.
func (t TypeDefinition) QualifiedName() string { return t.qualifiedName }

// SimpleName returns the simple type name.
func (t TypeDefinition) SimpleName() string { return t.simpleName }

// Kind returns the type kind (e.g., "struct", "interface", "alias").
func (t TypeDefinition) Kind() string { return t.kind }

// Docstring returns the type documentation.
func (t TypeDefinition) Docstring() string { return t.docstring }

// ConstructorParams returns the constructor parameters (struct fields).
func (t TypeDefinition) ConstructorParams() []string {
	params := make([]string, len(t.constructorParams))
	copy(params, t.constructorParams)
	return params
}
