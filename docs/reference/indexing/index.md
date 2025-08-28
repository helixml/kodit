---
title: Indexing
description: Learn how to index code sources in Kodit for AI-powered code search and generation.
weight: 1
---

Kodit's indexing system allows you to create searchable indexes of your codebases, enabling AI assistants to find and reference relevant code snippets. This page explains how indexing works, what sources are supported, and how to use the indexing features.

## How Indexing Works

Kodit's indexing process consists of several stages:

1. **Source Creation**: Kodit clones or copies your source code to a local working directory
2. **File Processing**: Files are scanned and metadata is extracted (timestamps, authors, etc.)
3. **Snippet Extraction**: Code is parsed using tree-sitter to extract meaningful snippets (functions, classes, methods)
4. **Index Building**: Multiple search indexes are created:
   - **BM25 Index**: For keyword-based search
   - **Semantic Code Index**: For code similarity search using embeddings
5. **Enrichment**: AI-powered enrichment of snippets for better search results
   - **Semantic Text Index**: For natural language search using embeddings

### Supported Source Types

Kodit supports two main types of sources:

#### Git Repositories

Kodit can index any Git repository accessible via standard Git protocols:

- **HTTPS**: Public repositories and private repositories with authentication
- **SSH**: Using SSH keys for authentication
- **Git Protocol**: For public repositories

#### Local Directories

Kodit can index local directories on your filesystem:

- **Absolute paths**: `/path/to/your/code`
- **Relative paths**: `./my-project`
- **Home directory expansion**: `~/projects/my-app`
- **File URIs**: `file:///path/to/your/code`

## Basic Usage

### Manual Indexing

To index a source, use the `kodit index` command followed by the source location:

```sh
# Index a local directory
kodit index /path/to/your/code

# Index a public Git repository
kodit index https://github.com/pydantic/pydantic

# Index a private Git repository (requires authentication)
kodit index https://github.com/username/private-repo

# Index using SSH
kodit index git@github.com:username/repo.git
```

### Listing Indexes

To see all your indexed sources:

```sh
kodit index
```

This will display a table showing:

- Index ID
- Creation and update timestamps
- Source URI
- Number of snippets extracted

## REST API

Kodit provides a REST API that allows you to programmatically manage indexes and search
code snippets. The API is automatically available when you start the Kodit server and
follows the JSON:API specification for consistent request/response formats.

Please see the [API documentation](../api/index.md) for a full description of the API. You can also
browse to the live API documentation by visiting `/docs`.

### Index Management

#### List All Indexes

Retrieve a list of all indexes with their metadata:

```sh
curl -H "x-api-key: your-api-key" \
     http://localhost:8000/api/v1/indexes
```

#### Create a New Index

Create a new index by providing the source URI. The indexing process starts asynchronously:

```sh
curl -X POST \
     -H "x-api-key: your-api-key" \
     -H "Content-Type: application/json" \
     -d '{
       "data": {
         "type": "index",
         "attributes": {
           "uri": "https://github.com/fastapi/fastapi"
         }
       }
     }' \
     http://localhost:8000/api/v1/indexes
```

#### Delete an Index

Remove an index and all its associated data:

```sh
curl -X DELETE \
     -H "x-api-key: your-api-key" \
     http://localhost:8000/api/v1/indexes/1
```

### Code Search

#### Search Code Snippets

Search through indexed code snippets using various query types and filters:

```sh
curl -X POST \
     -H "x-api-key: your-api-key" \
     -H "Content-Type: application/json" \
     -d '{
       "data": {
         "type": "search",
         "attributes": {
           "text": "async function to handle user authentication",
           "limit": 10,
           "filters": {
             "languages": ["javascript", "typescript"],
             "sources": ["https://github.com/fastapi/fastapi"]
           }
         }
       }
     }' \
     http://localhost:8000/api/v1/search
```

## Git Protocol Support

### HTTPS Authentication

For private repositories, you can authenticate using:

1. **Personal Access Token** (GitHub, GitLab, etc.):

   ```sh
   kodit index https://username:token@github.com/username/repo.git
   ```

2. **Username/Password** (if supported by your Git provider):

   ```sh
   kodit index https://username:password@github.com/username/repo.git
   ```

### SSH Authentication

For SSH-based repositories:

1. **SSH Key Authentication**:

   ```sh
   kodit index git@github.com:username/repo.git
   ```

   Ensure your SSH key is properly configured in your SSH agent or `~/.ssh/config`.

2. **SSH with Custom Port**:

   ```sh
   kodit index ssh://git@github.com:2222/username/repo.git
   ```

### Git Providers

Kodit works with any Git provider that supports standard Git protocols:

- **GitHub**: `https://github.com/username/repo.git`
- **GitLab**: `https://gitlab.com/username/repo.git`
- **Bitbucket**: `https://bitbucket.org/username/repo.git`
- **Azure DevOps**: `https://dev.azure.com/organization/project/_git/repo`
- **Self-hosted Git servers**: Any Git server supporting HTTP/HTTPS or SSH

## Examples of Use

### Index a Public Azure DevOps Repository

```sh
kodit index https://winderai@dev.azure.com/winderai/public-test/_git/simple-ddd-brewing-demo
```

### Indexing a Private Azure DevOps Repository

If you're accessing Azure DevOps from your local machine and have the Git credential
helper you should be able to clone the repository as usual (obviously you won't be able
to clone this because it is private):

```sh
kodit index https://winderai@dev.azure.com/winderai/private-test/_git/private-test
```

You can also use a Personal Access Token (PAT):

```sh
kodit index https://phil:xxxxxxSECRET_PATxxxxxxx@dev.azure.com/winderai/private-test/_git/private-test
```

## File Processing and Filtering

### Ignored Files

Kodit respects [standard ignore patterns](#ignore-patterns):

- **`.gitignore`**: Standard Git ignore patterns
- **`.noindex`**: Custom ignore patterns for Kodit (uses gitignore syntax)

### Supported Programming Languages

Kodit automatically detects and processes files based on their extensions. The following languages are supported with advanced Tree-sitter parsing:

| Language | Extensions | Features |
|----------|------------|----------|
| Python | `.py`, `.pyw`, `.pyx`, `.pxd` | Function/method extraction, import analysis, call graph |
| JavaScript | `.js`, `.jsx`, `.mjs` | Function extraction, ES6 modules, JSX support |
| TypeScript | `.ts`, `.tsx` | Type definitions, interfaces, decorators |
| Java | `.java` | Method declarations, constructors, class hierarchies |
| Go | `.go` | Function/method extraction, package imports |
| Rust | `.rs` | Function definitions, trait implementations |
| C/C++ | `.c`, `.h`, `.cpp`, `.cc`, `.cxx`, `.hpp`, `.hxx` | Function definitions, header includes |
| C# | `.cs` | Method declarations, using directives, constructors |
| HTML | `.html`, `.htm` | Element extraction with ID/class identification |
| CSS | `.css`, `.scss`, `.sass`, `.less` | Rule extraction, selector analysis, keyframes |

### Advanced Snippet Extraction

Kodit uses a sophisticated Tree-sitter-based slicing system to intelligently extract code snippets with context:

#### Core Features

- **Functions and Methods**: Complete function definitions with their bodies
- **Classes**: Class definitions and their methods
- **Imports**: Import statements for context
- **Dependencies**: Ancestor classes and functions that the snippet depends on
- **Call Graph Analysis**: Builds relationships between functions to understand dependencies
- **Context-Aware Extraction**: Includes related functions and usage examples
- **Topological Sorting**: Orders dependencies for optimal LLM consumption

#### Smart Dependency Tracking

- **Import Maps**: Tracks import statements and their usage
- **Function Calls**: Identifies which functions call which others
- **Reverse Dependencies**: Finds all callers of a given function
- **Usage Examples**: Includes examples of how functions are used in the codebase

#### Language-Specific Extraction

- **Python**: Decorators, async functions, class inheritance
- **JavaScript/TypeScript**: Arrow functions, async/await, ES6 modules
- **Java**: Annotations, generics, inheritance hierarchies
- **Go**: Interfaces, struct methods, package organization
- **HTML/CSS**: Elements with semantic context, CSS rules and selectors

## Advanced Features

### Selective Re-indexing

Kodit includes intelligent re-indexing that only processes files that have been modified:

#### How It Works

- **SHA256 Change Detection**: Compares file content hashes to detect changes
- **File Status Tracking**: Tracks files as CLEAN, MODIFIED, or DELETED
- **Incremental Updates**: Only re-processes changed files, improving performance for large codebases
- **Metadata Preservation**: Maintains file metadata and Git information

#### Benefits

- **Performance**: Dramatically faster re-indexing for large repositories
- **Resource Efficiency**: Reduces CPU and memory usage during updates
- **Consistency**: Ensures only actual changes trigger re-processing
- **Scalability**: Enables efficient handling of large, frequently-updated codebases

#### Usage

Re-indexing automatically uses selective processing when you re-index an existing source:

```sh
# Re-index with selective processing
kodit index /path/to/existing/source

# Or for Git repositories
kodit index https://github.com/username/repo.git
```

### Progress Monitoring

Kodit shows progress during indexing operations:

- File processing progress
- Snippet extraction progress
- Index building progress (BM25, embeddings)

### Automatic Sync

For server deployments, Kodit includes an automatic sync scheduler that keeps your indexes up-to-date:

- **Periodic sync**: Automatically re-indexes all existing sources at configurable intervals
- **Failure handling**: Gracefully handles sync failures with retry logic
- **Background operation**: Runs in the background without blocking the MCP server
- **Configurable timing**: Adjust sync frequency based on your needs

See the [Sync Configuration](/kodit/reference/sync/index.md) documentation for detailed setup instructions.

The sync scheduler is enabled by default and will:

- Start automatically when you run `kodit serve`
- Sync all existing indexes every 30 minutes by default
- Log detailed progress and results
- Retry failed operations up to 3 times by default

## Privacy and Security

### Local Processing

- All code is processed locally by default
- No code is sent to external services unless you configure AI providers
- Cloned repositories are stored locally in your data directory

### Ignore Patterns

Kodit respects privacy by honoring:

- `.gitignore` patterns
- `.noindex` files for custom exclusions
- Hidden files and directories (starting with `.`)

### Authentication

- SSH keys and tokens are handled by your system's Git configuration
- Kodit doesn't store or transmit credentials
- Use environment variables for sensitive configuration

## Troubleshooting

### Common Issues

1. **"Failed to clone repository"**: Check your Git credentials and network connection
2. **"Unsupported source"**: Ensure the path or URL is valid and accessible
3. **"No snippets found"**: Check if the source contains supported file types
4. **"Permission denied"**: Ensure you have read access to the source
5. **"No auto-index sources configured"**: Check your environment variables are set correctly

### Checking Index Status

To verify your indexes are working correctly:

```sh
# List all indexes
kodit index

# Test search functionality
kodit search text "example function"
```
