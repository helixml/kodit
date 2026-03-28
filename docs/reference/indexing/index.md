---
title: Indexing
description: Learn how to index Git repositories in Kodit for AI-powered code search and generation.
weight: 1
---

Kodit indexes Git repositories to create searchable code databases for AI assistants. The
system splits source files into fixed-size text chunks, builds multiple search indexes,
and optionally generates AI-powered documentation.

## How Indexing Works

Kodit transforms Git repositories through a multi-stage pipeline:

1. **Clone Repository**: Downloads the Git repository locally
2. **Scan Repository**: Extracts Git metadata (commits, branches, tags)
3. **Chunk Files**: Splits source files into fixed-size, overlapping text chunks
4. **Build Search Indexes**: Creates BM25 (keyword), code embeddings (semantic), and text embeddings (natural language) indexes
5. **AI Enrichment** (requires an enrichment endpoint): Generates documentation using an LLM provider

### Enrichment Types

When an enrichment endpoint is configured, Kodit generates the following for each commit:

| Type | Subtype | Description |
|------|---------|-------------|
| Architecture | Physical | High-level architecture documentation |
| Architecture | Database Schema | Database schema documentation |
| Development | Snippet | Code snippets with context |
| Development | Snippet Summary | AI-generated summaries of code snippets |
| Development | Example | Usage examples extracted from code |
| Development | Chunk | Raw text chunks |
| History | Commit Description | AI-generated commit summaries |
| Usage | API Docs | API and interface documentation (AST-based, no LLM required) |
| Usage | Cookbook | Cookbook entries with usage examples |
| Usage | Wiki | Structured wiki pages covering architecture, APIs, data models, and more |

### Pipeline Presets

Kodit supports two pipeline presets:

- **Full pipeline** (default when an enrichment endpoint is configured): runs all
  operations including LLM-powered enrichments
- **RAG-only pipeline**: runs snippet extraction, BM25 indexing, code embeddings, and
  AST-based API docs. Skips LLM enrichments. Useful for lightweight search without AI
  documentation generation.

When no enrichment endpoint is configured, Kodit automatically disables LLM-dependent
operations.

For the Go SDK, use `kodit.WithRAGPipeline()` or `kodit.WithFullPipeline()` to select a
preset explicitly.

### Supported Sources

Kodit indexes Git repositories via:

- **HTTPS**: Public and private repositories with authentication
- **SSH**: Using SSH keys
- **Git Protocol**: For public repositories

Supports GitHub, GitLab, Bitbucket, Azure DevOps, and self-hosted Git servers.

## REST API

Kodit provides a REST API that allows you to programmatically manage repositories and
search code. The API is automatically available when you start the Kodit server and
follows the JSON:API specification for consistent request/response formats.

Please see the [API documentation](../api/index.md) for a full description. You can also
browse the live API documentation by visiting `/docs`.

## Authentication

```sh
# HTTPS with token
https://username:token@github.com/username/repo.git

# SSH (ensure SSH key is configured)
git@github.com:username/repo.git
```

## Supported Languages

20+ programming languages including Python, JavaScript/TypeScript, Java, Go, Rust, C/C++,
C#, HTML/CSS, and more.

## Advanced Features

### Intelligent Re-indexing

- Git commit tracking for change detection
- Only processes new/modified commits
- Bulk operations for performance
- Concurrent file chunking

### Branch Tracking

Each repository has a configurable tracking configuration that controls which branch is
indexed. By default, Kodit tracks the repository's default branch. You can update the
tracking configuration via the API:

```sh
curl http://localhost:8080/api/v1/repositories/1/tracking-config \
-X PUT \
-H "Content-Type: application/json" \
-d '{
  "data": {
    "type": "tracking_config",
    "attributes": {
      "mode": "branch",
      "value": "develop"
    }
  }
}'
```

### Task Queue System

- User-initiated operations (high priority)
- Background sync (low priority)
- Repository and commit operations
- Automatic retry with backoff

### Auto-Sync

- 30-minute default sync intervals (configurable)
- Background processing
- Incremental updates only
- See the [sync reference](../sync/index.md) for details

### Wiki Generation

When an enrichment endpoint is configured, Kodit generates a structured wiki for each
repository. The wiki provides hierarchical documentation pages and can be browsed via the
API or through MCP tools.

```sh
# View wiki table of contents
curl http://localhost:8080/api/v1/repositories/1/wiki

# View a specific wiki page
curl http://localhost:8080/api/v1/repositories/1/wiki/architecture/overview.md

# Regenerate the wiki
curl -X POST http://localhost:8080/api/v1/repositories/1/wiki/rescan
```

### File Browsing

Kodit provides direct access to repository files without needing a local clone:

```sh
# Read a file at a specific commit/branch/tag
curl "http://localhost:8080/api/v1/repositories/1/blob/main/src/main.go?line_numbers=true"

# Read specific lines
curl "http://localhost:8080/api/v1/repositories/1/blob/main/src/main.go?lines=L17-L26"

# Grep for patterns
curl "http://localhost:8080/api/v1/search/grep?repository_id=1&pattern=func.*Handler"

# List files
curl "http://localhost:8080/api/v1/search/ls?repository_id=1&pattern=**/*.go"
```

## Configuration

Please see the [configuration reference](../configuration/index.md) for full details
and examples.
