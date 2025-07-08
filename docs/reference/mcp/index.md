---
title: MCP
description: Model Context Protocol (MCP) server implementation for AI coding assistants
weight: 2
---

Kodit provides an MCP (Model Context Protocol) server that enables AI coding assistants to search and retrieve relevant code snippets from your indexed codebases. This allows AI assistants to provide more accurate and contextually relevant code suggestions.

## What is MCP?

The Model Context Protocol (MCP) is a standard that enables AI assistants to communicate with external tools and data sources. Kodit implements an MCP server that exposes your indexed codebases to AI assistants, allowing them to:

- Search for relevant code examples
- Retrieve specific code snippets
- Filter results by various criteria
- Provide context-aware code suggestions

## How Kodit MCP Works

The Kodit MCP server runs as a separate service that:

1. **Connects to your indexed codebases** - Uses the same database and indexes created by the `kodit index` command
2. **Exposes search functionality** - Provides a `search` tool that AI assistants can call
3. **Handles filtering** - Supports filtering by language, author, date range, and source repository
4. **Returns relevant snippets** - Combines keyword, semantic code, and semantic text search for optimal results

## Integration with AI Assistants

To use Kodit with your AI coding assistant, you need to:

1. **Start the Kodit MCP server**:

   ```sh
   kodit serve
   ```

2. **Configure your AI assistant** to connect to the MCP server. See the [Integration Guide](../../getting-started/integration/index.md) for detailed instructions for:
   - Cursor
   - Cline
   - Other MCP-compatible assistants

## Search Tool

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
