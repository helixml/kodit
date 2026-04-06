package mcp

import (
	"strings"
	"testing"
)

func TestToolDefinitions_Count(t *testing.T) {
	defs := ToolDefinitions()

	if len(defs) != 15 {
		names := make([]string, len(defs))
		for i, def := range defs {
			names[i] = def.Name()
		}
		t.Fatalf("ToolDefinitions() length = %d, want 15; got %v", len(defs), names)
	}
}

func TestServerInstructions(t *testing.T) {
	instr := ServerInstructions()

	if instr == "" {
		t.Fatal("ServerInstructions() is empty")
	}
	for _, phrase := range []string{"kodit_repositories", "kodit_semantic_search", "kodit_keyword_search", "kodit_grep"} {
		if !strings.Contains(instr, phrase) {
			t.Errorf("ServerInstructions() does not contain %q", phrase)
		}
	}
}

func TestToolDefinitions_SemanticSearch(t *testing.T) {
	var found bool
	for _, def := range ToolDefinitions() {
		if def.Name() != "kodit_semantic_search" {
			continue
		}
		found = true

		params := def.Params()
		if len(params) != 4 {
			t.Fatalf("semantic_search params = %d, want 4", len(params))
		}

		byName := map[string]struct {
			typ      string
			required bool
		}{}
		for _, p := range params {
			byName[p.Name()] = struct {
				typ      string
				required bool
			}{p.Type(), p.Required()}
		}

		if p, ok := byName["query"]; !ok {
			t.Error("missing query param")
		} else {
			if p.typ != "string" {
				t.Errorf("query type = %q, want string", p.typ)
			}
			if !p.required {
				t.Error("query should be required")
			}
		}

		for _, name := range []string{"language", "source_repo", "limit"} {
			p, ok := byName[name]
			if !ok {
				t.Errorf("missing %s param", name)
				continue
			}
			if p.required {
				t.Errorf("%s should be optional", name)
			}
		}

		if byName["limit"].typ != "number" {
			t.Errorf("limit type = %q, want number", byName["limit"].typ)
		}
	}

	if !found {
		t.Fatal("semantic_search tool not found")
	}
}
