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

### Indexing a Source

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

### Supported File Types

Kodit automatically detects and processes files based on their extensions:

| Language | Extensions |
|----------|------------|
| Python | `.py` |
| JavaScript | `.js`, `.jsx` |
| TypeScript | `.ts`, `.tsx` |
| Go | `.go` |
| C# | `.cs` |

### Snippet Extraction

Kodit uses tree-sitter to intelligently extract code snippets:

- **Functions and Methods**: Complete function definitions with their bodies
- **Classes**: Class definitions and their methods
- **Imports**: Import statements for context
- **Dependencies**: Ancestor classes and functions that the snippet depends on

## Configuration

### Clone Directory

By default, Kodit stores cloned repositories in `~/.kodit/clones/`. You can configure this using the `DATA_DIR` environment variable:

```sh
export DATA_DIR=/custom/path/to/kodit/data
```

### Database Configuration

Kodit uses SQLite by default, but supports PostgreSQL with VectorChord for better performance:

```sh
# SQLite (default)
DB_URL=sqlite+aiosqlite:///path/to/kodit.db

# PostgreSQL with VectorChord
DB_URL=postgresql+asyncpg://user:password@localhost:5432/kodit
DEFAULT_SEARCH_PROVIDER=vectorchord
```

### AI Provider Configuration

For semantic search and enrichment, configure your AI provider:

```sh
# OpenAI
DEFAULT_ENDPOINT_TYPE=openai
DEFAULT_ENDPOINT_BASE_URL=https://api.openai.com/v1
DEFAULT_ENDPOINT_API_KEY=sk-your-api-key

# Or use local models (slower but private)
# No configuration needed - uses local models by default
```

## Advanced Features

### Re-indexing Sources

Future feature!

### Progress Monitoring

Kodit shows progress during indexing operations:

- File processing progress
- Snippet extraction progress
- Index building progress (BM25, embeddings)

### Error Handling

Common issues and solutions:

1. **Authentication Errors**: Ensure your credentials are correct
2. **Network Issues**: Check your internet connection and firewall settings
3. **Permission Errors**: Ensure you have read access to the source
4. **Unsupported Files**: Kodit will skip unsupported file types automatically

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

### Checking Index Status

To verify your indexes are working correctly:

```sh
# List all indexes
kodit index

# Test search functionality
kodit search text "example function"
```
