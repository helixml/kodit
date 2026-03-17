---
title: Indexing
description: Learn how to index Git repositories in Kodit for AI-powered code search and generation.
weight: 1
---

Kodit indexes Git repositories to create searchable code databases for AI assistants. The system splits source files into fixed-size text chunks and builds multiple search indexes for different query types.

## How Indexing Works

Kodit transforms Git repositories through a 5-stage pipeline:

1. **Clone Repository**: Downloads the Git repository locally
2. **Scan Repository**: Extracts Git metadata (commits, branches, tags)
3. **Chunk Files**: Splits source files into fixed-size, overlapping text chunks
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

20+ programming languages including Python, JavaScript/TypeScript, Java, Go, Rust, C/C++, C#, HTML/CSS, and more.

## Advanced Features

### Intelligent Re-indexing

- Git commit tracking for change detection
- Only processes new/modified commits
- Bulk operations for performance
- Concurrent file chunking

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
