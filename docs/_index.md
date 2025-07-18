---
title: "Kodit: Code Indexing MCP Server"
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
Kodit connects your AI coding assistant to external codebases to provide accurate and up-to-date snippets of code.
</p>

<div class="flex justify-center items-center gap-4">

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge)](https://github.com/helixml/kodit/blob/main/LICENSE)
[![Discussions](https://img.shields.io/badge/Discussions-181717?style=for-the-badge&logo=github&logoColor=white)](https://github.com/helixml/kodit/discussions)

</div>

**Helix Kodit** is an **MCP server** that connects your AI coding assistant to external codebases. It can:

- Improve your AI-assisted code by providing canonical examples direct from the source
- Index local and public codebases
- Integrates with any AI coding assistant via MCP
- Search using keyword and semantic search
- Integrate with any OpenAI-compatible or custom API/model

If you're an engineer working with AI-powered coding assistants, Kodit helps by
providing relevant and up-to-date examples of your task so that LLMs make less mistakes
and produce fewer hallucinations.

## Features

### Codebase Indexing

Kodit connects to a variety of local and remote codebases to build an index of your
code. This index is used to build a snippet library, ready for ingestion into an LLM.

- Index local directories and public Git repositories
- Build comprehensive snippet libraries for LLM ingestion
- Support for 20+ programming languages including Python, JavaScript/TypeScript, Java, Go, Rust, C/C++, C#, HTML/CSS, and more
- Advanced code analysis with dependency tracking and call graph generation
- Intelligent snippet extraction with context-aware dependencies
- Efficient indexing with selective reindexing (only processes modified files)
- Privacy first: respects .gitignore and .noindex files
- **NEW in 0.3**: Auto-indexing configuration for shared server deployments
- **NEW in 0.3**: Enhanced Git provider support including Azure DevOps
- **NEW in 0.3**: Index private repositories via a PAT
- **NEW in 0.3**: Improved progress monitoring and reporting during indexing
- **NEW in 0.4**: Advanced code slicing infrastructure with Tree-sitter parsing

### MCP Server

Relevant snippets are exposed to an AI coding assistant via an MCP server. This allows
the assistant to request relevant snippets by providing keywords, code, and semantic
intent. Kodit has been tested to work well with:

- Seamless integration with popular AI coding assistants
- Tested and verified with:
  - [Cursor](./getting-started/integration/index.md#integration-with-cursor)
  - [Cline](./getting-started/integration/index.md#integration-with-cline)
- Please contribute more instructions! ... any other assistant is likely to work ...
- **New in 0.3**: Advanced search filters by source, language, author, date range, and file path
- **New in 0.3**: Hybrid search combining BM25 keyword search with semantic search
- **New in 0.3**: Enhanced MCP tools with rich context parameters and metadata

### Hosted MCP Server

**New in 0.4**: Try Kodit instantly with our hosted MCP server at [https://kodit.helix.ml/mcp](https://kodit.helix.ml/mcp)! No installation required - just add it to your AI coding assistant and start searching popular codebases immediately.

The hosted server provides:

- Pre-indexed popular open source repositories
- Zero configuration - works out of the box
- Same powerful search capabilities as self-hosted Kodit
- Perfect for trying Kodit before setting up your own instance

Read the [hosted Kodit documentation](./reference/hosted-kodit/index.md).

### Enterprise Ready

Out of the box, Kodit works with a local SQLite database and very small, local models.
But enterprises can scale out with performant databases and dedicated models. Everything
can even run securely, privately, with on-premise LLM platforms like
[Helix](https://helix.ml).

Supported databases:

- SQLite
- [Vectorchord](https://github.com/tensorchord/VectorChord)

Supported providers:

- Local (which uses tiny CPU-only open-source models)
- OpenAI
- Secure, private LLM enclave with [Helix](https://helix.ml).
- Any other OpenAI compatible API

**NEW in 0.3**: Enhanced deployment options:

- Docker Compose configurations with VectorChord
- Kubernetes manifests for production deployments

## Roadmap

The roadmap is currently maintained as a [Github Project](https://github.com/orgs/helixml/projects/4).

## 💬 Support

For commercial support, please contact [Helix.ML](https://docs.helixml.tech/helix/help/). To ask a question,
please [open a discussion](https://github.com/helixml/kodit/discussions).

## License

[Apache 2.0 © 2025 HelixML, Inc.](https://github.com/helixml/kodit/blob/main/LICENSE)
