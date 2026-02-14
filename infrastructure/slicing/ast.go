package slicing

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// Walker provides AST traversal utilities.
type Walker struct{}

// NewWalker creates a new Walker.
func NewWalker() Walker {
	return Walker{}
}

// WalkFunc is called for each node during traversal.
// Return false to stop traversal.
type WalkFunc func(node *sitter.Node) bool

// Walk performs a breadth-first traversal of the AST.
func (w Walker) Walk(root *sitter.Node, fn WalkFunc) {
	if root == nil {
		return
	}

	queue := []*sitter.Node{root}
	visited := make(map[uintptr]struct{})

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		nodeID := current.ID()
		if _, ok := visited[nodeID]; ok {
			continue
		}
		visited[nodeID] = struct{}{}

		if !fn(current) {
			return
		}

		for i := uint32(0); i < current.ChildCount(); i++ {
			child := current.Child(int(i))
			if child != nil {
				queue = append(queue, child)
			}
		}
	}
}

// CollectNodes returns all nodes of the specified types.
func (w Walker) CollectNodes(root *sitter.Node, nodeTypes []string) []*sitter.Node {
	typeSet := make(map[string]struct{})
	for _, t := range nodeTypes {
		typeSet[t] = struct{}{}
	}

	var nodes []*sitter.Node
	w.Walk(root, func(node *sitter.Node) bool {
		if _, ok := typeSet[node.Type()]; ok {
			nodes = append(nodes, node)
		}
		return true
	})

	return nodes
}

// FindChildByType finds the first direct child with the specified type.
func (w Walker) FindChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	if node == nil {
		return nil
	}

	for i := uint32(0); i < node.ChildCount(); i++ {
		child := node.Child(int(i))
		if child != nil && child.Type() == nodeType {
			return child
		}
	}

	return nil
}

// FindChildByField finds the child with the specified field name.
func (w Walker) FindChildByField(node *sitter.Node, fieldName string) *sitter.Node {
	if node == nil {
		return nil
	}

	return node.ChildByFieldName(fieldName)
}

// FindDescendant finds the first descendant with the specified type.
func (w Walker) FindDescendant(root *sitter.Node, nodeType string) *sitter.Node {
	if root == nil {
		return nil
	}

	var result *sitter.Node
	w.Walk(root, func(node *sitter.Node) bool {
		if node.Type() == nodeType {
			result = node
			return false
		}
		return true
	})

	return result
}

// CollectDescendants returns all descendants with the specified type.
func (w Walker) CollectDescendants(root *sitter.Node, nodeType string) []*sitter.Node {
	var nodes []*sitter.Node
	w.Walk(root, func(node *sitter.Node) bool {
		if node.Type() == nodeType {
			nodes = append(nodes, node)
		}
		return true
	})
	return nodes
}

// NodeText extracts the text content of a node.
func (w Walker) NodeText(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	start := node.StartByte()
	end := node.EndByte()

	if start >= uint32(len(source)) || end > uint32(len(source)) || start >= end {
		return ""
	}

	return string(source[start:end])
}

// PreviousSibling returns the previous sibling of a node.
func (w Walker) PreviousSibling(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	return node.PrevSibling()
}

// NextSibling returns the next sibling of a node.
func (w Walker) NextSibling(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	return node.NextSibling()
}

// Parent returns the parent of a node.
func (w Walker) Parent(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	return node.Parent()
}

// ChildCount returns the number of children.
func (w Walker) ChildCount(node *sitter.Node) uint32 {
	if node == nil {
		return 0
	}
	return node.ChildCount()
}

// Child returns the child at the specified index.
func (w Walker) Child(node *sitter.Node, index int) *sitter.Node {
	if node == nil {
		return nil
	}
	return node.Child(index)
}

// IsIdentifier checks if a node is an identifier type.
func (w Walker) IsIdentifier(node *sitter.Node) bool {
	if node == nil {
		return false
	}

	identifierTypes := map[string]struct{}{
		"identifier":                    {},
		"type_identifier":               {},
		"field_identifier":              {},
		"property_identifier":           {},
		"shorthand_property_identifier": {},
	}

	_, ok := identifierTypes[node.Type()]
	return ok
}

// IsComment checks if a node is a comment type.
func (w Walker) IsComment(node *sitter.Node) bool {
	if node == nil {
		return false
	}

	commentTypes := map[string]struct{}{
		"comment":       {},
		"line_comment":  {},
		"block_comment": {},
	}

	_, ok := commentTypes[node.Type()]
	return ok
}

// IsString checks if a node is a string type.
func (w Walker) IsString(node *sitter.Node) bool {
	if node == nil {
		return false
	}

	stringTypes := map[string]struct{}{
		"string":                     {},
		"string_literal":             {},
		"interpreted_string_literal": {},
		"raw_string_literal":         {},
		"template_string":            {},
	}

	_, ok := stringTypes[node.Type()]
	return ok
}

// CallGraph represents function call relationships.
type CallGraph struct {
	calls        map[string]map[string]struct{}
	reverseCalls map[string]map[string]struct{}
}

// NewCallGraph creates an empty CallGraph.
func NewCallGraph() *CallGraph {
	return &CallGraph{
		calls:        make(map[string]map[string]struct{}),
		reverseCalls: make(map[string]map[string]struct{}),
	}
}

// AddCall registers a function call relationship.
func (g *CallGraph) AddCall(caller, callee string) {
	if g.calls[caller] == nil {
		g.calls[caller] = make(map[string]struct{})
	}
	g.calls[caller][callee] = struct{}{}

	if g.reverseCalls[callee] == nil {
		g.reverseCalls[callee] = make(map[string]struct{})
	}
	g.reverseCalls[callee][caller] = struct{}{}
}

// Callees returns functions called by the specified function.
func (g *CallGraph) Callees(name string) []string {
	callees, ok := g.calls[name]
	if !ok {
		return nil
	}

	result := make([]string, 0, len(callees))
	for callee := range callees {
		result = append(result, callee)
	}
	return result
}

// Callers returns functions that call the specified function.
func (g *CallGraph) Callers(name string) []string {
	callers, ok := g.reverseCalls[name]
	if !ok {
		return nil
	}

	result := make([]string, 0, len(callers))
	for caller := range callers {
		result = append(result, caller)
	}
	return result
}

// Dependencies finds all transitive dependencies of a function.
func (g *CallGraph) Dependencies(name string, maxDepth, maxCount int) []string {
	var result []string
	visited := make(map[string]struct{})
	queue := []struct {
		name  string
		depth int
	}{{name, 0}}

	for len(queue) > 0 && len(result) < maxCount {
		current := queue[0]
		queue = queue[1:]

		if current.depth > maxDepth {
			continue
		}

		if _, ok := visited[current.name]; ok {
			continue
		}
		visited[current.name] = struct{}{}

		if current.name != name {
			result = append(result, current.name)
		}

		for _, callee := range g.Callees(current.name) {
			if _, ok := visited[callee]; !ok {
				queue = append(queue, struct {
					name  string
					depth int
				}{callee, current.depth + 1})
			}
		}
	}

	return result
}
