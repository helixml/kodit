<p align="center">
    <a href="https://docs.helix.ml/kodit/"><img src="https://docs.helix.ml/images/helix-kodit-logo.png" alt="Helix Kodit Logo" width="300"></a>
</p>

<h1 align="center">
Kodit: Code Understanding MCP Server
</h1>

<p align="center">
Kodit indexes Git repositories and connects your AI coding assistant to accurate, up-to-date code and documentation.
</p>

<div align="center">

[![Documentation](https://img.shields.io/badge/Documentation-6B46C1?style=for-the-badge&logo=readthedocs&logoColor=white)](https://docs.helix.ml/kodit/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge)](./LICENSE)
[![Discussions](https://img.shields.io/badge/Discussions-181717?style=for-the-badge&logo=github&logoColor=white)](https://github.com/helixml/kodit/discussions)

</div>

:star: _Help us reach more developers and grow the Helix community. Star this repo!_

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

- **Clone and scan** Git metadata (commits, branches, tags)
- **Chunk source files** into fixed-size, overlapping text segments
- **Build search indexes** for BM25 keyword search, code embeddings, and text embeddings
- **AI enrichment** (with an LLM provider): architecture docs, API docs, database schemas, cookbook entries, commit summaries, and a full wiki
- Support for 20+ programming languages including Python, JavaScript/TypeScript, Java, Go, Rust, C/C++, C#, HTML/CSS, and more
- Efficient incremental indexing (only processes new/modified commits)
- Automatic periodic sync to keep indexes up-to-date
- Configurable pipeline presets: full pipeline with LLM enrichments, or RAG-only for lightweight search

### MCP Server

Kodit exposes 14 MCP tools to AI coding assistants:

| Tool | Description |
|------|-------------|
| `kodit_repositories` | List all indexed repositories |
| `kodit_version` | Get the server version |
| `kodit_architecture_docs` | High-level architecture documentation |
| `kodit_api_docs` | API/interface documentation |
| `kodit_database_schema` | Database schema documentation |
| `kodit_cookbook` | Usage examples and cookbook entries |
| `kodit_commit_description` | Commit description and summary |
| `kodit_wiki` | Table of contents for a repository's wiki |
| `kodit_wiki_page` | Content of a specific wiki page |
| `kodit_semantic_search` | Semantic similarity search with file URIs |
| `kodit_keyword_search` | BM25 keyword search with file URIs |
| `kodit_grep` | Regex search via git grep |
| `kodit_ls` | List files matching a glob pattern |
| `kodit_read_resource` | Read file contents from a resource URI |

Tested and verified with [Cursor](https://docs.helix.ml/kodit/reference/mcp/),
[Cline](https://docs.helix.ml/kodit/reference/mcp/),
[Claude Code](https://docs.helix.ml/kodit/reference/mcp/), and
[Kilo Code](https://docs.helix.ml/kodit/reference/mcp/).
Any MCP-compatible assistant should work.

### Go SDK

Kodit can be embedded directly into Go applications as a library:

```go
client, err := kodit.New(
    kodit.WithSQLite("/path/to/data.db"),
    kodit.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
)
defer client.Close()

// Index a repository
repo, _, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{
    URL: "https://github.com/example/repo",
})

// Search
results, err := client.Search.Query(ctx, "create a deployment",
    service.WithSemanticWeight(0.7),
    service.WithLimit(10),
)
```

A generated HTTP client (`clients/go`) is also available for calling a remote Kodit server.
See the [Go SDK reference](https://docs.helix.ml/kodit/reference/go-sdk/) for details.

### Enterprise Ready

Out of the box, Kodit works with a local SQLite database and a built-in CPU-only embedding
model. Enterprises can scale out with performant databases and dedicated models. Everything
can run securely and privately with on-premise LLM platforms like [Helix](https://helix.ml).

Supported databases:

- SQLite (with FTS5 for BM25)
- [VectorChord](https://github.com/tensorchord/VectorChord) (PostgreSQL extension for BM25 and vector search)

Supported providers:

- Built-in local embedding model (CPU-only, no external dependencies)
- OpenAI
- Anthropic (text generation only; requires a separate embedding provider)
- Any OpenAI-compatible API (via LiteLLM)
- Secure, private LLM enclave with [Helix](https://helix.ml)

Deployment options:

- Docker and Docker Compose
- Kubernetes manifests
- Pre-built binaries for Linux and macOS

## Quick Start

1. [Install Kodit](https://docs.helix.ml/kodit/getting-started/installation/)
2. [Index codebases](https://docs.helix.ml/kodit/getting-started/quick-start/)
3. [Integrate with your coding assistant](https://docs.helix.ml/kodit/getting-started/integration/)

### Documentation

- [Getting Started Guide](https://docs.helix.ml/kodit/getting-started/)
- [Reference Guide](https://docs.helix.ml/kodit/reference/)
- [Demos](https://docs.helix.ml/kodit/demos/)
- [Contribution Guidelines](.github/CONTRIBUTING.md)

## Roadmap

The roadmap is currently maintained as a [Github Project](https://github.com/orgs/helixml/projects/4).

## Support

For commercial support, please contact [Helix.ML](founders@helix.ml). To ask a question,
please [open a discussion](https://github.com/helixml/kodit/discussions).

## License

[Apache 2.0 © 2026 HelixML, Inc.](./LICENSE)
