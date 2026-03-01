---
title: Configuration Reference
description: Kodit Configuration Reference
weight: 29
---

This document contains the complete configuration reference for Kodit. All configuration is done through environment variables.

## Environment Variables

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `HOST` | string | `0.0.0.0` | The server host to bind to. |
| `PORT` | int | `8080` | The server port to listen on. |
| `DATA_DIR` | string | `~/.kodit` | The data directory path. |
| `DB_URL` | string | `sqlite:///{data_dir}/kodit.db` | The database connection URL. |
| `LOG_LEVEL` | string | `INFO` | The log verbosity level. |
| `LOG_FORMAT` | string | `pretty` | The log output format (pretty or json). |
| `DISABLE_TELEMETRY` | bool | `false` | Telemetry collection. |
| `SKIP_PROVIDER_VALIDATION` | bool | `false` | **Deprecated.** No longer needed — enrichments are automatically disabled when no enrichment endpoint is configured. Will be removed in a future release. |
| `API_KEYS` | string | _(empty)_ | A comma-separated list of valid API keys. |
| `EMBEDDING_ENDPOINT_BASE_URL` | string | _(empty)_ | The base URL for the endpoint. |
| `EMBEDDING_ENDPOINT_MODEL` | string | _(empty)_ | The model identifier (e.g., openai/text-embedding-3-small). |
| `EMBEDDING_ENDPOINT_API_KEY` | string | _(empty)_ | The API key for authentication. |
| `EMBEDDING_ENDPOINT_NUM_PARALLEL_TASKS` | int | `1` | The number of parallel tasks. |
| `EMBEDDING_ENDPOINT_SOCKET_PATH` | string | _(empty)_ | The Unix socket path for local communication. |
| `EMBEDDING_ENDPOINT_TIMEOUT` | float | `60` | The request timeout in seconds. |
| `EMBEDDING_ENDPOINT_MAX_RETRIES` | int | `5` | The maximum number of retries. |
| `EMBEDDING_ENDPOINT_INITIAL_DELAY` | float | `2.0` | The initial retry delay in seconds. |
| `EMBEDDING_ENDPOINT_BACKOFF_FACTOR` | float | `2.0` | The retry backoff multiplier. |
| `EMBEDDING_ENDPOINT_EXTRA_PARAMS` | string | _(empty)_ | A JSON-encoded map of extra parameters. |
| `EMBEDDING_ENDPOINT_MAX_TOKENS` | int | `4000` | The maximum token limit. |
| `EMBEDDING_ENDPOINT_MAX_BATCH_CHARS` | int | `16000` | The maximum total characters per embedding batch. |
| `EMBEDDING_ENDPOINT_MAX_BATCH_SIZE` | int | `1` | The maximum number of requests per batch. |
| `ENRICHMENT_ENDPOINT_BASE_URL` | string | _(empty)_ | The base URL for the endpoint. |
| `ENRICHMENT_ENDPOINT_MODEL` | string | _(empty)_ | The model identifier (e.g., openai/text-embedding-3-small). |
| `ENRICHMENT_ENDPOINT_API_KEY` | string | _(empty)_ | The API key for authentication. |
| `ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS` | int | `1` | The number of parallel tasks. |
| `ENRICHMENT_ENDPOINT_SOCKET_PATH` | string | _(empty)_ | The Unix socket path for local communication. |
| `ENRICHMENT_ENDPOINT_TIMEOUT` | float | `60` | The request timeout in seconds. |
| `ENRICHMENT_ENDPOINT_MAX_RETRIES` | int | `5` | The maximum number of retries. |
| `ENRICHMENT_ENDPOINT_INITIAL_DELAY` | float | `2.0` | The initial retry delay in seconds. |
| `ENRICHMENT_ENDPOINT_BACKOFF_FACTOR` | float | `2.0` | The retry backoff multiplier. |
| `ENRICHMENT_ENDPOINT_EXTRA_PARAMS` | string | _(empty)_ | A JSON-encoded map of extra parameters. |
| `ENRICHMENT_ENDPOINT_MAX_TOKENS` | int | `4000` | The maximum token limit. |
| `ENRICHMENT_ENDPOINT_MAX_BATCH_CHARS` | int | `16000` | The maximum total characters per embedding batch. |
| `ENRICHMENT_ENDPOINT_MAX_BATCH_SIZE` | int | `1` | The maximum number of requests per batch. |
| `PERIODIC_SYNC_ENABLED` | bool | `true` | Whether periodic sync is enabled. |
| `PERIODIC_SYNC_INTERVAL_SECONDS` | float | `1800` | The sync interval in seconds. |
| `PERIODIC_SYNC_RETRY_ATTEMPTS` | int | `3` | The number of retry attempts. |
| `REMOTE_SERVER_URL` | string | _(empty)_ | The remote server URL. |
| `REMOTE_API_KEY` | string | _(empty)_ | The API key for authentication. |
| `REMOTE_TIMEOUT` | float | `30` | The request timeout in seconds. |
| `REMOTE_MAX_RETRIES` | int | `3` | The maximum retry attempts. |
| `REMOTE_VERIFY_SSL` | bool | `true` | SSL certificate verification. |
| `REPORTING_LOG_TIME_INTERVAL` | float | `5` | The logging interval in seconds. |
| `LITELLM_CACHE_ENABLED` | bool | `true` | Whether caching is enabled. |
| `WORKER_COUNT` | int | `1` | The number of background workers. |
| `SEARCH_LIMIT` | int | `10` | The default search result limit. |
| `HTTP_CACHE_DIR` | string | _(empty)_ | The directory for caching HTTP responses to disk. |
| `SIMPLE_CHUNKING_ENABLED` | bool | `false` | SimpleChunking enables fixed-size text chunking instead of AST-based snippet extraction. |
| `CHUNK_SIZE` | int | `1500` | The target size in characters for each text chunk. |
| `CHUNK_OVERLAP` | int | `200` | The number of overlapping characters between adjacent chunks. |
| `CHUNK_MIN_SIZE` | int | `50` | The minimum chunk size in characters; smaller chunks are dropped. |

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
ENRICHMENT_ENDPOINT_MODEL=Qwen/Qwen3-8B
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=1
ENRICHMENT_ENDPOINT_TIMEOUT=300
ENRICHMENT_ENDPOINT_API_KEY=hl-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

#### Local Ollama Enrichment Endpoint

```sh
ENRICHMENT_ENDPOINT_BASE_URL=http://localhost:11434
ENRICHMENT_ENDPOINT_MODEL=qwen3:1.7b
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=1
ENRICHMENT_ENDPOINT_EXTRA_PARAMS='{"think": false}'
ENRICHMENT_ENDPOINT_TIMEOUT=300
```
