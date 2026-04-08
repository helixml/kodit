---
title: "Kodit: Code Understanding MCP Server"
linkTitle: Kodit Docs
cascade:
  type: docs
menu:
  main:
    name: Kodit Docs
    weight: 3
next: /kodit/getting-started
weight: 1
aliases:
- /coda
---

<p align="center">
    <a href="https://docs.helix.ml/kodit/"><img src="https://docs.helix.ml/images/helix-kodit-logo.png" alt="Helix Kodit Logo" width="300"></a>
</p>

<p align="center">
Kodit indexes Git repositories and connects your AI coding assistant to accurate, up-to-date code and documentation.
</p>

<div class="flex justify-center items-center gap-4">

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge)](https://github.com/helixml/kodit/blob/main/LICENSE)
[![Discussions](https://img.shields.io/badge/Discussions-181717?style=for-the-badge&logo=github&logoColor=white)](https://github.com/helixml/kodit/discussions)

</div>

**Helix Kodit** is a code understanding platform that indexes Git repositories and
provides hybrid search with LLM-powered enrichments via MCP and a REST API. It can:

- Index public and private Git repositories
- Generate AI-powered documentation (architecture docs, API docs, database schemas, cookbooks, wikis)
- Search using BM25 keyword, semantic similarity, and hybrid search
- Expose code intelligence via MCP for any AI coding assistant
- Browse repository files with grep, ls, and raw file access
- Integrate with any OpenAI-compatible API or run fully local

If you're an engineer working with AI-powered coding assistants, Kodit helps by providing
relevant and up-to-date examples and documentation so that LLMs make fewer mistakes and
produce fewer hallucinations.

## Features

### Codebase Indexing

Kodit clones Git repositories and runs a multi-stage indexing pipeline:

- Clone and scan Git metadata (commits, branches, tags)
- Chunk source files into fixed-size, overlapping text segments
- Build search indexes for BM25 keyword search, code embeddings, and text embeddings
- AI enrichment (with an LLM provider): architecture docs, API docs, database schemas, cookbook entries, commit summaries, and a full wiki
- Support for 20+ programming languages
- Efficient incremental indexing (only processes new/modified commits)
- Automatic periodic sync to keep indexes up-to-date
- Configurable pipeline: full (with LLM enrichments) or RAG-only (lightweight search)

### MCP Server

Kodit exposes 14 MCP tools to AI coding assistants including search, documentation
retrieval, file browsing, and wiki access. Tested and verified with:

- [Cursor](./reference/mcp/index.md)
- [Cline](./reference/mcp/index.md)
- [Claude Code](./reference/mcp/index.md)
- [Kilo Code](./reference/mcp/index.md)
- Any MCP-compatible assistant

### Go SDK

Kodit can be embedded directly into Go applications as a library, or accessed via a
generated HTTP client. See the [Go SDK reference](./reference/go-sdk/index.md).

### Enterprise Ready

Out of the box, Kodit works with a local SQLite database and a built-in CPU-only embedding
model. Enterprises can scale out with performant databases and dedicated models. Everything
can run securely and privately with on-premise LLM platforms like
[Helix](https://helix.ml).

Supported databases:

- SQLite (with FTS5 for BM25)
- [VectorChord](https://github.com/tensorchord/VectorChord) (PostgreSQL extension)

Supported providers:

- Built-in local embedding model (CPU-only)
- OpenAI
- Anthropic (text generation only; requires a separate embedding provider)
- Any OpenAI-compatible API (via LiteLLM)
- Secure, private LLM enclave with [Helix](https://helix.ml)

Deployment options:

- Docker and Docker Compose
- Kubernetes manifests
- Pre-built binaries for Linux and macOS

## Roadmap

The roadmap is currently maintained as a [Github Project](https://github.com/orgs/helixml/projects/4).

## Support

For commercial support, please contact [Helix.ML](https://docs.helixml.tech/helix/help/). To ask a question,
please [open a discussion](https://github.com/helixml/kodit/discussions).

## License

[Apache 2.0 © 2026 HelixML, Inc.](https://github.com/helixml/kodit/blob/main/LICENSE)
