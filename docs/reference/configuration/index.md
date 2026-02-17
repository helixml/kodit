---
title: Configuration Reference
description: Kodit Configuration Reference
weight: 29
---

This document contains the complete configuration reference for Kodit. All configuration is done through environment variables.

## Core Settings

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `HOST` | string | `0.0.0.0` | Server host to bind to |
| `PORT` | int | `8080` | Server port to listen on |
| `DATA_DIR` | string | `~/.kodit` | Data directory for indexes and cloned repos |
| `DB_URL` | string | `sqlite:///{data_dir}/kodit.db` | Database connection URL |
| `LOG_LEVEL` | string | `INFO` | Log level: DEBUG, INFO, WARN, ERROR |
| `LOG_FORMAT` | string | `pretty` | Log format: `pretty` or `json` |
| `DISABLE_TELEMETRY` | bool | `false` | Disable anonymous telemetry |
| `API_KEYS` | string | _(empty)_ | Comma-separated list of valid API keys (e.g. `key1,key2`) |
| `WORKER_COUNT` | int | `1` | Number of background workers |
| `SEARCH_LIMIT` | int | `10` | Default search result limit |

## Embedding Endpoint

Configuration for the embedding AI service used for semantic search.

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `EMBEDDING_ENDPOINT_BASE_URL` | string | _(empty)_ | Base URL for the endpoint (e.g. `https://app.helix.ml/v1`) |
| `EMBEDDING_ENDPOINT_MODEL` | string | _(empty)_ | Model identifier (e.g. `openai/text-embedding-3-small`) |
| `EMBEDDING_ENDPOINT_API_KEY` | string | _(empty)_ | API key for the endpoint |
| `EMBEDDING_ENDPOINT_NUM_PARALLEL_TASKS` | int | `10` | Number of parallel tasks |
| `EMBEDDING_ENDPOINT_SOCKET_PATH` | string | _(empty)_ | Unix socket path for local communication |
| `EMBEDDING_ENDPOINT_TIMEOUT` | float | `60` | Request timeout in seconds |
| `EMBEDDING_ENDPOINT_MAX_RETRIES` | int | `5` | Maximum number of retries |
| `EMBEDDING_ENDPOINT_INITIAL_DELAY` | float | `2.0` | Initial retry delay in seconds |
| `EMBEDDING_ENDPOINT_BACKOFF_FACTOR` | float | `2.0` | Backoff factor for retries |
| `EMBEDDING_ENDPOINT_EXTRA_PARAMS` | string | _(empty)_ | JSON-encoded extra parameters |
| `EMBEDDING_ENDPOINT_MAX_TOKENS` | int | `4000` | Maximum token limit for the embedding model |

If no external embedding endpoint is configured, Kodit uses a built-in local embedding
model (jina-embeddings-v2-base-code via ONNX Runtime).

## Enrichment Endpoint

Configuration for the enrichment AI service used for generating summaries and documentation.

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `ENRICHMENT_ENDPOINT_BASE_URL` | string | _(empty)_ | Base URL for the endpoint (e.g. `https://app.helix.ml/v1`) |
| `ENRICHMENT_ENDPOINT_MODEL` | string | _(empty)_ | Model identifier (e.g. `openai/gpt-4.1-nano`) |
| `ENRICHMENT_ENDPOINT_API_KEY` | string | _(empty)_ | API key for the endpoint |
| `ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS` | int | `10` | Number of parallel tasks |
| `ENRICHMENT_ENDPOINT_SOCKET_PATH` | string | _(empty)_ | Unix socket path for local communication |
| `ENRICHMENT_ENDPOINT_TIMEOUT` | float | `60` | Request timeout in seconds |
| `ENRICHMENT_ENDPOINT_MAX_RETRIES` | int | `5` | Maximum number of retries |
| `ENRICHMENT_ENDPOINT_INITIAL_DELAY` | float | `2.0` | Initial retry delay in seconds |
| `ENRICHMENT_ENDPOINT_BACKOFF_FACTOR` | float | `2.0` | Backoff factor for retries |
| `ENRICHMENT_ENDPOINT_EXTRA_PARAMS` | string | _(empty)_ | JSON-encoded extra parameters |
| `ENRICHMENT_ENDPOINT_MAX_TOKENS` | int | `4000` | Maximum token limit for the enrichment model |

## Search

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `DEFAULT_SEARCH_PROVIDER` | string | `sqlite` | Search backend: `sqlite` or `vectorchord` |

## Git

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `GIT_PROVIDER` | string | `dulwich` | Git provider identifier (used internally) |

## Periodic Sync

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `PERIODIC_SYNC_ENABLED` | bool | `true` | Enable periodic sync |
| `PERIODIC_SYNC_INTERVAL_SECONDS` | float | `1800` | Interval between periodic syncs in seconds |
| `PERIODIC_SYNC_RETRY_ATTEMPTS` | int | `3` | Number of retry attempts for failed syncs |

## Remote Server

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `REMOTE_SERVER_URL` | string | _(empty)_ | Remote Kodit server URL |
| `REMOTE_API_KEY` | string | _(empty)_ | API key for authentication |
| `REMOTE_TIMEOUT` | float | `30` | Request timeout in seconds |
| `REMOTE_MAX_RETRIES` | int | `3` | Maximum retry attempts |
| `REMOTE_VERIFY_SSL` | bool | `true` | Verify SSL certificates |

## Reporting

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `REPORTING_LOG_TIME_INTERVAL` | float | `5` | Progress log interval in seconds |

## LLM Cache

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `LITELLM_CACHE_ENABLED` | bool | `true` | Enable LLM response caching |

## Applying Configuration

There are two ways to apply configuration to Kodit:

1. A local `.env` file (e.g. `kodit serve --env-file .env`)
2. Environment variables (e.g. `DATA_DIR=/path/to/kodit/data kodit serve`)

How you specify environment variables is dependent on your deployment mechanism.

### Docker Compose

For example, in docker compose you can use the `environment` key:

```yaml
services:
  kodit:
    environment:
      - DATA_DIR=/path/to/kodit/data
```

### Kubernetes

For example, in Kubernetes you can use the `env` key:

```yaml
env:
  - name: DATA_DIR
    value: /path/to/kodit/data
```

## Example Configurations

### Enrichment Endpoints

Enrichment is typically the slowest part of the indexing process because it requires
calling a remote LLM provider. Ideally you want to maximise the number of parallel tasks
but all services have rate limits. Start low and increase over time.

See the [configuration reference](/kodit/reference/configuration/index.md) for
full details. The following is a selection of examples.

#### Helix.ml Enrichment Endpoint

Get your free API key from [Helix.ml](https://app.helix.ml/account).

```sh
ENRICHMENT_ENDPOINT_BASE_URL=https://app.helix.ml/v1
ENRICHMENT_ENDPOINT_MODEL=hosted_vllm/Qwen/Qwen3-8B
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=1
ENRICHMENT_ENDPOINT_TIMEOUT=300
ENRICHMENT_ENDPOINT_API_KEY=hl-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

#### Local Ollama Enrichment Endpoint

```sh
ENRICHMENT_ENDPOINT_BASE_URL=http://localhost:11434
ENRICHMENT_ENDPOINT_MODEL=ollama_chat/qwen3:1.7b
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=1
ENRICHMENT_ENDPOINT_EXTRA_PARAMS='{"think": false}'
ENRICHMENT_ENDPOINT_TIMEOUT=300
```

#### Azure OpenAI Enrichment Endpoint

```sh
ENRICHMENT_ENDPOINT_BASE_URL=https://winderai-openai-test.openai.azure.com/
ENRICHMENT_ENDPOINT_MODEL=azure/gpt-4.1-nano # Must be in the format "azure/azure_deployment_name"
ENRICHMENT_ENDPOINT_API_KEY=XXXX
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=5 # Azure defaults to 100K TPM
ENRICHMENT_ENDPOINT_EXTRA_PARAMS={"api_version": "2024-12-01-preview"}
```
