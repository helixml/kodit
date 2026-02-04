package slicing_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/slicing"
	"github.com/helixml/kodit/infrastructure/slicing/language"
)

func newParser(lang slicing.Language) *sitter.Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(lang.SitterLanguage())
	return parser
}

func TestLanguageConfig_ByExtension(t *testing.T) {
	config := slicing.NewLanguageConfig()

	tests := []struct {
		ext      string
		expected string
		ok       bool
	}{
		{".py", "python", true},
		{".go", "go", true},
		{".java", "java", true},
		{".c", "c", true},
		{".cpp", "cpp", true},
		{".rs", "rust", true},
		{".js", "javascript", true},
		{".ts", "typescript", true},
		{".tsx", "tsx", true},
		{".cs", "csharp", true},
		{".unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			lang, ok := config.ByExtension(tt.ext)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.expected, lang.Name())
			}
		})
	}
}

func TestNodeTypes_IsFunctionNode(t *testing.T) {
	config := slicing.NewLanguageConfig()

	pythonLang, ok := config.ByName("python")
	require.True(t, ok)

	nodes := pythonLang.Nodes()
	assert.True(t, nodes.IsFunctionNode("function_definition"))
	assert.False(t, nodes.IsFunctionNode("class_definition"))

	goLang, ok := config.ByName("go")
	require.True(t, ok)

	goNodes := goLang.Nodes()
	assert.True(t, goNodes.IsFunctionNode("function_declaration"))
	assert.True(t, goNodes.IsMethodNode("method_declaration"))
}

func TestWalker_Walk(t *testing.T) {
	config := slicing.NewLanguageConfig()
	lang, _ := config.ByExtension(".py")

	source := []byte(`def foo():
    pass

def bar():
    foo()
`)

	parser := lang.SitterLanguage()
	require.NotNil(t, parser)

	walker := slicing.NewWalker()

	sitterParser := newParser(lang)
	tree, err := sitterParser.ParseCtx(context.Background(), nil, source)
	require.NoError(t, err)

	var nodeTypes []string
	walker.Walk(tree.RootNode(), func(node *sitter.Node) bool {
		nodeTypes = append(nodeTypes, node.Type())
		return true
	})

	assert.Contains(t, nodeTypes, "module")
	assert.Contains(t, nodeTypes, "function_definition")
}

func TestWalker_CollectNodes(t *testing.T) {
	config := slicing.NewLanguageConfig()
	lang, _ := config.ByExtension(".py")

	source := []byte(`def foo():
    pass

def bar():
    pass

class MyClass:
    def method(self):
        pass
`)

	sitterParser := newParser(lang)
	tree, err := sitterParser.ParseCtx(context.Background(), nil, source)
	require.NoError(t, err)

	walker := slicing.NewWalker()
	funcNodes := walker.CollectNodes(tree.RootNode(), []string{"function_definition"})

	assert.Len(t, funcNodes, 3)
}

func TestCallGraph(t *testing.T) {
	graph := slicing.NewCallGraph()

	graph.AddCall("main", "foo")
	graph.AddCall("main", "bar")
	graph.AddCall("foo", "helper")
	graph.AddCall("bar", "helper")

	callees := graph.Callees("main")
	assert.Len(t, callees, 2)

	callers := graph.Callers("helper")
	assert.Len(t, callers, 2)

	deps := graph.Dependencies("main", 2, 10)
	assert.Contains(t, deps, "foo")
	assert.Contains(t, deps, "bar")
	assert.Contains(t, deps, "helper")
}

func TestSlicer_SlicePythonFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.py")

	pythonCode := `def greet(name):
    """Greet someone."""
    return f"Hello, {name}!"

def main():
    message = greet("World")
    print(message)
`
	err := os.WriteFile(testFile, []byte(pythonCode), 0644)
	require.NoError(t, err)

	config := slicing.NewLanguageConfig()
	factory := language.NewFactory(config)
	s := slicing.NewSlicer(config, factory)

	files := []repository.File{
		repository.NewFile("abc123", "test.py", "python", int64(len(pythonCode))),
	}

	cfg := slicing.DefaultSliceConfig()
	result, err := s.Slice(context.Background(), files, tmpDir, cfg)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Functions())
	assert.NotEmpty(t, result.Snippets())

	var functionNames []string
	for _, f := range result.Functions() {
		functionNames = append(functionNames, f.SimpleName())
	}

	assert.Contains(t, functionNames, "greet")
	assert.Contains(t, functionNames, "main")
}

func TestSlicer_SliceGoFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	goCode := `package main

// Greet returns a greeting message.
func Greet(name string) string {
	return "Hello, " + name + "!"
}

func main() {
	message := Greet("World")
	println(message)
}
`
	err := os.WriteFile(testFile, []byte(goCode), 0644)
	require.NoError(t, err)

	config := slicing.NewLanguageConfig()
	factory := language.NewFactory(config)
	s := slicing.NewSlicer(config, factory)

	files := []repository.File{
		repository.NewFile("abc123", "test.go", "go", int64(len(goCode))),
	}

	cfg := slicing.DefaultSliceConfig()
	result, err := s.Slice(context.Background(), files, tmpDir, cfg)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Functions())

	var functionNames []string
	for _, f := range result.Functions() {
		functionNames = append(functionNames, f.SimpleName())
	}

	assert.Contains(t, functionNames, "Greet")
	assert.Contains(t, functionNames, "main")
}

func TestSlicer_SliceJavaScriptFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.js")

	jsCode := `function greet(name) {
    return "Hello, " + name + "!";
}

const sayHello = () => {
    console.log(greet("World"));
};
`
	err := os.WriteFile(testFile, []byte(jsCode), 0644)
	require.NoError(t, err)

	config := slicing.NewLanguageConfig()
	factory := language.NewFactory(config)
	s := slicing.NewSlicer(config, factory)

	files := []repository.File{
		repository.NewFile("abc123", "test.js", "javascript", int64(len(jsCode))),
	}

	cfg := slicing.DefaultSliceConfig()
	result, err := s.Slice(context.Background(), files, tmpDir, cfg)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Functions())
}

func TestAnalyzers_PythonDocstring(t *testing.T) {
	config := slicing.NewLanguageConfig()
	lang, _ := config.ByExtension(".py")
	analyzer := language.NewPython(lang)

	source := []byte(`def foo():
    """This is a docstring."""
    pass
`)

	sitterParser := newParser(lang)
	tree, err := sitterParser.ParseCtx(context.Background(), nil, source)
	require.NoError(t, err)

	walker := slicing.NewWalker()
	funcNodes := walker.CollectNodes(tree.RootNode(), []string{"function_definition"})
	require.Len(t, funcNodes, 1)

	docstring := analyzer.Docstring(funcNodes[0], source)
	assert.Equal(t, "This is a docstring.", docstring)
}

func TestAnalyzers_GoPublicFunction(t *testing.T) {
	config := slicing.NewLanguageConfig()
	lang, _ := config.ByExtension(".go")
	analyzer := language.NewGo(lang)

	assert.True(t, analyzer.IsPublic(nil, "PublicFunc", nil))
	assert.False(t, analyzer.IsPublic(nil, "privateFunc", nil))
}

func TestAnalyzers_PythonPublicFunction(t *testing.T) {
	config := slicing.NewLanguageConfig()
	lang, _ := config.ByExtension(".py")
	analyzer := language.NewPython(lang)

	assert.True(t, analyzer.IsPublic(nil, "public_func", nil))
	assert.False(t, analyzer.IsPublic(nil, "_private_func", nil))
	assert.False(t, analyzer.IsPublic(nil, "__dunder__", nil))
}

func TestSliceConfig_Defaults(t *testing.T) {
	cfg := slicing.DefaultSliceConfig()

	assert.Equal(t, 2, cfg.MaxDependencyDepth)
	assert.Equal(t, 8, cfg.MaxDependencyCount)
	assert.Equal(t, 2, cfg.MaxExamples)
	assert.False(t, cfg.IncludePrivate)
}
