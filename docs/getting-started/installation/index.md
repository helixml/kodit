---
title: Installing Kodit
description: How to install Kodit.
weight: 1
---

Kodit is a Go binary that hosts a REST API and an MCP server. Most people deploy Kodit
remotely in a container, but you can also run it locally.

## Docker

```sh
docker run -it --rm -p 8080:8080 registry.helix.ml/helix/kodit:latest
```

Always replace `latest` with a specific version.

## Pre-built Binaries

Pre-built binaries for Linux and macOS are available on the
[GitHub releases page](https://github.com/helixml/kodit/releases). Download the
appropriate binary for your platform, make it executable, and run it:

```sh
chmod +x kodit
./kodit serve
```

## Embedding Model

Kodit includes a built-in CPU-only embedding model for semantic search, so basic keyword
and semantic search works out of the box with no external dependencies.

## Enrichment Model

To unlock AI-powered enrichments (architecture docs, API docs, database schemas, cookbook
entries, commit summaries, and wiki generation), you need to configure an enrichment
endpoint that points to an LLM provider.

Without an enrichment model, Kodit will index and search code but will not generate any AI
documentation or summaries.

Set the following environment variables to configure an enrichment provider:

```sh
ENRICHMENT_ENDPOINT_BASE_URL=https://app.helix.ml/v1
ENRICHMENT_ENDPOINT_MODEL=Qwen/Qwen3-8B
ENRICHMENT_ENDPOINT_API_KEY=your-api-key
```

See the [configuration reference](../../reference/configuration/index.md) for more
provider examples including Ollama, Azure OpenAI, and other LiteLLM-compatible services.

## Pipeline Presets

Kodit supports two pipeline presets that control which operations run during indexing:

- **Full pipeline** (default when an enrichment endpoint is configured): runs all operations
  including LLM-powered enrichments (architecture docs, API docs, database schemas,
  cookbooks, commit summaries, wiki generation)
- **RAG-only pipeline**: runs snippet extraction, BM25 indexing, code embeddings, and
  AST-based API docs. Skips all LLM enrichments. Useful when you only need search
  without AI documentation.

When no enrichment endpoint is configured, Kodit automatically runs a reduced pipeline
equivalent to RAG-only.

## Next Steps

See the [deployment guide](../../reference/deployment/index.md) for Docker Compose and
Kubernetes deployment instructions, or jump straight to the
[quick start](../quick-start/index.md) to index your first repository.
