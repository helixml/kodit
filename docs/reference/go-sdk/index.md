---
title: Go SDK Reference
description: Kodit Go Library and HTTP Client API Reference
weight: 31
---

Kodit provides two Go packages for programmatic access:

1. **`github.com/helixml/kodit`** — The core library for embedding Kodit directly into your Go application
2. **`github.com/helixml/kodit/clients/go`** — A generated HTTP client for calling a remote Kodit server

## Go Library

### Installation

```bash
go get github.com/helixml/kodit
```

### Creating a Client

The `kodit.Client` is the main entry point. Create one with `kodit.New()` and configure it with functional options.

#### SQLite (local, single-process)

```go
import "github.com/helixml/kodit"

client, err := kodit.New(
    kodit.WithSQLite("/path/to/data.db"),
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

#### PostgreSQL with VectorChord (production, multi-process)

```go
client, err := kodit.New(
    kodit.WithPostgresVectorchord("postgres://user:pass@localhost:5432/kodit"),
    kodit.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
)
```

### Configuration Options

| Option | Description |
|--------|-------------|
| `WithSQLite(path)` | Use SQLite as the database. BM25 uses FTS5, vector search uses the configured embedding provider. |
| `WithPostgresVectorchord(dsn)` | Use PostgreSQL with the VectorChord extension for BM25 and vector search. |
| `WithOpenAI(apiKey)` | Set OpenAI as the AI provider for text generation and embeddings. |
| `WithOpenAIConfig(cfg)` | Set OpenAI with custom configuration (base URL, model overrides). |
| `WithAnthropic(apiKey)` | Set Anthropic Claude as the text generation provider. Requires a separate embedding provider. |
| `WithAnthropicConfig(cfg)` | Set Anthropic Claude with custom configuration. |
| `WithTextProvider(p)` | Set a custom text generation provider. |
| `WithEmbeddingProvider(p)` | Set a custom embedding provider. |
| `WithDataDir(dir)` | Set the data directory for cloned repositories and storage. Defaults to `~/.kodit`. |
| `WithCloneDir(dir)` | Set the directory where repositories are cloned. Defaults to `{dataDir}/repos`. |
| `WithModelDir(dir)` | Set the directory for built-in model files. Defaults to `{dataDir}/models`. |
| `WithLogger(l)` | Set a custom `*slog.Logger`. |
| `WithAPIKeys(keys...)` | Set API keys for HTTP API authentication. |
| `WithWorkerCount(n)` | Set the number of background worker goroutines. Defaults to 1. |
| `WithWorkerPollPeriod(d)` | Set how often the background worker checks for new tasks. Defaults to 1s. |
| `WithPeriodicSyncConfig(cfg)` | Configure periodic repository sync intervals. |

### Services

The `Client` exposes services as public fields:

| Field | Type | Description |
|-------|------|-------------|
| `Repositories` | `*service.Repository` | Add, delete, sync, and query repositories |
| `Commits` | `*service.Commit` | Query indexed commits |
| `Tags` | `*service.Tag` | Query repository tags |
| `Files` | `*service.File` | Query indexed files |
| `Enrichments` | `*service.Enrichment` | Query enrichments (snippets, architecture docs, etc.) |
| `Tasks` | `*service.Queue` | Inspect and manage the background task queue |
| `Tracking` | `*service.Tracking` | Monitor indexing progress |
| `Search` | `*service.Search` | Hybrid code search (BM25 + vector) |

### Usage Examples

#### Adding and Indexing a Repository

```go
import "github.com/helixml/kodit/application/service"

source, created, err := client.Repositories.Add(ctx, &service.RepositoryAddParams{
    URL:    "https://github.com/kubernetes/kubernetes",
    Branch: "master",
})
if err != nil {
    log.Fatal(err)
}

// Indexing runs automatically in the background worker.
// Wait for the worker to finish:
for !client.WorkerIdle() {
    time.Sleep(time.Second)
}
```

#### Searching Code

```go
results, err := client.Search.Query(ctx, "create a deployment",
    service.WithSemanticWeight(0.7),
    service.WithLimit(10),
    service.WithLanguages("go"),
)
if err != nil {
    log.Fatal(err)
}

for _, enrichment := range results.Enrichments() {
    fmt.Printf("ID: %d, Language: %s\n", enrichment.ID(), enrichment.Language())
    fmt.Println(enrichment.Content())
}
```

##### Search Options

| Option | Description |
|--------|-------------|
| `WithSemanticWeight(w)` | Weight for semantic (vector) search, 0.0 to 1.0 |
| `WithLimit(n)` | Maximum number of results |
| `WithOffset(n)` | Offset for pagination |
| `WithLanguages(langs...)` | Filter by programming languages |
| `WithRepositories(ids...)` | Filter by repository IDs |
| `WithEnrichmentTypes(types...)` | Filter by enrichment types |
| `WithMinScore(score)` | Minimum score threshold |
| `WithSnippets(bool)` | Include code snippets |
| `WithDocuments(bool)` | Include enrichment documents |

#### Querying Commits and Files

```go
import "github.com/helixml/kodit/domain/repository"

// Find all commits for a repository
commits, err := client.Commits.Find(ctx, repository.WithRepoID(repoID))

// Find files
files, err := client.Files.Find(ctx, repository.WithRepoID(repoID))
```

### Error Handling

The `kodit` package exports sentinel errors for common failure modes:

| Error | Description |
|-------|-------------|
| `ErrNoDatabase` | No database was configured |
| `ErrNoProvider` | No AI provider was configured |
| `ErrProviderNotCapable` | The provider lacks a required capability |
| `ErrNotFound` | A requested resource was not found |
| `ErrValidation` | A validation error occurred |
| `ErrConflict` | A conflict with existing data |
| `ErrEmptySource` | A source with no content to process |
| `ErrClientClosed` | The client has been closed |

Use `errors.Is` to check:

```go
_, err := client.Search.Query(ctx, "test")
if errors.Is(err, kodit.ErrClientClosed) {
    // handle closed client
}
```

### Lifecycle

- **`Close()`** — Stops the background worker and periodic sync, closes the embedding provider and database. Always defer this after creating a client.
- **`WorkerIdle()`** — Reports whether the background worker has no in-flight tasks. Useful for waiting until indexing completes.

---

## Go HTTP Client

The `clients/go` package provides a generated HTTP client for calling a remote Kodit server's REST API.

### Installation

```bash
go get github.com/helixml/kodit/clients/go
```

### Creating an HTTP Client

```go
import koditclient "github.com/helixml/kodit/clients/go"

client, err := koditclient.NewClient("https://kodit.example.com")
if err != nil {
    log.Fatal(err)
}
```

You can pass a custom `http.Client` for authentication or transport configuration:

```go
httpClient := &http.Client{
    Timeout: 30 * time.Second,
}
client, err := koditclient.NewClient(
    "https://kodit.example.com",
    koditclient.WithHTTPClient(httpClient),
)
```

### Usage

The client exposes methods matching the [REST API](../api/):

```go
// List repositories
resp, err := client.GetApiV1Repositories(ctx)

// Add a repository
resp, err := client.PostApiV1Repositories(ctx, koditclient.PostApiV1RepositoriesJSONRequestBody{
    Url: "https://github.com/example/repo",
})

// Search
resp, err := client.PostApiV1SearchMulti(ctx, koditclient.PostApiV1SearchMultiJSONRequestBody{
    TextQuery: "create a deployment",
    TopK:      10,
})
```

### Notes

- Types are auto-generated from the Kodit OpenAPI specification using [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen).
- The generated client provides type-safe request and response structs for all API endpoints.
- See the [Server API Reference](../api/) for the full list of available endpoints.
