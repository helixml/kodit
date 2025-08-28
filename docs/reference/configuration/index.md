---
title: Configuration
description: How to configure Kodit.
weight: 10
---

Configuration of Kodit is performed by setting environmental variables or adding
variables to a .env file.

{{< warn >}}
Note that updating a setting does not automatically update the data that uses that
setting. For example, if you change a provider, you will need to delete and
recreate all indexes.
{{< /warn >}}

## Configuring Indexing

### Default Indexing Provider

By default, Kodit will use small local models for semantic search and enrichment. If you
are using Kodit in a professional capacity, it is likely that the local model latency is
too high to provide a good developer experience.

Instead, you should use an external provider. Under the hood, Kodit uses
[LiteLLM](https://docs.litellm.ai/) so that you can connect to 100+ providers.

### Configuring the Provider

The following configuration settings are available for you to configure the provider
used during indexing:

```bash
EMBEDDING_ENDPOINT_BASE_URL
EMBEDDING_ENDPOINT_MODEL # Model to use for the endpoint in litellm format (e.g. 'openai/text-embedding-3-small') 
EMBEDDING_ENDPOINT_API_KEY
EMBEDDING_ENDPOINT_NUM_PARALLEL_TASKS
EMBEDDING_ENDPOINT_SOCKET_PATH # Unix socket path for local communication (e.g., /tmp/openai.sock)
EMBEDDING_ENDPOINT_TIMEOUT # Request timeout in seconds (default: 30.0)
EMBEDDING_ENDPOINT_EXTRA_PARAMS # Extra provider-specific non-secret parameters for LiteLLM

ENRICHMENT_ENDPOINT_BASE_URL
ENRICHMENT_ENDPOINT_MODEL # Model to use for the endpoint in litellm format (e.g. 'openai/text-embedding-3-small') 
ENRICHMENT_ENDPOINT_API_KEY
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS
ENRICHMENT_ENDPOINT_SOCKET_PATH # Unix socket path for local communication (e.g., /tmp/openai.sock)
ENRICHMENT_ENDPOINT_TIMEOUT # Request timeout in seconds (default: 30.0)
ENRICHMENT_ENDPOINT_EXTRA_PARAMS # Extra provider-specific non-secret parameters for LiteLLM
```

### OpenAI Provider Example

Here is an example that works with OpenAI as the provider:

```bash
EMBEDDING_ENDPOINT_API_KEY=sk-proj-xxxx
EMBEDDING_ENDPOINT_MODEL=openai/text-embedding-3-small
ENRICHMENT_ENDPOINT_API_KEY=sk-proj-xxxx
ENRICHMENT_ENDPOINT_MODEL=openai/gpt-5-nano
```

### Ollama Provider Example

Here is an example that works with a local [Ollama](https://ollama.com/) setup:

```bash
EMBEDDING_ENDPOINT_BASE_URL=http://localhost:11434
EMBEDDING_ENDPOINT_MODEL=ollama/mxbai-embed-large
EMBEDDING_ENDPOINT_NUM_PARALLEL_TASKS=1
ENRICHMENT_ENDPOINT_BASE_URL=http://localhost:11434
ENRICHMENT_ENDPOINT_MODEL=ollama/qwen3:1.7b
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=1 # This is important as the local API can't accept concurrent requests
```

### Azure OpenAI Provider Example

Prerequisites:

- Azure OpenAI enabled
- Chat and embedding model deployments deployed

Here is an example that works with Azure AI Foundry | Azure OpenAI:

```bash
EMBEDDING_ENDPOINT_BASE_URL=https://winderai-openai-test.openai.azure.com/ # Matches base url listed in example
EMBEDDING_ENDPOINT_MODEL=azure/text-embedding-3-small # Must be in the format "azure/azure_deployment_name"
EMBEDDING_ENDPOINT_API_KEY=the_api_key_listed_in_azure
EMBEDDING_ENDPOINT_NUM_PARALLEL_TASKS=5 # Azure defaults to 150K TPM, 900 RPM, tune according to load
EMBEDDING_ENDPOINT_EXTRA_PARAMS={"api_version": "2024-12-01-preview"} # Matches API version listed in example

ENRICHMENT_ENDPOINT_BASE_URL=https://winderai-openai-test.openai.azure.com/  # Matches base url listed in example
ENRICHMENT_ENDPOINT_MODEL=azure/gpt-4.1-nano # Must be in the format "azure/azure_deployment_name"
ENRICHMENT_ENDPOINT_API_KEY=the_api_key_listed_in_azure
ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS=3 # Azure defaults to 100K TPM, 100 RPM, tune according to load
ENRICHMENT_ENDPOINT_EXTRA_PARAMS={"api_version": "2024-12-01-preview"} # Matches API version listed in example
```

## Configuring the Database

Out of the box Kodit uses a local sqlite file to make it easier for users to get
started. But for production use, it's likely you will want to use a database that has
dedicated semantic and keyword search capabilities for reduced latency.

### VectorChord Database

[VectorChord](https://github.com/tensorchord/VectorChord) is an optimized PostgreSQL
extension that provides both vector and BM25 search. (See [Search](#search))

Start a container with:

```sh
docker run \
  --name kodit-vectorchord \
  -e POSTGRES_DB=kodit \
  -e POSTGRES_PASSWORD=mysecretpassword \
  -p 5432:5432 \
  -d tensorchord/vchord-suite:pg17-20250601
```

{{< warn >}}
Kodit assumes the database exists. In the above example I'm abusing the POSTGRES_DB
environmental variable from the [Postgres Docker
container](https://hub.docker.com/_/postgres/) to create the database for me. In
production setups, please create a database yourself.
{{< /warn >}}

Then update your `.env` file to include:

```env
DB_URL=postgresql+asyncpg://postgres:mysecretpassword@localhost:5432/kodit
```

## Configuring Search

### Default Search Provider

By default, Kodit will use built-in implementations of BM25 and similarity search to
improve the out of the box experience. If you are using Kodit in a professional
capacity, it is likely that the search latency is too high to provide a good developer
experience.

Instead, you should use the features included in your database. The settings provided
here will cause all search functionality to use this database by default. You can
override the database used for each search type if you wish. (Coming soon!)

#### VectorChord Search

Configure Kodit to use a [VectorChord database](#vectorchord-database).

Then update your `.env` file to include:

```env
DB_URL=postgresql+asyncpg://postgres:mysecretpassword@localhost:5432/kodit
DEFAULT_SEARCH_PROVIDER=vectorchord
```

## Configuring Sync

### Periodic Sync Configuration

Kodit can automatically sync all indexed codebases at regular intervals to keep them up-to-date with the latest changes. This is especially useful for server deployments where multiple users are working with the same codebases.

```bash
# Enable/disable periodic sync (default: true)
SYNC_PERIODIC_ENABLED=true

# Sync interval in seconds (default: 1800 = 30 minutes)
SYNC_PERIODIC_INTERVAL_SECONDS=1800

# Number of retry attempts for failed syncs (default: 3)
SYNC_PERIODIC_RETRY_ATTEMPTS=3
```

The sync scheduler will:

- Run automatically in the background when the server starts
- Sync all existing indexes at the configured interval
- Handle failures gracefully with retry logic
- Log detailed progress and results
- Shut down cleanly when the server stops

{{< info >}}
**Note**: The sync scheduler only syncs existing indexes. It does not create new indexes for repositories that haven't been indexed yet.
{{< /info >}}

## Configuring Enrichment

### Default Enrichment Provider

The default enrichment provider is the same as [the default indexing provider](#default-indexing-provider).
