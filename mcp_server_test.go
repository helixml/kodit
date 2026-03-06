package kodit

import (
	"testing"
)

func TestParameter(t *testing.T) {
	p := NewParameter("query", "Natural language query", "string", true)

	if p.Name() != "query" {
		t.Errorf("Name() = %q, want %q", p.Name(), "query")
	}
	if p.Description() != "Natural language query" {
		t.Errorf("Description() = %q, want %q", p.Description(), "Natural language query")
	}
	if p.Type() != "string" {
		t.Errorf("Type() = %q, want %q", p.Type(), "string")
	}
	if !p.Required() {
		t.Error("Required() = false, want true")
	}
}

func TestParameter_Optional(t *testing.T) {
	p := NewParameter("limit", "Max results", "number", false)

	if p.Required() {
		t.Error("Required() = true, want false")
	}
}

func TestTool(t *testing.T) {
	params := []Parameter{
		NewParameter("query", "Search query", "string", true),
		NewParameter("limit", "Max results", "number", false),
	}
	tool := NewTool("semantic_search", "Search using semantic similarity", params)

	if tool.Name() != "semantic_search" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "semantic_search")
	}
	if tool.Description() != "Search using semantic similarity" {
		t.Errorf("Description() = %q, want %q", tool.Description(), "Search using semantic similarity")
	}
	got := tool.Parameters()
	if len(got) != 2 {
		t.Fatalf("Parameters() length = %d, want 2", len(got))
	}
	if got[0].Name() != "query" {
		t.Errorf("Parameters()[0].Name() = %q, want %q", got[0].Name(), "query")
	}
	if got[1].Name() != "limit" {
		t.Errorf("Parameters()[1].Name() = %q, want %q", got[1].Name(), "limit")
	}
}

func TestTool_ParametersDefensiveCopy(t *testing.T) {
	params := []Parameter{
		NewParameter("query", "Search query", "string", true),
	}
	tool := NewTool("test", "test tool", params)

	got := tool.Parameters()
	got[0] = NewParameter("mutated", "", "", false)

	if tool.Parameters()[0].Name() != "query" {
		t.Error("Parameters() returned a reference that allowed mutation")
	}
}

func TestMCPServer(t *testing.T) {
	tools := []Tool{
		NewTool("list_repositories", "List repos", nil),
		NewTool("semantic_search", "Search code", []Parameter{
			NewParameter("query", "Search query", "string", true),
		}),
	}
	srv := NewMCPServer("Use these tools to explore code.", tools)

	if srv.Instructions() != "Use these tools to explore code." {
		t.Errorf("Instructions() = %q, want %q", srv.Instructions(), "Use these tools to explore code.")
	}
	got := srv.Tools()
	if len(got) != 2 {
		t.Fatalf("Tools() length = %d, want 2", len(got))
	}
	if got[0].Name() != "list_repositories" {
		t.Errorf("Tools()[0].Name() = %q, want %q", got[0].Name(), "list_repositories")
	}
	if got[1].Name() != "semantic_search" {
		t.Errorf("Tools()[1].Name() = %q, want %q", got[1].Name(), "semantic_search")
	}
}

func TestMCPServer_ToolsDefensiveCopy(t *testing.T) {
	tools := []Tool{
		NewTool("test", "test tool", nil),
	}
	srv := NewMCPServer("instructions", tools)

	got := srv.Tools()
	got[0] = NewTool("mutated", "", nil)

	if srv.Tools()[0].Name() != "test" {
		t.Error("Tools() returned a reference that allowed mutation")
	}
}
