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

Enrichment is typically the slowest part of the indexing process because it requires
calling a remote LLM provider. Ideally you want to maximise the number of parallel tasks
but all services have rate limits. Start low and increase over time.

See the [configuration reference](/kodit/reference/configuration/index.md) for
full details. The following is a selection of examples.

### Helix.ml Enrichment Endpoint

Get your free API key from [Helix.ml](https://app.helix.ml/account).

```sh
ENRICHMENT_ENDPOINT_BASE_URL=https://app.helix.ml/v1
ENRICHMENT_ENDPOINT_MODEL=hosted_vllm/Qwen/Qwen3-8B
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=1
ENRICHMENT_ENDPOINT_TIMEOUT=300
ENRICHMENT_ENDPOINT_API_KEY=hl-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

### Local Ollama Enrichment Endpoint

```sh
ENRICHMENT_ENDPOINT_BASE_URL=http://localhost:11434
ENRICHMENT_ENDPOINT_MODEL=ollama_chat/qwen3:1.7b
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=1
ENRICHMENT_ENDPOINT_EXTRA_PARAMS='{"think": false}'
ENRICHMENT_ENDPOINT_TIMEOUT=300
```

### Azure OpenAI Enrichment Endpoint

```sh
ENRICHMENT_ENDPOINT_BASE_URL=https://winderai-openai-test.openai.azure.com/
ENRICHMENT_ENDPOINT_MODEL=azure/gpt-4.1-nano # Must be in the format "azure/azure_deployment_name"
ENRICHMENT_ENDPOINT_API_KEY=XXXX
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=5 # Azure defaults to 100K TPM
ENRICHMENT_ENDPOINT_EXTRA_PARAMS={"api_version": "2024-12-01-preview"}
```
