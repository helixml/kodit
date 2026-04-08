---
title: MCP
description: Model Context Protocol (MCP) server implementation for AI coding assistants
weight: 2
---

The [Model Context Protocol](https://modelcontextprotocol.io/introduction) (MCP) is a
standard that enables AI assistants to communicate with external tools and data sources.

Kodit provides an MCP server that enables AI coding assistants to search, browse, and
retrieve relevant code and documentation from your indexed codebases.

## MCP Server Connection

Kodit runs an HTTP server that streams responses to connected AI coding assistants over
the `/mcp` endpoint.

- **How it works:** Kodit starts a local web server and listens for HTTP requests from
  your AI assistant. Responses are streamed for low-latency, real-time results.
- **When to use:** Most modern AI coding assistants (like Cursor, Cline, etc.) support
  HTTP streaming. Use this for best compatibility and performance.
- **How to start:**

  ```sh
  kodit serve
  ```

  The server will listen on `http://localhost:8080/mcp` by default. If you're using the
  Kodit container, `kodit serve` is the default command.

## Integration with AI Assistants

You need to connect your AI coding assistant to take advantage of Kodit. The
instructions to do this depend on how you've deployed it. This section provides
comprehensive instructions for all popular coding assistants.

### Integration With Claude Code

#### Claude Code Streaming HTTP Mode (recommended)

```sh
claude mcp add --transport http kodit http://localhost:8080/mcp
```

### Integration With Cursor

#### Cursor Streaming HTTP Mode (recommended)

Add the following to `$HOME/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "kodit": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

- Or find this configuration in `Cursor Settings` -> `MCP`.
- Replace `http://localhost:8080` with the URL of your Kodit instance if running remotely.

### Integration With Cline

1. Open Cline from the side menu
2. Click the `MCP Servers` button at the top right of the Cline window (the icon looks
   like a server)
3. Click the `Remote Servers` tab.
4. Click `Edit Configuration`

#### Cline Streaming HTTP Mode (recommended)

Add the following configuration:

```json
{
  "mcpServers": {
    "kodit": {
      "autoApprove": [],
      "disabled": false,
      "timeout": 60,
      "type": "streamableHttp",
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

### Integration With Kilo Code

1. Open Kilo Code from the side menu
2. Click the `MCP Servers` button at the top right of the Kilo Code window (the icon looks
   like a server)
3. Click the `Edit Project/Global MCP` button.

Add the following configuration:

```json
{
  "mcpServers": {
    "kodit": {
      "type": "streamable-http",
      "url": "http://localhost:8080/mcp",
      "alwaysAllow": [],
      "disabled": false
    }
  }
}
```

## Forcing AI Assistants to use Kodit

Although Kodit has been developed to work well out of the box with popular AI coding
assistants, they sometimes still think they know better.

You can force your assistant to use Kodit by editing the system prompt used by the
assistant. Each assistant exposes this slightly differently, but it's usually in the
settings.

Try using this system prompt:

```txt
For *every* user request that involves writing or modifying code (of any language or
domain), the assistant's *first* action **must** be to call the kodit.search MCP tool.
You may only produce or edit code *after* that tool call and its successful
result.
```

Feel free to alter that to suit your specific circumstances.

### Forcing Cursor to Use Kodit

Add the following prompt to `.cursor/rules/kodit.mdc` in your project directory:

```markdown
---
alwaysApply: true
---
For *every* user request that involves writing or modifying code (of any language or
domain), the assistant's *first* action **must** be to call the kodit.search MCP tool.
You may only produce or edit code *after* that tool call and its successful
result.
```

Alternatively, you can browse to the Cursor settings and set this prompt globally.

### Forcing Cline to Use Kodit

1. Go to `Settings` -> `API Configuration`
2. At the bottom there is a `Custom Instructions` section.

## MCP Tools

The Kodit MCP server exposes the following tools to AI coding assistants:

### Discovery Tools

| Tool | Description |
|------|-------------|
| `kodit_version` | Get the Kodit server version |
| `kodit_repositories` | List all indexed repositories. Call this first to discover available repos |

### Repository Knowledge Tools

These tools retrieve AI-generated documentation for a repository. They require a
`repo_url` parameter (the remote URL of the repository) and accept an optional
`commit_sha` to pin to a specific commit (defaults to latest).

| Tool | Description |
|------|-------------|
| `kodit_architecture_docs` | High-level architecture documentation |
| `kodit_api_docs` | API and interface documentation |
| `kodit_database_schema` | Database schema documentation |
| `kodit_cookbook` | Usage examples and cookbook entries |
| `kodit_commit_description` | Description and summary of a specific commit |

### Wiki Tools

Kodit generates a structured wiki for each repository. The wiki provides hierarchical
documentation pages covering architecture, APIs, data models, and more.

| Tool | Description |
|------|-------------|
| `kodit_wiki` | Get the table of contents for a repository's wiki. Parameters: `repo_url` (required), `commit_sha` (optional) |
| `kodit_wiki_page` | Get the content of a specific wiki page. Parameters: `repo_url` (required), `page_slug` (required), `commit_sha` (optional) |

### Search Tools

Search tools return file resource URIs that can be read with `kodit_read_resource`.

| Tool | Description |
|------|-------------|
| `kodit_semantic_search` | Semantic similarity search. Parameters: `query` (required), `language`, `source_repo`, `limit` |
| `kodit_keyword_search` | BM25 keyword search. Parameters: `keywords` (required), `source_repo`, `language`, `limit` |

### File Browsing Tools

| Tool | Description |
|------|-------------|
| `kodit_grep` | Regex search via git grep. Parameters: `repo_url` (required), `pattern` (required), `glob`, `limit` |
| `kodit_ls` | List files matching a glob pattern. Parameters: `repo_url` (required), `pattern` (required) |
| `kodit_read_resource` | Read file contents from a resource URI returned by search, grep, or ls. Parameters: `uri` (required) |

### Resource URI Format

Search, grep, and ls tools return file resource URIs in the format:

```
file://{repo_id}/{ref}/{path}?lines=L17-L26&line_numbers=true
```

Use `kodit_read_resource` to fetch the content of these URIs. The `lines` parameter
supports ranges like `L17-L26,L45,L55-L90` and `line_numbers=true` prefixes each line
with its 1-based line number.

## Filtering Capabilities

Kodit's MCP server supports comprehensive filtering to help AI assistants find the most relevant code examples.

### Language Filtering

Filter results by programming language:

**Example prompts:**
> "I need to create a web server in Python. Please search for Flask or FastAPI examples and show me the best practices."
> "I'm working on a Go microservice. Can you search for Go-specific patterns for handling HTTP requests and database connections?"

### Author Filtering

Filter results by code author:

**Example prompts:**
> "I'm reviewing code written by john.doe. Can you search for their authentication implementations to understand their coding style?"

### Date Range Filtering

Filter results by creation date:

**Example prompts:**
> "I need to see authentication patterns from 2025. Please search for JWT and OAuth implementations created in 2025."

### Source Repository Filtering

Filter results by source repository:

**Example prompts:**
> "I'm working on the auth-service project. Please search for authentication patterns specifically from github.com/company/auth-service."

### Combining Filters

You can combine multiple filters for precise results:

**Example prompts:**
> "I need Python authentication code written by alice.smith in 2025 from the auth-service repository."

## AI Assistant Integration Tips

### 1. Provide Clear User Intent

**Good examples:**

- "Create a REST API endpoint for user authentication with JWT tokens"
- "Implement a database connection pool for PostgreSQL"

**Poor examples:**

- "Help me with auth"
- "Database stuff"

### 2. Use Relevant Keywords

Provide specific, technical keywords that are relevant to your task. The language model
is more than capable of generating appropriate keywords for your intent.

### 3. Leverage File Context

If you're working with existing files, mention them in your prompt:

> "I'm working on the authentication function in auth.go. Can you search for similar error handling patterns?"

### 4. Use Language Filtering

Specify the programming language in your prompt:

> "I need to create a web server in Python. Please search for Flask and FastAPI examples."

### 5. Filter by Source Repository

If you have multiple codebases indexed, mention the specific repository:

> "Search for user management patterns from github.com/company/user-service."

## Troubleshooting

### Common Issues

1. **AI assistant not using Kodit**: Ensure you've configured the enforcement prompt and MCP server connection properly.

2. **No search results**: Check that you have indexed codebases and that your search terms are relevant.

3. **Filter not working**: Verify that the filter values match your indexed data (e.g., correct language names, author names, repository URLs).

4. **Connection issues**: Ensure the Kodit MCP server is running (`kodit serve`) and accessible to your AI assistant.

### Debugging

Enable debug logging to see what's happening:

```sh
export LOG_LEVEL=DEBUG
kodit serve
```

This will show you:

- Search queries being executed
- Filter parameters being applied
- Results being returned
- Any errors or issues
