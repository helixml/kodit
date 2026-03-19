# Design: Kodit-based RAG

## Architecture Overview

Helix knowledge indexing reconciler (`api/pkg/controller/knowledge/`) invokes a `rag.RAG` interface implementation chosen by `RAG_DEFAULT_PROVIDER`. This task adds a `rag_kodit.go` implementation and wires it into the server startup.

```
Reconciler.indexKnowledge()
  └─► [kodit provider] indexKoditFilestore()
        ├─ derive local path from filestore basePath + knowledge source path
        ├─ koditClient.Repositories.Add("file://{localPath}")  → repoID
        ├─ persist repoID to .kodit_meta.json in the filestore dir
        └─ return (kodit worker indexes asynchronously)

Reconciler.getRagClient() → KoditRAG

KoditRAG.Index(ctx, chunks...)  → no-op (kodit already indexed)
KoditRAG.Query(ctx, q)         → koditClient.Search.Query(ctx, q.Prompt,
                                      service.WithRepositories(repoID),
                                      service.WithLimit(n))
KoditRAG.Delete(ctx, req)      → koditClient.Repositories.Delete(repoID)
                                  + remove .kodit_meta.json
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
  1. Resolve repoID from `.kodit_meta.json` at the filestore path for the `DataEntityID`.
  2. Call `k.client.Search.Query(ctx, q.Prompt, service.WithRepositories(repoID), service.WithLimit(maxResults))`.
  3. Convert `service.SearchResult` → `*types.SessionRAGResult`.

- **Delete method**:
  1. Resolve repoID from metadata.
  2. Call `k.client.Repositories.Delete(ctx, repoID)`.
  3. Remove `.kodit_meta.json`.

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
5. Persist `{repo_id: N}` as `.kodit_meta.json` inside `localPath`.
6. Wait for the kodit worker to finish indexing (`r.koditClient.WorkerIdle()` poll or event).
7. Mark knowledge as ready.

### 5. Metadata storage

Store the mapping as a JSON sidecar file **inside the filestore directory** being indexed:

```
{localPath}/.kodit_meta.json
{ "repo_id": 42 }
```

Pros: co-located with the files, survives helix restarts, survives DB resets.
Cons: lives inside the user's filestore dir — not a problem since `.kodit_*` prefix is reserved.

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
| Metadata location | `.kodit_meta.json` inside filestore dir | Close to data, survives restarts |
| Index method | no-op | Kodit handles chunking/embedding internally |
| Indexing trigger | intercept before `getIndexingData` | Avoids redundant extraction for filestore sources |
| Non-filestore sources | return error | Kodit file:// only works for local paths; web sources not supported |
