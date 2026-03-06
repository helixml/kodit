package kodit

import (
	mcpinternal "github.com/helixml/kodit/internal/mcp"
)

// Parameter describes a single parameter accepted by an MCP tool.
type Parameter struct {
	name        string
	description string
	typ         string
	required    bool
}

// NewParameter creates a Parameter.
func NewParameter(name, description, typ string, required bool) Parameter {
	return Parameter{
		name:        name,
		description: description,
		typ:         typ,
		required:    required,
	}
}

// Name returns the parameter name.
func (p Parameter) Name() string { return p.name }

// Description returns the parameter description.
func (p Parameter) Description() string { return p.description }

// Type returns the parameter type (e.g. "string", "number").
func (p Parameter) Type() string { return p.typ }

// Required reports whether the parameter is required.
func (p Parameter) Required() bool { return p.required }

// Tool describes an MCP tool with its parameters.
type Tool struct {
	name        string
	description string
	parameters  []Parameter
}

// NewTool creates a Tool.
func NewTool(name, description string, parameters []Parameter) Tool {
	cp := make([]Parameter, len(parameters))
	copy(cp, parameters)
	return Tool{
		name:        name,
		description: description,
		parameters:  cp,
	}
}

// Name returns the tool name.
func (t Tool) Name() string { return t.name }

// Description returns the tool description.
func (t Tool) Description() string { return t.description }

// Parameters returns a copy of the tool's parameters.
func (t Tool) Parameters() []Parameter {
	cp := make([]Parameter, len(t.parameters))
	copy(cp, t.parameters)
	return cp
}

// MCPServer describes the metadata of a kodit MCP server: its usage
// instructions and the tools it provides.
type MCPServer struct {
	instructions string
	tools        []Tool
}

// NewMCPServer creates an MCPServer.
func NewMCPServer(instructions string, tools []Tool) MCPServer {
	cp := make([]Tool, len(tools))
	copy(cp, tools)
	return MCPServer{
		instructions: instructions,
		tools:        cp,
	}
}

// Instructions returns the server's usage instructions.
func (s MCPServer) Instructions() string { return s.instructions }

// Tools returns a copy of the server's tools.
func (s MCPServer) Tools() []Tool {
	cp := make([]Tool, len(s.tools))
	copy(cp, s.tools)
	return cp
}

// mcpServerFromDefinitions builds an MCPServer from the internal MCP tool
// definitions, which are the single source of truth shared with the MCP
// server registration.
func mcpServerFromDefinitions() MCPServer {
	defs := mcpinternal.ToolDefinitions()
	result := make([]Tool, len(defs))
	for i, def := range defs {
		defParams := def.Params()
		params := make([]Parameter, len(defParams))
		for j, p := range defParams {
			params[j] = NewParameter(p.Name(), p.Description(), p.Type(), p.Required())
		}
		result[i] = NewTool(def.Name(), def.Description(), params)
	}
	return NewMCPServer(mcpinternal.ServerInstructions(), result)
}
