---
title: MCP
description: Model Context Protocol (MCP) server implementation for AI coding assistants
weight: 2
---

The [Model Context Protocol](https://modelcontextprotocol.io/introduction) (MCP) is a
standard that enables AI assistants to communicate with external tools and data sources.

Kodit provides an MCP (Model Context Protocol) server that enables AI coding assistants
to search and retrieve relevant code snippets from your indexed codebases. This allows
AI assistants to provide more accurate and contextually relevant code suggestions.

## MCP Server Connection Methods

Kodit supports three different ways to run the MCP server, depending on your integration
needs. Each method exposes the same code search capabilities, but differs in how the
connection is established and which assistants/tools it is compatible with.

### HTTP Streaming (Recommended)

This is the default and recommended method for most users. Kodit runs an HTTP server
that streams responses to connected AI coding assistants over the `/mcp` endpoint.

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

### STDIO

Kodit can run as an MCP server over standard input/output (STDIO) for direct integration
with local AI coding assistants that support MCP stdio transport. No network port is
opened.

- **How it works:** Kodit communicates with your AI assistant via standard input and
  output streams, making it ideal for local, low-latency, and networkless setups.
- **When to use:** Use this mode if your coding assistant supports MCP stdio (for
  example, some local LLM tools or advanced IDE integrations), or if you want to avoid
  network configuration entirely.
- **How to start:**

  Configure your AI assistant to run the following command:
  
  ```sh
  kodit stdio
  ```

### SSE (Server-Sent Events) [Deprecated]

Kodit also supports the older SSE protocol on the `/sse` endpoint. This is provided for
backward compatibility with tools that require SSE.

- **How it works:** Kodit starts a local web server and streams results using the SSE
  protocol, which is less efficient and less widely supported than HTTP streaming.
- **When to use:** Only if your assistant specifically requires SSE and does not support
  HTTP streaming.
- **How to start:**

  ```sh
  kodit serve
  ```

  The server will listen on `http://localhost:8080/sse`.

## Integration with AI Assistants

You need to connect your AI coding assistant to take advantage of Kodit. The
instructions to do this depend on how you've deployed it. This section provides
comprehensive instructions for all popular coding assistants.

### Integration With Claude Code

#### Claude Code Streaming HTTP Mode (recommended)

```sh
claude mcp add --transport http kodit https://kodit.helix.ml/mcp
```

#### Claude Code STDIO Mode

```sh
claude mcp add kodit -- kodit stdio
```

### Integration With Cursor

#### Cursor Streaming HTTP Mode (recommended)

![Install MCP Server](https://cursor.com/deeplink/mcp-install-light.svg)
{class="h-8 inline-block" href="cursor://anysphere.cursor-deeplink/mcp/install?name=kodit&config=eyJ1cmwiOiJodHRwczovL2tvZGl0LmhlbGl4Lm1sL21jcCJ9"}

Add the following to `$HOME/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "kodit": {
      "url": "https://kodit.helix.ml/mcp"
    }
  }
}
```

- Or find this configuration in `Cursor Settings` -> `MCP`.
- `https://kodit.helix.ml` is the URL of the hosted Kodit instance. You can replace this
 with <http://localhost:8080> if you are running locally.

#### Cursor STDIO

![Install MCP Server](https://cursor.com/deeplink/mcp-install-light.svg)
{class="h-8 inline-block" href="cursor://anysphere.cursor-deeplink/mcp/install?name=kodit&config=eyJjb21tYW5kIjoicGlweCBydW4ga29kaXQgc3RkaW8ifQ%3D%3D"}

Add the following to `$HOME/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "kodit": {
      "command": "kodit stdio"
    }
  }
}
```

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
      "url": "https://kodit.helix.ml/mcp"
    }
  }
}
```

#### Cline STDIO Mode

For STDIO mode, please use:

```json
{
  "mcpServers": {
    "kodit": {
      "autoApprove": [],
      "command": "kodit",
      "args": ["stdio"],
      "disabled": false,
    }
  }
}
```

### Integration With Kilo Code

1. Open Kilo Code from the side menu
2. Click the `MCP Servers` button at the top right of the Cline window (the icon looks
   like a server)
3. Click the `Edit Project/Global MCP` button.

Add the following configuration:

```json
{
  "mcpServers": {
    "kodit": {
      "type": "streamable-http",
      "url": "https://kodit.helix.ml/mcp",
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
⚠️ **ENFORCEMENT:**
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
⚠️ **ENFORCEMENT:**
For *every* user request that involves writing or modifying code (of any language or
domain), the assistant's *first* action **must** be to call the kodit.search MCP tool.
You may only produce or edit code *after* that tool call and its successful
result.
```

Alternatively, you can browse to the Cursor settings and set this prompt globally.

### Forcing Cline to Use Kodit

1. Go to `Settings` -> `API Configuration`
2. At the bottom there is a `Custom Instructions` section.

## Search Tool

<!--Future: move this to a dedicated reference page-->

The primary tool exposed by the Kodit MCP server is the `search` function, which provides comprehensive code search capabilities.

### Search Parameters

The search tool accepts the following parameters:

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `user_intent` | string | Description of what the user wants to achieve | "Create a REST API endpoint for user authentication" |
| `related_file_paths` | list[Path] | Absolute paths to relevant files | `["/path/to/auth.py"]` |
| `related_file_contents` | list[string] | Contents of relevant files | `["def authenticate(): ..."]` |
| `keywords` | list[string] | Relevant keywords for the search | `["authentication", "jwt", "login"]` |
| `language` | string \| None | Filter by programming language (20+ supported) | `"python"`, `"go"`, `"javascript"`, `"html"`, `"css"` |
| `author` | string \| None | Filter by author name | `"john.doe"` |
| `created_after` | string \| None | Filter by creation date (YYYY-MM-DD) | `"2023-01-01"` |
| `created_before` | string \| None | Filter by creation date (YYYY-MM-DD) | `"2023-12-31"` |
| `source_repo` | string \| None | Filter by source repository | `"github.com/example/repo"` |
| `file_path` | string \| None | Filter by file path pattern | `"src/"`, `"*.test.py"` |

### Advanced Search Functionality

The search tool combines multiple search strategies with sophisticated ranking:

1. **BM25 Keyword Search** - Advanced keyword matching with relevance scoring
2. **Semantic Code Search** - Uses embeddings to find semantically similar code patterns
3. **Semantic Text Search** - Uses embeddings to find code matching natural language descriptions
4. **Reciprocal Rank Fusion (RRF)** - Intelligently combines results from multiple search strategies
5. **Context-Aware Filtering** - Advanced filtering by language, author, date, source, and file path
6. **Dependency-Aware Results** - Returns code snippets with their dependencies and usage examples

#### Enhanced Result Quality

- **Smart Snippet Selection**: Returns functions with their dependencies and context
- **Rich Metadata**: Each result includes file path, language, author, and creation date
- **Usage Examples**: Includes examples of how functions are used in the codebase
- **Topological Ordering**: Dependencies are ordered for optimal LLM consumption

## Filtering Capabilities

Kodit's MCP server supports comprehensive filtering to help AI assistants find the most relevant code examples. These filters work the same way as the CLI search filters.

### Language Filtering

Filter results by programming language:

**Example prompts:**
> "I need to create a web server in Python. Please search for Flask or FastAPI examples and show me the best practices."
> "I'm working on a Go microservice. Can you search for Go-specific patterns for handling HTTP requests and database connections?"
> "I need JavaScript examples for form validation. Please search for modern JavaScript/TypeScript validation patterns."
> "I'm building a responsive layout. Please search for CSS Grid and Flexbox examples in our stylesheets."
> "I need HTML form examples. Please search for form elements with proper accessibility attributes."

### Author Filtering

Filter results by code author:

**Example prompts:**
> "I'm reviewing code written by john.doe. Can you search for their authentication implementations to understand their coding style?"
> "I need to find all the database-related code written by alice.smith. Please search for her database connection and query patterns."

### Date Range Filtering

Filter results by creation date:

**Example prompts:**
> "I need to see authentication patterns from 2023. Please search for JWT and OAuth implementations created in 2023."
> "Show me modern React patterns from the last year. Search for React components and hooks created after 2023."

### Source Repository Filtering

Filter results by source repository:

**Example prompts:**
> "I'm working on the auth-service project. Please search for authentication patterns specifically from github.com/company/auth-service."
> "I need to understand how the user-service handles user management. Search for user-related code from github.com/company/user-service."

### Combining Filters

You can combine multiple filters for precise results:

**Example prompts:**
> "I need Python authentication code written by alice.smith in 2023 from the auth-service repository. Please search for JWT token validation patterns."
> "Show me Go microservice patterns from john.doe created in 2023 from the backend-services repository."
> "I'm looking for modern React patterns from the frontend team (search for authors: alice.smith, bob.jones) created in 2024 from the web-app repository."

## AI Assistant Integration Tips

To get the best results from Kodit with your AI assistant, follow these prompting strategies:

### 1. Provide Clear User Intent

When the AI assistant calls the search tool, ensure it provides a clear, descriptive `user_intent`:

**Good examples:**

- "Create a REST API endpoint for user authentication with JWT tokens"
- "Implement a database connection pool for PostgreSQL"
- "Write a function to validate email addresses using regex"

**Poor examples:**

- "Help me with auth"
- "Database stuff"
- "Email validation"

### 2. Use Relevant Keywords

Provide specific, technical keywords that are relevant to your task, where applicable.
Remember that the language model is more than capable of generating appropriate keywords
for your intent.

**Good examples:**

- `["authentication", "jwt", "login", "password"]`
- `["database", "postgresql", "connection", "pool"]`
- `["email", "validation", "regex", "format"]`

### 3. Leverage File Context

If you're working with existing files, mention them in your prompt:

**Example prompts:**
> "I'm working on the authentication function in auth.py. Can you search for similar error handling patterns and show me how to improve the error handling in my existing code?"
> "I have a database connection setup in database.py. Please search for connection pooling patterns and show me how to optimize my current implementation."

### 4. Use Language Filtering

Specify the programming language in your prompt:

**Example prompts:**
> "I need to create a web server in Python. Please search for Flask and FastAPI examples and show me the best practices."
> "I'm building a Go microservice. Can you search for Go-specific patterns for handling HTTP requests and database connections?"
> "I need JavaScript examples for form validation. Please search for modern JavaScript/TypeScript validation patterns."

### 5. Filter by Source Repository

If you have multiple codebases indexed, mention the specific repository:

**Example prompts:**
> "I'm working on the user-service project. Please search for user management patterns specifically from github.com/company/user-service."
> "I need to understand how the auth-service handles authentication. Search for auth-related code from github.com/company/auth-service."

### 6. Example Prompts for AI Assistants

Here are some example prompts you can use with your AI assistant:

**For new code development:**
> "I want to create a new Python web API. Please search for examples of Flask/FastAPI authentication patterns and show me the best practices."

**For debugging existing code:**
> "I'm having issues with this database connection code. Can you search for similar patterns and show me how others handle connection errors?"

**For learning new patterns:**
> "I need to implement JWT authentication in Go. Please search for production-ready examples and show me the security best practices."

**For code review:**
> "I'm reviewing this authentication function. Can you search for similar implementations and show me potential security issues or improvements?"

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
