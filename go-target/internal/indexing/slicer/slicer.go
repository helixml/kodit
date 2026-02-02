package slicer

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/helixml/kodit/internal/git"
	"github.com/helixml/kodit/internal/indexing"
)

// Slicer extracts code snippets from source files using AST parsing.
type Slicer struct {
	config          LanguageConfig
	analyzerFactory AnalyzerFactory
	walker          Walker
}

// AnalyzerFactory creates analyzers for different languages.
type AnalyzerFactory interface {
	ByExtension(ext string) (Analyzer, bool)
}

// NewSlicer creates a new Slicer.
func NewSlicer(config LanguageConfig, factory AnalyzerFactory) *Slicer {
	return &Slicer{
		config:          config,
		analyzerFactory: factory,
		walker:          NewWalker(),
	}
}

// SliceConfig configures snippet extraction behavior.
type SliceConfig struct {
	MaxDependencyDepth int
	MaxDependencyCount int
	MaxExamples        int
	IncludePrivate     bool
}

// DefaultSliceConfig returns default configuration.
func DefaultSliceConfig() SliceConfig {
	return SliceConfig{
		MaxDependencyDepth: 2,
		MaxDependencyCount: 8,
		MaxExamples:        2,
		IncludePrivate:     false,
	}
}

// SliceResult contains the output of slicing a set of files.
type SliceResult struct {
	snippets  []indexing.Snippet
	functions []FunctionDefinition
	classes   []ClassDefinition
	types     []TypeDefinition
	callGraph *CallGraph
}

// NewSliceResult creates an empty SliceResult.
func NewSliceResult() SliceResult {
	return SliceResult{
		snippets:  make([]indexing.Snippet, 0),
		functions: make([]FunctionDefinition, 0),
		classes:   make([]ClassDefinition, 0),
		types:     make([]TypeDefinition, 0),
		callGraph: NewCallGraph(),
	}
}

// Snippets returns the extracted snippets.
func (r SliceResult) Snippets() []indexing.Snippet { return r.snippets }

// Functions returns the extracted function definitions.
func (r SliceResult) Functions() []FunctionDefinition { return r.functions }

// Classes returns the extracted class definitions.
func (r SliceResult) Classes() []ClassDefinition { return r.classes }

// Types returns the extracted type definitions.
func (r SliceResult) Types() []TypeDefinition { return r.types }

// CallGraph returns the function call graph.
func (r SliceResult) CallGraph() *CallGraph { return r.callGraph }

// State holds parsing state during slicing.
type State struct {
	files        []ParsedFile
	defIndex     map[string]FunctionDefinition
	callGraph    *CallGraph
	importIndex  map[string]map[string]string
}

// Slice extracts snippets from the given files.
func (s *Slicer) Slice(ctx context.Context, files []git.File, basePath string, cfg SliceConfig) (SliceResult, error) {
	result := NewSliceResult()
	state := &State{
		files:       make([]ParsedFile, 0, len(files)),
		defIndex:    make(map[string]FunctionDefinition),
		callGraph:   NewCallGraph(),
		importIndex: make(map[string]map[string]string),
	}

	for _, file := range files {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		parsed, err := s.parseFile(file, basePath)
		if err != nil {
			continue
		}

		if parsed.tree == nil {
			continue
		}

		state.files = append(state.files, parsed)
	}

	for _, parsed := range state.files {
		s.extractDefinitions(parsed, state, cfg)
	}

	for _, parsed := range state.files {
		s.buildCallGraph(parsed, state)
	}

	result.callGraph = state.callGraph

	for name, funcDef := range state.defIndex {
		result.functions = append(result.functions, funcDef)

		if !funcDef.IsPublic() && !cfg.IncludePrivate {
			continue
		}

		snippet := s.buildSnippet(name, funcDef, state, cfg)
		result.snippets = append(result.snippets, snippet)
	}

	return result, nil
}

func (s *Slicer) parseFile(file git.File, basePath string) (ParsedFile, error) {
	fullPath := filepath.Join(basePath, file.Path())
	ext := filepath.Ext(file.Path())

	lang, ok := s.config.ByExtension(ext)
	if !ok {
		return ParsedFile{}, nil
	}

	source, err := os.ReadFile(fullPath)
	if err != nil {
		return ParsedFile{}, err
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang.SitterLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return ParsedFile{}, err
	}

	return NewParsedFile(file.Path(), tree, source), nil
}

func (s *Slicer) extractDefinitions(parsed ParsedFile, state *State, cfg SliceConfig) {
	ext := filepath.Ext(parsed.Path())
	analyzer, ok := s.analyzerFactory.ByExtension(ext)
	if !ok {
		return
	}

	modulePath := analyzer.ModulePath(parsed)
	source := parsed.SourceCode()
	tree := parsed.Tree()
	nodes := tree.RootNode()

	langNodes := analyzer.Language().Nodes()
	funcTypes := append(langNodes.FunctionNodes(), langNodes.MethodNodes()...)
	funcNodes := s.walker.CollectNodes(nodes, funcTypes)

	for _, node := range funcNodes {
		name := analyzer.FunctionName(node, source)
		if name == "" {
			continue
		}

		qualifiedName := buildQualified(modulePath, name)

		if analyzer.IsMethod(node) {
			receiverName := s.extractReceiverName(node, source, analyzer)
			if receiverName != "" {
				qualifiedName = buildQualified(modulePath, receiverName+"."+name)
			}
		}

		funcDef := NewFunctionDefinition(
			parsed.Path(),
			node,
			node.StartByte(),
			node.EndByte(),
			qualifiedName,
			name,
			analyzer.IsPublic(node, name, source),
			analyzer.IsMethod(node),
			analyzer.Docstring(node, source),
			nil,
			"",
		)

		state.defIndex[qualifiedName] = funcDef
	}

	classes := analyzer.Classes(tree, source)
	for _, class := range classes {
		for _, method := range class.Methods() {
			if !method.IsPublic() && !cfg.IncludePrivate {
				continue
			}
			state.defIndex[method.QualifiedName()] = method
		}
	}

	types := analyzer.Types(tree, source)
	for range types {
	}
}

func (s *Slicer) extractReceiverName(node *sitter.Node, source []byte, analyzer Analyzer) string {
	receiver := node.ChildByFieldName("receiver")
	if receiver == nil {
		return ""
	}

	var typeName string
	s.walker.Walk(receiver, func(n *sitter.Node) bool {
		if n.Type() == "type_identifier" {
			typeName = s.walker.NodeText(n, source)
			return false
		}
		return true
	})

	return typeName
}

func (s *Slicer) buildCallGraph(parsed ParsedFile, state *State) {
	ext := filepath.Ext(parsed.Path())
	analyzer, ok := s.analyzerFactory.ByExtension(ext)
	if !ok {
		return
	}

	modulePath := analyzer.ModulePath(parsed)
	source := parsed.SourceCode()
	tree := parsed.Tree()
	nodes := tree.RootNode()

	langNodes := analyzer.Language().Nodes()
	funcTypes := append(langNodes.FunctionNodes(), langNodes.MethodNodes()...)
	funcNodes := s.walker.CollectNodes(nodes, funcTypes)

	for _, funcNode := range funcNodes {
		funcName := analyzer.FunctionName(funcNode, source)
		if funcName == "" {
			continue
		}

		callerQualified := buildQualified(modulePath, funcName)

		if analyzer.IsMethod(funcNode) {
			receiverName := s.extractReceiverName(funcNode, source, analyzer)
			if receiverName != "" {
				callerQualified = buildQualified(modulePath, receiverName+"."+funcName)
			}
		}

		callNodeType := langNodes.CallNode()
		callNodes := s.walker.CollectDescendants(funcNode, callNodeType)

		for _, callNode := range callNodes {
			calleeName := s.extractCalleeName(callNode, source)
			if calleeName == "" {
				continue
			}

			calleeQualified := s.resolveCallee(calleeName, modulePath, state)
			state.callGraph.AddCall(callerQualified, calleeQualified)
		}
	}
}

func (s *Slicer) extractCalleeName(node *sitter.Node, source []byte) string {
	funcNode := node.ChildByFieldName("function")
	if funcNode != nil {
		return s.walker.NodeText(funcNode, source)
	}

	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return s.walker.NodeText(nameNode, source)
	}

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child != nil && s.walker.IsIdentifier(child) {
			return s.walker.NodeText(child, source)
		}
	}

	return ""
}

func (s *Slicer) resolveCallee(name, modulePath string, state *State) string {
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		name = parts[len(parts)-1]
	}

	qualified := buildQualified(modulePath, name)
	if _, ok := state.defIndex[qualified]; ok {
		return qualified
	}

	for qname := range state.defIndex {
		if strings.HasSuffix(qname, "."+name) {
			return qname
		}
	}

	return name
}

func (s *Slicer) buildSnippet(name string, funcDef FunctionDefinition, state *State, cfg SliceConfig) indexing.Snippet {
	var contentParts []string

	source, err := os.ReadFile(funcDef.FilePath())
	if err == nil {
		start, end := funcDef.Span()
		if start < uint32(len(source)) && end <= uint32(len(source)) {
			funcSource := string(source[start:end])
			contentParts = append(contentParts, funcSource)
		}
	}

	deps := state.callGraph.Dependencies(name, cfg.MaxDependencyDepth, cfg.MaxDependencyCount)
	for _, depName := range deps {
		depDef, ok := state.defIndex[depName]
		if !ok {
			continue
		}

		depSource, err := os.ReadFile(depDef.FilePath())
		if err != nil {
			continue
		}

		start, end := depDef.Span()
		if start < uint32(len(depSource)) && end <= uint32(len(depSource)) {
			depContent := string(depSource[start:end])
			contentParts = append(contentParts, depContent)
		}
	}

	callers := state.callGraph.Callers(name)
	sort.Strings(callers)

	exampleCount := 0
	for _, callerName := range callers {
		if exampleCount >= cfg.MaxExamples {
			break
		}

		callerDef, ok := state.defIndex[callerName]
		if !ok {
			continue
		}

		callerSource, err := os.ReadFile(callerDef.FilePath())
		if err != nil {
			continue
		}

		start, end := callerDef.Span()
		if start < uint32(len(callerSource)) && end <= uint32(len(callerSource)) {
			exampleContent := string(callerSource[start:end])
			contentParts = append(contentParts, "// Example usage:\n"+exampleContent)
			exampleCount++
		}
	}

	content := strings.Join(contentParts, "\n\n")

	ext := filepath.Ext(funcDef.FilePath())
	derivesFrom := []git.File{
		git.NewFile("", funcDef.FilePath(), extToLanguage(ext), 0),
	}

	return indexing.NewSnippet(content, ext, derivesFrom)
}

func buildQualified(modulePath, name string) string {
	if modulePath == "" {
		return name
	}
	return modulePath + "." + name
}

func extToLanguage(ext string) string {
	languages := map[string]string{
		".py":   "python",
		".go":   "go",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".cc":   "cpp",
		".cxx":  "cpp",
		".rs":   "rust",
		".js":   "javascript",
		".ts":   "typescript",
		".tsx":  "tsx",
		".cs":   "csharp",
	}

	if lang, ok := languages[ext]; ok {
		return lang
	}
	return ""
}
