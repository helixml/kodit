# Design: Kodit-based RAG

## Architecture Overview

Helix knowledge indexing reconciler (`api/pkg/controller/knowledge/`) invokes a `rag.RAG` interface implementation chosen by `RAG_DEFAULT_PROVIDER`. This task adds a `rag_kodit.go` implementation and wires it into the server startup.

```
Reconciler.indexKnowledge()
  └─► [kodit provider] indexKoditFilestore()
        ├─ derive local path from filestore basePath + knowledge source path
        ├─ koditClient.Repositories.Add("file://{localPath}")  → repoID
        ├─ store repoID in knowledge.KoditRepoID (DB update)
        └─ return (kodit worker indexes asynchronously)

Reconciler.getRagClient() → KoditRAG

KoditRAG.Index(ctx, chunks...)  → no-op (kodit already indexed)
KoditRAG.Query(ctx, q)         → koditClient.Search.Query(ctx, q.Prompt,
                                      service.WithRepositories(repoID),
                                      service.WithLimit(n))
KoditRAG.Delete(ctx, req)      → koditClient.Repositories.Delete(repoID)
```

## Key Components

### 1. Update kodit dependency

In `go.mod`, replace the published `v1.2.0` with the specific commit that adds `file://` support:

```
go get github.com/helixml/kodit@417f16b7dfce928b0e9d1a888454cfc6cbe98892
go mod tidy
```

### 2. Config: add `kodit` RAG provider

`api/pkg/config/config.go` already defines:

```go
type RAGProvider string
const (
    RAGProviderTypesense  RAGProvider = "typesense"
    RAGProviderLlamaindex RAGProvider = "llamaindex"
    RAGProviderHaystack   RAGProvider = "haystack"
)
```

Add:
```go
RAGProviderKodit RAGProvider = "kodit"
```

No new env var is needed — the existing `RAG_DEFAULT_PROVIDER` covers this.

### 3. KoditRAG struct (`api/pkg/rag/rag_kodit.go`)

```go
type KoditRAG struct {
    client      *kodit.Client
    filestoreBasePath string  // config.FileStore.LocalFSPath
}
```

- **Conversion (Index method)**: The `Index` method is called with pre-chunked data from the helix indexing pipeline. For the kodit provider, helix should call `indexKoditFilestore` **before** extraction/chunking (see §4). The `Index` method itself is a **no-op** — kodit has already done the work.

- **Index method** (no-op):
  ```go
  func (k *KoditRAG) Index(ctx context.Context, req ...*types.SessionRAGIndexChunk) error {
      return nil
  }
  ```

- **Query method**:
  1. Resolve repoID from `knowledge.KoditRepoID` (looked up via `store.GetKnowledge` by `DataEntityID`).
  2. Call `k.client.Search.Query(ctx, q.Prompt, service.WithRepositories(repoID), service.WithLimit(maxResults))`.
  3. Convert `service.SearchResult` → `*types.SessionRAGResult`.

- **Delete method**:
  1. Resolve repoID via store lookup.
  2. Call `k.client.Repositories.Delete(ctx, repoID)`.

### 4. Indexer integration (`api/pkg/controller/knowledge/knowledge_indexer.go`)

Add a special case in `indexKnowledge` before the generic `getIndexingData` call:

```go
if r.config.RAG.DefaultRagProvider == config.RAGProviderKodit {
    return r.indexKoditFilestore(ctx, k, version)
}
```

`indexKoditFilestore`:
1. Require `k.Source.Filestore != nil` — return error otherwise.
2. Build local path: `filepath.Join(r.config.FileStore.LocalFSPath, k.Source.Filestore.Path)`.
3. Wait for the directory to exist (retry with timeout, reuse existing `ErrNoFilesFound` pattern).
4. Call `r.koditClient.Repositories.Add(ctx, &service.RepositoryAddParams{URL: "file://" + localPath})`.
5. Store the returned `repoID` in `k.KoditRepoID` and call `r.store.UpdateKnowledge(ctx, k)`.
6. Wait for the kodit worker to finish indexing (`r.koditClient.WorkerIdle()` poll or event).
7. Mark knowledge as ready.

### 5. Metadata storage

Add a `KoditRepoID` column to the `Knowledge` DB table:

```go
// In types.Knowledge struct:
KoditRepoID int64 `json:"kodit_repo_id,omitempty" gorm:"default:0"`
```

GORM AutoMigrate will add the column on next startup (per project conventions). The value is written once in `indexKoditFilestore` via `store.UpdateKnowledge` and read by `KoditRAG.Query` and `KoditRAG.Delete` via a store lookup on `DataEntityID`.

`RAGSettings` already has a nested `Typesense` struct for Typesense-specific connection settings. The kodit repo ID is runtime state (not a user-configurable setting), so it belongs on `Knowledge` directly rather than inside `RAGSettings`.

### 6. Reconciler construction

In `New()` (or its equivalent in the server startup), instantiate `KoditRAG` when `config.RAG.DefaultRagProvider == RAGProviderKodit` and the kodit client is available.

The `Reconciler` struct already holds `filestore filestore.FileStore`. For the local-path derivation we need the raw `basePath`. Two options:
- Pass `config.FileStore.LocalFSPath` directly to `KoditRAG`.
- Add `LocalPath() string` to the `FileStore` interface (invasive — prefer option 1).

## Patterns Found in Codebase

- Helix uses `envconfig` with a flat `ServerConfig` struct — no prefix needed on env vars.
- `RAGProvider` is a typed string constant — follow existing pattern exactly.
- `Reconciler` receives a `rag.RAG` interface at construction; custom per-knowledge RAG clients are created via `newRagClient func(settings *types.RAGSettings) rag.RAG`.
- All four existing RAG implementations live in `api/pkg/rag/` and are standalone structs that embed nothing — keep the same pattern for `rag_kodit.go`.
- Kodit `Client.Repositories.Add()` is idempotent: if the URL already exists it returns the existing source — safe to call on re-index.
- The knowledge indexer retries on `ErrNoFilesFound` for up to 10 minutes — the same retry is appropriate for kodit when files haven't appeared yet.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Where to put kodit RAG impl | `api/pkg/rag/rag_kodit.go` | Consistent with all other backends |
| Metadata location | `Knowledge.KoditRepoID` DB column | DB is the source of truth; avoids file I/O and sidecar management |
| Index method | no-op | Kodit handles chunking/embedding internally |
| Indexing trigger | intercept before `getIndexingData` | Avoids redundant extraction for filestore sources |
| Non-filestore sources | return error | Kodit file:// only works for local paths; web sources not supported |
