---
title: Indexing
description: Learn how to index Git repositories in Kodit for AI-powered code search and generation.
weight: 1
---

Kodit indexes Git repositories to create searchable code databases for AI assistants. The system extracts code snippets with semantic understanding and builds multiple search indexes for different query types.

## How Indexing Works

Kodit transforms Git repositories through a 5-stage pipeline:

1. **Clone Repository**: Downloads the Git repository locally
2. **Scan Repository**: Extracts Git metadata (commits, branches, tags)
3. **Extract Snippets**: Uses Tree-sitter parsing to extract functions, classes, and methods with dependencies
4. **Build Indexes**: Creates BM25 (keyword), code embeddings (semantic), and text embeddings (natural language) indexes
5. **AI Enrichment**: Generates summaries using LLM providers for enhanced search

### Supported Sources

Kodit indexes Git repositories via:

- **HTTPS**: Public and private repositories with authentication
- **SSH**: Using SSH keys
- **Git Protocol**: For public repositories

Supports GitHub, GitLab, Bitbucket, Azure DevOps, and self-hosted Git servers.

## REST API

Kodit provides a REST API that allows you to programmatically manage indexes and search
code snippets. The API is automatically available when you start the Kodit server and
follows the JSON:API specification for consistent request/response formats.

Please see the [API documentation](../api/index.md) for a full description of the API. You can also
browse to the live API documentation by visiting `/docs`.

## Authentication

```sh
# HTTPS with token
https://username:token@github.com/username/repo.git

# SSH (ensure SSH key is configured)
git@github.com:username/repo.git
```

## Supported Languages

20+ programming languages with Tree-sitter parsing:

| Language | Extensions | Key Features |
|----------|------------|-------------|
| Python | `.py`, `.pyw`, `.pyx` | Decorators, async functions, inheritance |
| JavaScript/TypeScript | `.js`, `.jsx`, `.ts`, `.tsx` | Arrow functions, ES6 modules, types |
| Java | `.java` | Annotations, generics, inheritance |
| Go | `.go` | Interfaces, struct methods, packages |
| Rust | `.rs` | Traits, ownership patterns, macros |
| C/C++ | `.c`, `.h`, `.cpp`, `.hpp` | Function pointers, templates |
| C# | `.cs` | Properties, LINQ, async patterns |
| HTML/CSS | `.html`, `.css`, `.scss` | Semantic elements, responsive patterns |

## Advanced Features

### Intelligent Re-indexing

- Git commit tracking for change detection
- Only processes new/modified commits
- Bulk operations for performance
- Concurrent snippet extraction

### Task Queue System

- User-initiated (high priority)
- Background sync (low priority)
- Repository and commit operations
- Automatic retry with backoff

### Auto-Sync Server

- 30-minute default sync intervals
- Background processing
- Incremental updates only

## Configuration

Please see the [configuration reference](/kodit/reference/configuration/index.md) for
full details and examples.
