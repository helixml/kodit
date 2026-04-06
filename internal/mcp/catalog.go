package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// ParamDefinition describes a single parameter in a tool definition.
type ParamDefinition struct {
	name        string
	description string
	typ         string
	required    bool
}

// Name returns the parameter name.
func (p ParamDefinition) Name() string { return p.name }

// Description returns the parameter description.
func (p ParamDefinition) Description() string { return p.description }

// Type returns the parameter type (e.g. "string", "number").
func (p ParamDefinition) Type() string { return p.typ }

// Required reports whether the parameter is required.
func (p ParamDefinition) Required() bool { return p.required }

// ToolDefinition describes a tool and its parameters.
type ToolDefinition struct {
	name        string
	description string
	params      []ParamDefinition
}

// Name returns the tool name.
func (d ToolDefinition) Name() string { return d.name }

// Description returns the tool description.
func (d ToolDefinition) Description() string { return d.description }

// Params returns the tool's parameter definitions.
func (d ToolDefinition) Params() []ParamDefinition {
	cp := make([]ParamDefinition, len(d.params))
	copy(cp, d.params)
	return cp
}

// tools returns the canonical list of tool definitions. Both ToolDefinitions()
// and registerTools() build from this list so they stay in sync.
func tools() []ToolDefinition {
	return []ToolDefinition{
		{
			name:        "kodit_version",
			description: "Get the kodit server version",
		},
		{
			name:        "kodit_repositories",
			description: "List all repositories tracked by kodit",
		},
		{
			name:        "kodit_architecture_docs",
			description: "Get high-level architecture documentation for a repository",
			params: []ParamDefinition{
				{name: "repo_url", description: "The remote URL of the repository", typ: "string", required: true},
				{name: "commit_sha", description: "The commit SHA to get docs for (defaults to latest)", typ: "string"},
			},
		},
		{
			name:        "kodit_api_docs",
			description: "Get API documentation for a repository",
			params: []ParamDefinition{
				{name: "repo_url", description: "The remote URL of the repository", typ: "string", required: true},
				{name: "commit_sha", description: "The commit SHA to get docs for (defaults to latest)", typ: "string"},
			},
		},
		{
			name:        "kodit_commit_description",
			description: "Get commit description for a repository",
			params: []ParamDefinition{
				{name: "repo_url", description: "The remote URL of the repository", typ: "string", required: true},
				{name: "commit_sha", description: "The commit SHA to get docs for (defaults to latest)", typ: "string"},
			},
		},
		{
			name:        "kodit_database_schema",
			description: "Get database schema documentation for a repository",
			params: []ParamDefinition{
				{name: "repo_url", description: "The remote URL of the repository", typ: "string", required: true},
				{name: "commit_sha", description: "The commit SHA to get docs for (defaults to latest)", typ: "string"},
			},
		},
		{
			name:        "kodit_cookbook",
			description: "Get cookbook with usage examples for a repository",
			params: []ParamDefinition{
				{name: "repo_url", description: "The remote URL of the repository", typ: "string", required: true},
				{name: "commit_sha", description: "The commit SHA to get docs for (defaults to latest)", typ: "string"},
			},
		},
		{
			name:        "kodit_wiki",
			description: "Get the table of contents for a repository's wiki",
			params: []ParamDefinition{
				{name: "repo_url", description: "The remote URL of the repository", typ: "string", required: true},
				{name: "commit_sha", description: "The commit SHA to get the wiki for (defaults to latest)", typ: "string"},
			},
		},
		{
			name:        "kodit_wiki_page",
			description: "Get the content of a specific wiki page",
			params: []ParamDefinition{
				{name: "repo_url", description: "The remote URL of the repository", typ: "string", required: true},
				{name: "page_slug", description: "The slug of the wiki page to retrieve", typ: "string", required: true},
				{name: "commit_sha", description: "The commit SHA to get the wiki for (defaults to latest)", typ: "string"},
			},
		},
		{
			name:        "kodit_semantic_search",
			description: "Search indexed files using semantic similarity and return file resource URIs",
			params: []ParamDefinition{
				{name: "query", description: "Natural language description of what you are looking for", typ: "string", required: true},
				{name: "language", description: "Filter by file extension (e.g. .go, .py)", typ: "string"},
				{name: "source_repo", description: "Filter by source repository URL", typ: "string"},
				{name: "limit", description: "Maximum number of results (default 10)", typ: "number"},
			},
		},
		{
			name:        "kodit_keyword_search",
			description: "Search indexed files using keyword-based BM25 search and return file resource URIs",
			params: []ParamDefinition{
				{name: "keywords", description: "Keywords to search for", typ: "string", required: true},
				{name: "source_repo", description: "Filter by source repository URL", typ: "string"},
				{name: "language", description: "Filter by programming language", typ: "string"},
				{name: "limit", description: "Maximum number of results (default 10)", typ: "number"},
			},
		},
		{
			name:        "kodit_visual_search",
			description: "Search indexed document pages (PDFs, etc.) using visual similarity. Embeds a text query and matches against page image embeddings to find visually relevant pages.",
			params: []ParamDefinition{
				{name: "query", description: "Natural language description of what you are looking for in document pages", typ: "string", required: true},
				{name: "source_repo", description: "Filter by source repository URL", typ: "string"},
				{name: "limit", description: "Maximum number of results (default 10)", typ: "number"},
			},
		},
		{
			name:        "kodit_grep",
			description: "Search file contents in a repository using git grep with regex patterns. Returns matching file URIs with line numbers. Use for exact/regex matching; use kodit_keyword_search for fuzzy/semantic matching.",
			params: []ParamDefinition{
				{name: "repo_url", description: "The remote URL of the repository", typ: "string", required: true},
				{name: "pattern", description: "Regex pattern to search for (git grep syntax)", typ: "string", required: true},
				{name: "glob", description: "File path filter (e.g. \"*.go\", \"src/**/*.ts\")", typ: "string"},
				{name: "limit", description: "Maximum number of file results (default 50)", typ: "number"},
			},
		},
		{
			name:        "kodit_read_resource",
			description: "Read the contents of a file resource URI. Use this to fetch file content from URIs returned by kodit_semantic_search, kodit_keyword_search, kodit_grep, and kodit_ls.",
			params: []ParamDefinition{
				{name: "uri", description: "The file resource URI (e.g. file://1/main/src/foo.go?lines=L17-L26&line_numbers=true)", typ: "string", required: true},
			},
		},
		{
			name:        "kodit_ls",
			description: "List files matching a glob pattern in a repository",
			params: []ParamDefinition{
				{name: "repo_url", description: "The remote URL of the repository", typ: "string", required: true},
				{name: "pattern", description: "Glob pattern to match files (e.g. **/*.go, src/*.py)", typ: "string", required: true},
			},
		},
	}
}

// ToolDefinitions returns the canonical list of MCP tool definitions.
func ToolDefinitions() []ToolDefinition {
	return tools()
}

// ServerInstructions returns the MCP server usage instructions.
func ServerInstructions() string {
	return instructions
}

// mcpTool converts a ToolDefinition into an mcp.Tool for server registration.
func mcpTool(def ToolDefinition) mcp.Tool {
	opts := []mcp.ToolOption{mcp.WithDescription(def.description)}
	for _, p := range def.params {
		switch p.typ {
		case "number":
			if p.required {
				opts = append(opts, mcp.WithNumber(p.name, mcp.Required(), mcp.Description(p.description)))
			} else {
				opts = append(opts, mcp.WithNumber(p.name, mcp.Description(p.description)))
			}
		default:
			if p.required {
				opts = append(opts, mcp.WithString(p.name, mcp.Required(), mcp.Description(p.description)))
			} else {
				opts = append(opts, mcp.WithString(p.name, mcp.Description(p.description)))
			}
		}
	}
	return mcp.NewTool(def.name, opts...)
}
